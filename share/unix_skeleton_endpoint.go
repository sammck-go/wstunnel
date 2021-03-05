package chshare

import (
	"context"
	"fmt"
	"net"
)

// UnixSkeletonEndpoint implements a local Unix skeleton
type UnixSkeletonEndpoint struct {
	// Implements LocalSkeletonChannelEndpoint
	BasicEndpoint
}

// NewUnixSkeletonEndpoint creates a new UnixSkeletonEndpoint
func NewUnixSkeletonEndpoint(logger Logger, ced *ChannelEndpointDescriptor) (*UnixSkeletonEndpoint, error) {
	ep := &UnixSkeletonEndpoint{
		BasicEndpoint: BasicEndpoint{
			ced: ced,
		},
	}
	ep.InitBasicEndpoint(logger, ep, "UnixSkeletonEndpoint: %s", ced)
	return ep, nil
}

// HandleOnceShutdown will be called exactly once, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
func (ep *UnixSkeletonEndpoint) HandleOnceShutdown(completionErr error) error {
	return completionErr
}

// Dial initiates a new connection to a Called Service. Part of the
// DialerChannelEndpoint interface
func (ep *UnixSkeletonEndpoint) Dial(ctx context.Context, extraData []byte) (ChannelConn, error) {

	if ep.IsStartedShutdown() {
		err := ep.Errorf("Endpoint is closed: %s", ep.String())
		return nil, err
	}

	// TODO: make sure IPV6 works
	var d net.Dialer
	netConn, err := d.DialContext(ctx, "unix", ep.ced.Path)
	if err != nil {
		return nil, fmt.Errorf("%s: DialContext failed: %s", ep.Logger.Prefix(), err)
	}

	conn, err := NewSocketConn(ep.Logger, netConn)
	if err != nil {
		return nil, ep.Errorf("Unable to create SocketConn: %s", err)
	}
	ep.AddShutdownChild(conn)
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
func (ep *UnixSkeletonEndpoint) DialAndServe(
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
