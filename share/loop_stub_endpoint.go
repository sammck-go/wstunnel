package chshare

import (
	"context"
	"fmt"
	"github.com/prep/socketpair"
)

// LoopStubEndpoint implements a local Loop stub
type LoopStubEndpoint struct {
	// Implements LocalStubChannelEndpoint
	BasicEndpoint
	loopServer *LoopServer
	listening  bool
	// callerConns contains a queue of Caller ChannelCons that are
	// waiting to be accepted with an Accept call
	callerConns chan ChannelConn
}

// NewLoopStubEndpoint creates a new LoopStubEndpoint
func NewLoopStubEndpoint(
	logger Logger,
	ced *ChannelEndpointDescriptor,
	loopServer *LoopServer,
) (*LoopStubEndpoint, error) {
	ep := &LoopStubEndpoint{
		BasicEndpoint: BasicEndpoint{
			ced: ced,
		},
		loopServer:  loopServer,
		callerConns: make(chan ChannelConn, 5), // Allow a backlog of 5 connect requests before Accept()
	}
	ep.InitBasicEndpoint(logger, ep, "LoopStubEndpoint: %s", ced)
	return ep, nil
}

// GetLoopPath returns the loop pathname associated with this LoopStubEndpoint
func (ep *LoopStubEndpoint) GetLoopPath() string {
	return ep.ced.Path
}

// HandleOnceShutdown will be called exactly once, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
func (ep *LoopStubEndpoint) HandleOnceShutdown(completionErr error) error {
	ep.Lock.Lock()
	if ep.listening {
		ep.loopServer.UnregisterAcceptor(ep.GetLoopPath(), ep)
		ep.listening = false
	}
	ep.Lock.Unlock()

	for dc := range ep.callerConns {
		if dc != nil {
			dc.Close()
		}
	}

	close(ep.callerConns)

	return completionErr
}

// StartListening begins responding to Caller network clients in anticipation of Accept() calls. It
// is implicitly called by the first call to Accept() if not already called. It is only necessary to call
// this method if you need to begin accepting Callers before you make the first Accept call. Part of
// AcceptorChannelEndpoint interface.
func (ep *LoopStubEndpoint) StartListening() error {
	ep.Lock.Lock()
	defer ep.Lock.Unlock()
	if !ep.listening {
		if ep.IsStartedShutdown() {
			return fmt.Errorf("%s: endpoint is closed", ep.Logger.Prefix())
		}
		err := ep.loopServer.RegisterAcceptor(ep.GetLoopPath(), ep)
		if err != nil {
			return fmt.Errorf("%s: StartListening failed: %s", ep.Logger.Prefix(), err)
		}
		ep.listening = true
	}
	return nil
}

// Accept listens for and accepts a single connection from a Caller network client as specified in the
// endpoint configuration. This call does not return until a new connection is available or a
// error occurs. There is no way to cancel an Accept() request other than closing the endpoint. Part of
// the AcceptorChannelEndpoint interface.
func (ep *LoopStubEndpoint) Accept(ctx context.Context) (ChannelConn, error) {
	dialConn, ok := <-ep.callerConns
	if !ok {
		return nil, fmt.Errorf("%s: endpoint is closed", ep.Logger.Prefix())
	}
	ep.AddShutdownChild(dialConn)
	return dialConn, nil
}

