package chshare

import (
	"context"
)

// LoopSkeletonEndpoint implements a local Loop skeleton
type LoopSkeletonEndpoint struct {
	// Implements LocalSkeletonChannelEndpoint
	BasicEndpoint
	loopServer *LoopServer
}

// NewLoopSkeletonEndpoint creates a new LoopSkeletonEndpoint
func NewLoopSkeletonEndpoint(
	logger Logger,
	ced *ChannelEndpointDescriptor,
	loopServer *LoopServer,
) (*LoopSkeletonEndpoint, error) {
	ep := &LoopSkeletonEndpoint{
		BasicEndpoint: BasicEndpoint{
			ced: ced,
		},
		loopServer: loopServer,
	}
	ep.InitBasicEndpoint(logger, ep, "LoopSkeletonEndpoint: %s", ced)
	return ep, nil
}

// GetLoopPath returns the loop pathname associated with this LoopStubEndpoint
func (ep *LoopSkeletonEndpoint) GetLoopPath() string {
	return ep.ced.Path
}

// HandleOnceShutdown will be called exactly once, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
func (ep *LoopSkeletonEndpoint) HandleOnceShutdown(completionErr error) error {
	return completionErr
}

// Dial initiates a new connection to a Called Service. Part of the
// DialerChannelEndpoint interface
func (ep *LoopSkeletonEndpoint) Dial(ctx context.Context, extraData []byte) (ChannelConn, error) {
	if ep.IsStartedShutdown() {
		return nil, ep.Errorf("Endpoint is closed")
	}
	conn, err := ep.loopServer.Dial(ctx, ep.GetLoopPath(), extraData)
	if err != nil {
		return nil, ep.Errorf("Unable to lopp-dial path \"%s\": %s", ep.GetLoopPath(), err)
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
func (ep *LoopSkeletonEndpoint) DialAndServe(
	ctx context.Context,
	callerConn ChannelConn,
	extraData []byte,
) (int64, int64, error) {
	if ep.IsStartedShutdown() {
		return 0, 0, ep.Errorf("Endpoint is closed")
	}
	return ep.loopServer.DialAndServe(ctx, ep.GetLoopPath(), callerConn, extraData)
}
