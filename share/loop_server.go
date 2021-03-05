package chshare

import (
	"context"
	"fmt"
	"sync"
)

// Implementation of "loop" endpoint protocol

// Each "name" in the loopserver's namespace is associated with a LoopStubEndpoint
// that is waiting on a loop pathname to accept connections from a reote Caller
type loopEntry struct {
	name     string
	acceptor *LoopStubEndpoint
}

// LoopServer maintains a namespace of loop pathnames with waiting LoopStubEndpoint's.
type LoopServer struct {
	Logger
	lock    sync.Mutex
	entries map[string]*loopEntry
}

// NewLoopServer creates a new LoopServer
func NewLoopServer(logger Logger) (*LoopServer, error) {
	s := &LoopServer{
		Logger:  logger.Fork("LoopServer"),
		entries: make(map[string]*loopEntry),
	}
	return s, nil
}

func (s *LoopServer) String() string {
	return s.Logger.Prefix()
}

// GetEntry gets the loopEntry associated with a loop pathname. Returns
// nil if the entry does not exist
func (s *LoopServer) getEntry(name string) *loopEntry {
	s.lock.Lock()
	defer s.lock.Unlock()
	entry, _ := s.entries[name]
	return entry
}

// GetAcceptor gets the LoopStubEndpoint associated with a loop pathname. Returns
// nil if the entry does not exist
func (s *LoopServer) GetAcceptor(name string) *LoopStubEndpoint {
	var acceptor *LoopStubEndpoint
	entry := s.getEntry(name)
	if entry != nil {
		acceptor = entry.acceptor
	}
	return acceptor
}

// RegisterAcceptor registers a LoopStubEndpoint as the acceptor for a given loop pathname.
// Only one acceptor can be registered at a given time with a given name
func (s *LoopServer) RegisterAcceptor(name string, acceptor *LoopStubEndpoint) error {
	s.lock.Lock()
	defer s.lock.Unlock()
	entry, _ := s.entries[name]
	if entry != nil {
		return fmt.Errorf("%s: Loopback acceptor already registered for name: %s", s.Logger.Prefix(), name)
	}
	entry = &loopEntry{name: name, acceptor: acceptor}
	s.entries[name] = entry
	return nil
}

// UnregisterAcceptor unregisters a LoopStubEndpoint as the acceptor for a given loop pathname.
// Has no effect if the endpoint is not the current acceptor for the pathname. Returns true
// iff a removal occurred. Does *not* close the acceptor.
func (s *LoopServer) UnregisterAcceptor(name string, acceptor *LoopStubEndpoint) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	entry, _ := s.entries[name]
	remove := entry != nil && acceptor == entry.acceptor
	if remove {
		delete(s.entries, name)
	}
	return remove
}

// Dial initiates a new connection to a Called Service registered at a loop pathname
func (s *LoopServer) Dial(ctx context.Context, name string, extraData []byte) (ChannelConn, error) {
	acceptor := s.GetAcceptor(name)
	if acceptor == nil {
		return nil, fmt.Errorf("%s: Nothing listening on loopback name: %s", s.Logger.Prefix(), name)
	}
	return acceptor.HandleDial(ctx, extraData)
}

// DialAndServe initiates a new connection to a Called Service registered at a loop
// pathname, then services the connection using an already established
// callerConn as the proxied Caller's end of the session. This call does not return until
// the bridged session completes or an error occurs. The context may be used to cancel
// connection or servicing of the active session.
// Ownership of callerConn is transferred to this function, and it will be closed before
// this function returns, regardless of whether an error occurs.
// This API may be more efficient than separately using Dial() and then bridging between the two
// ChannelConns with BasicBridgeChannels. In particular, "loop" endpoints can avoid creation
// of a socketpair and an extra bridging goroutine, by directly coupling the acceptor ChannelConn
// to the dialer ChannelConn.
// The return value is a tuple consisting of:
//        Number of bytes sent from callerConn to the dialed calledServiceConn
//        Number of bytes sent from the dialed calledServiceConn callerConn
//        An error, if one occured during dial or copy in either direction
func (s *LoopServer) DialAndServe(
	ctx context.Context,
	name string,
	callerConn ChannelConn,
	extraData []byte,
) (int64, int64, error) {
	acceptor := s.GetAcceptor(name)
	if acceptor == nil {
		return 0, 0, fmt.Errorf("%s: Nothing listening on loopback name: %s", s.Logger.Prefix(), name)
	}
	return acceptor.HandleDialAndServe(ctx, callerConn, extraData)
}

// EnqueueCallerConn adds an existing ChannelConn to be used as a result from a pending or
// future Accept() request on a given loop name. Does not block; If the pending connect
// queue is full, an error will be returned.
func (s *LoopServer) EnqueueCallerConn(name string, dialConn ChannelConn) error {
	acceptor := s.GetAcceptor(name)
	if acceptor == nil {
		return fmt.Errorf("%s: Nothing listening on loopback name: %s", s.Logger.Prefix(), name)
	}
	return acceptor.EnqueueCallerConn(dialConn)
}
