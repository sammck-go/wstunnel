package chshare

import (
	"context"
	"net"
)

// TCPSkeletonEndpoint implements a local TCP skeleton
type TCPSkeletonEndpoint struct {
	// Implements LocalSkeletonChannelEndpoint
	BasicEndpoint
}

// NewTCPSkeletonEndpoint creates a new TCPSkeletonEndpoint
func NewTCPSkeletonEndpoint(logger Logger, ced *ChannelEndpointDescriptor) (*TCPSkeletonEndpoint, error) {
	ep := &TCPSkeletonEndpoint{
		BasicEndpoint: BasicEndpoint{
			ced: ced,
		},
	}
	ep.InitBasicEndpoint(logger, ep, "TCPSkeletonEndpoint: %s", ced)
	return ep, nil
}

// HandleOnceShutdown will be called exactly once, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
func (ep *TCPSkeletonEndpoint) HandleOnceShutdown(completionErr error) error {
	return completionErr
}

// Dial initiates a new connection to a Called Service. Part of the
// DialerChannelEndpoint interface
func (ep *TCPSkeletonEndpoint) Dial(ctx context.Context, extraData []byte) (ChannelConn, error) {
	ep.DLogf("Dialing local TCP service at %s", ep.ced.Path)

	if ep.IsStartedShutdown() {
		err := ep.Errorf("Endpoint is closed: %s", ep.String())
		return nil, err
	}

	// TODO: make sure IPV6 works
	var d net.Dialer
	netConn, err := d.DialContext(ctx, "tcp", ep.ced.Path)
	if err != nil {
		return nil, ep.Errorf("DialContext failed: %s", err)
	}

	conn, err := NewSocketConn(ep.Logger, netConn)
	if err != nil {
		return nil, ep.Errorf("Unable to create SocketConn: %s", err)
	}

	ep.AddShutdownChild(conn)

	ep.DLogf("Connected to local TCP service %s", ep.String())
	return conn, nil
}

// DialAndServe initiates a new connection to a Called Service as specified in the
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
func (ep *TCPSkeletonEndpoint) DialAndServe(
	ctx context.Context,
	callerConn ChannelConn,
	extraData []byte,
) (int64, int64, error) {
	calledServiceConn, err := ep.Dial(ctx, extraData)
	if err != nil {
		callerConn.Close()
		return 0, 0, err
	}
	return BasicBridgeChannels(ctx, ep.Logger, callerConn, calledServiceConn)
}
