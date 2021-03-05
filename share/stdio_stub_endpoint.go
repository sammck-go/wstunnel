package chshare

import (
	"context"
	"os"
)

// StdioStubEndpoint implements a local Stdio stub
type StdioStubEndpoint struct {
	// Implements LocalStubChannelEndpoint
	BasicEndpoint
	pipeConn *PipeConn
}

// NewStdioStubEndpoint creates a new StdioStubEndpoint
func NewStdioStubEndpoint(
	logger Logger,
	ced *ChannelEndpointDescriptor,
) (*StdioStubEndpoint, error) {
	ep := &StdioStubEndpoint{
		BasicEndpoint: BasicEndpoint{
			ced: ced,
		},
	}
	ep.InitBasicEndpoint(logger, ep, "StdioStubEndpoint")
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
func (ep *StdioStubEndpoint) HandleOnceShutdown(completionErr error) error {
	err := ep.pipeConn.Close()
	if completionErr == nil {
		completionErr = err
	}
	return completionErr
}

// StartListening begins responding to Caller network clients in anticipation of Accept() calls. It
// is implicitly called by the first call to Accept() if not already called. It is only necessary to call
// this method if you need to begin accepting Callers before you make the first Accept call. Part of
// AcceptorChannelEndpoint interface.
func (ep *StdioStubEndpoint) StartListening() error {
	return nil
}

// Accept listens for and accepts a single connection from a Caller network client as specified in the
// endpoint configuration. This call does not return until a new connection is available or a
// error occurs. There is no way to cancel an Accept() request other than closing the endpoint. Part of
// the AcceptorChannelEndpoint interface.
func (ep *StdioStubEndpoint) Accept(ctx context.Context) (ChannelConn, error) {
	return ep.pipeConn, nil
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
func (ep *StdioStubEndpoint) AcceptAndServe(ctx context.Context, calledServiceConn ChannelConn) (int64, int64, error) {
	callerConn, err := ep.Accept(ctx)
	if err != nil {
		calledServiceConn.Close()
		return 0, 0, err
	}
	return BasicBridgeChannels(ctx, ep.Logger, callerConn, calledServiceConn)
}
