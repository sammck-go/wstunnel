package chshare

import (
	"context"
	"os"
)

// StdioSkeletonEndpoint implements a local Stdio skeleton
type StdioSkeletonEndpoint struct {
	// Implements LocalSkeletonChannelEndpoint
	BasicEndpoint
	pipeConn *PipeConn
}

// NewStdioSkeletonEndpoint creates a new StdioSkeletonEndpoint
func NewStdioSkeletonEndpoint(
	logger Logger,
	ced *ChannelEndpointDescriptor,
) (*StdioSkeletonEndpoint, error) {
	ep := &StdioSkeletonEndpoint{
		BasicEndpoint: BasicEndpoint{
			ced: ced,
		},
	}
	ep.InitBasicEndpoint(logger, ep, "StdioSkeletonEndpoint")
	pipeConn, err := NewPipeConn(ep.Logger, os.Stdin, os.Stdout)
	if err != nil {
		return nil, ep.Errorf("Failed to create stdio PipeConn: %s", err)
	}
	ep.AddShutdownChild(pipeConn)
	ep.pipeConn = pipeConn
	return ep, nil
}

// HandleOnceShutdown will be called exactly once, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
func (ep *StdioSkeletonEndpoint) HandleOnceShutdown(completionErr error) error {
	err := ep.pipeConn.Close()
	if completionErr == nil {
		completionErr = err
	}
	return completionErr
}

// Dial initiates a new connection to a Called Service. Part of the
// DialerChannelEndpoint interface
func (ep *StdioSkeletonEndpoint) Dial(ctx context.Context, extraData []byte) (ChannelConn, error) {
	return ep.pipeConn, nil
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
func (ep *StdioSkeletonEndpoint) DialAndServe(
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