// AcceptAndServe listens for and accepts a single connection from a Caller network client as specified in the
// endpoint configuration, then services the connection using an already established
// calledServiceConn as the proxied Called Service's end of the session. This call does not return until
// the bridged session completes or an error occurs. There is no way to cancel the Accept() portion
// of the request other than closing the endpoint through other means. After the connection has been
// accepted, the context may be used to cancel servicing of the active session.
// Ownership of calledServiceConn is transferred to this function, and it will be closed before this function returns.
// This API may be more efficient than separately using Accept() and then bridging between the two
// ChannelConns with BasicBridgeChannels. In particular, "loop" endpoints can avoid creation
// of a socketpair and an extra bridging goroutine, by directly coupling the acceptor ChannelConn
// to the dialer ChannelConn.
// The return value is a tuple consisting of:
//        Number of bytes sent from the accepted callerConn to calledServiceConn
//        Number of bytes sent from calledServiceConn to the accelpted callerConn
//        An error, if one occured during accept or copy in either direction
func (ep *LoopStubEndpoint) AcceptAndServe(ctx context.Context, calledServiceConn ChannelConn) (int64, int64, error) {
	callerConn, err := ep.Accept(ctx)
	if err != nil {
		calledServiceConn.Close()
		return 0, 0, err
	}
	return BasicBridgeChannels(ctx, ep.Logger, callerConn, calledServiceConn)
}

// EnqueueCallerConn provides a ChannelConn to be returned by a future or pending Accept call
func (ep *LoopStubEndpoint) EnqueueCallerConn(dialConn ChannelConn) error {
	ep.Lock.Lock()
	defer ep.Lock.Unlock()
	if !ep.listening {
		return fmt.Errorf("%s: No listener on loop path", ep.Logger.Prefix())
	}
	select {
	case ep.callerConns <- dialConn:
		return nil
	default:
		return fmt.Errorf("%s: Listener accept backlog full", ep.Logger.Prefix())
	}
}

// HandleDial implements the bulk of Dial as required by the loopback skeleton endpoint
// It is more efficient to use HandleDialAndServe
func (ep *LoopStubEndpoint) HandleDial(ctx context.Context, extraData []byte) (ChannelConn, error) {
	// Create a socket pair so that the guy who calls Accept() has something to talk to and
	// we have something to return to the caller of Dial(). This results in one hop through a socket
	// but it preserves our abstraction that requires endpoints to create their ChannelConn
	// first, then we wire them together with a pipe task. This hop can be avoided if caller
	// uses HandleDialAndServe
	callerNetConn, calledServiceNetConn, err := socketpair.New("unix")
	if err != nil {
		return nil, fmt.Errorf("%s: Unable to create socketpair: %s", ep.Logger.Prefix(), err)
	}

	// Now we can create a ChannelCon for each end of the connection
	callerConn, err := NewSocketConn(ep.Logger, callerNetConn)
	if err != nil {
		callerNetConn.Close()
		calledServiceNetConn.Close()
		return nil, fmt.Errorf("%s: Unable to wrap net.Conn with SocketConn: %s", ep.Logger.Prefix(), err)
	}
	calledServiceConn, err := NewSocketConn(ep.Logger, calledServiceNetConn)
	if err != nil {
		callerConn.Close()
		calledServiceNetConn.Close()
		return nil, fmt.Errorf("%s: Unable to wrap net.Conn with SocketConn: %s", ep.Logger.Prefix(), err)
	}

	err = ep.EnqueueCallerConn(calledServiceConn)
	if err != nil {
		callerConn.Close()
		calledServiceConn.Close()
		return nil, fmt.Errorf("%s: EnqueueCallerConn failed: %s", ep.Logger.Prefix(), err)
	}

	return callerConn, nil
}

// HandleDialAndServe initiates a new connection to a Called Service as specified in the
// endpoint configuration, then services the connection using an already established
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
func (ep *LoopStubEndpoint) HandleDialAndServe(
	ctx context.Context,
	callerConn ChannelConn,
	extraData []byte,
) (int64, int64, error) {
	err := ep.EnqueueCallerConn(callerConn)
	if err != nil {
		callerConn.Close()
		return 0, 0, fmt.Errorf("%s: EnqueueCallerConn failed: %s", ep.Logger.Prefix(), err)
	}
	// There is no need to run a bridge because that will be done by whoever called Accept(). However,
	// to fulfill our contract we should wait for callerConn to close
	err = callerConn.WaitForClose()
	return callerConn.GetNumBytesRead(), callerConn.GetNumBytesWritten(), err
}
