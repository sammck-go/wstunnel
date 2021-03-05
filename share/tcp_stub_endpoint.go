package chshare

import (
	"context"
	"fmt"
	"net"
)

// TCPStubEndpoint implements a local TCP stub
type TCPStubEndpoint struct {
	// Implements LocalStubChannelEndpoint
	BasicEndpoint
	listenErr error
	listener  net.Listener
}

// NewTCPStubEndpoint creates a new TCPStubEndpoint
func NewTCPStubEndpoint(logger Logger, ced *ChannelEndpointDescriptor) (*TCPStubEndpoint, error) {
	ep := &TCPStubEndpoint{
		BasicEndpoint: BasicEndpoint{
			ced: ced,
		},
	}
	ep.InitBasicEndpoint(logger, ep, "TCPStubEndpoint: %s", ced)
	return ep, nil
}

// HandleOnceShutdown will be called exactly once, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
func (ep *TCPStubEndpoint) HandleOnceShutdown(completionErr error) error {
	var listener net.Listener
	ep.Lock.Lock()
	listener = ep.listener
	ep.listener = nil
	ep.Lock.Unlock()

	var err error
	if listener != nil {
		err = listener.Close()
	}

	if completionErr == nil {
		completionErr = err
	}
	return completionErr
}

func (ep *TCPStubEndpoint) getListener() (net.Listener, error) {
	var listener net.Listener
	var err error

	ep.Lock.Lock()
	{
		if ep.IsStartedShutdown() {
			err = fmt.Errorf("%s: Endpoint is closed", ep.Logger.Prefix())
		} else if ep.listener == nil && ep.listenErr == nil {
			// TODO: support IPV6
			listener, err = net.Listen("tcp4", ep.ced.Path)
			if err != nil {
				err = fmt.Errorf("%s: TCP listen failed for path '%s': %s", ep.Logger.Prefix(), ep.ced.Path, err)
			} else {
				ep.listener = listener
			}
			ep.listenErr = err
		} else {
			listener = ep.listener
			err = ep.listenErr
		}
	}
	ep.Lock.Unlock()

	return listener, err
}

// StartListening begins responding to Caller network clients in anticipation of Accept() calls. It
// is implicitly called by the first call to Accept() if not already called. It is only necessary to call
// this method if you need to begin accepting Callers before you make the first Accept call. Part of
// AcceptorChannelEndpoint interface.
func (ep *TCPStubEndpoint) StartListening() error {
	_, err := ep.getListener()
	return err
}

// Accept listens for and accepts a single connection from a Caller network client as specified in the
// endpoint configuration. This call does not return until a new connection is available or a
// error occurs. There is no way to cancel an Accept() request other than closing the endpoint. Part of
// the AcceptorChannelEndpoint interface.
func (ep *TCPStubEndpoint) Accept(ctx context.Context) (ChannelConn, error) {
	listener, err := ep.getListener()
	if err != nil {
		return nil, err
	}

	netConn, err := listener.Accept()
	if err != nil {
		return nil, fmt.Errorf("%s: Accept failed: %s", ep.Logger.Prefix(), err)
	}

	conn, err := NewSocketConn(ep.Logger, netConn)
	if err != nil {
		return nil, fmt.Errorf("%s: Unable to create SocketConn: %s", ep.Logger.Prefix(), err)
	}
	ep.AddShutdownChild(conn)
	return conn, nil
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
func (ep *TCPStubEndpoint) AcceptAndServe(ctx context.Context, calledServiceConn ChannelConn) (int64, int64, error) {
	callerConn, err := ep.Accept(ctx)
	if err != nil {
		calledServiceConn.Close()
		return 0, 0, err
	}
	return BasicBridgeChannels(ctx, ep.Logger, callerConn, calledServiceConn)
}
