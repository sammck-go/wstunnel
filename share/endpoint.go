package chshare

import (
	"context"
	"fmt"
	"io"
)

// ChannelEndpoint is a virtual network endpoint service of any type and role. Stub endpoints
// are able to listen for and accept connections from local network clients, and proxy
// them to a remote endpoint. Skeleton endpoints are able to accept connection requests from
// remote endpoints and proxy them to local network services.
type ChannelEndpoint interface {
	io.Closer
	AsyncShutdowner
}

// AcceptorChannelEndpoint is a ChannelEndpoint that can be asked to accept and return connections from
// a Caller network client as expected by the endpoint configuration.
type AcceptorChannelEndpoint interface {
	ChannelEndpoint

	// StartListening begins responding to Caller network clients in anticipation of Accept() calls. It
	// is implicitly called by the first call to Accept() if not already called. It is only necessary to call
	// this method if you need to begin accepting Callers before you make the first Accept call.
	StartListening() error

	// Accept listens for and accepts a single connection from a Caller network client as specified in the
	// endpoint configuration. This call does not return until a new connection is available or a
	// error occurs. There is no way to cancel an Accept() request other than closing the endpoint
	Accept(ctx context.Context) (ChannelConn, error)

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
	AcceptAndServe(ctx context.Context, calledServiceConn ChannelConn) (int64, int64, error)
}

// DialerChannelEndpoint is a ChannelEndpoint that can be asked to create a new connection to a network service
// as expected in the endpoint configuration.
type DialerChannelEndpoint interface {
	ChannelEndpoint

	// Dial initiates a new connection to a Called Service
	Dial(ctx context.Context, extraData []byte) (ChannelConn, error)

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
	DialAndServe(
		ctx context.Context,
		callerConn ChannelConn,
		extraData []byte,
	) (int64, int64, error)
}

// LocalStubChannelEndpoint is an AcceptorChannelEndpoint that accepts connections from local network clients
type LocalStubChannelEndpoint interface {
	AcceptorChannelEndpoint
}

// LocalSkeletonChannelEndpoint is a Dialer that connects to local network services
type LocalSkeletonChannelEndpoint interface {
	DialerChannelEndpoint
}

// BasicEndpoint is a base common implementation for local ChannelEndPoints
type BasicEndpoint struct {
	ShutdownHelper
	Strname string
	ced     *ChannelEndpointDescriptor
}

// InitBasicEndpoint initializes a BasicEndpoint
func (ep *BasicEndpoint) InitBasicEndpoint(
	logger Logger,
	shutdownHandler OnceShutdownHandler,
	namef string,
	args ...interface{},
) {
	ep.Strname = fmt.Sprintf(namef, args...)
	ep.InitShutdownHelper(logger.Fork("%s", ep.Strname), shutdownHandler)
	ep.PanicOnError(ep.Activate())
}

func (ep *BasicEndpoint) String() string {
	return ep.Strname
}

// NewLocalStubChannelEndpoint creates a LocalStubChannelEndpoint from its descriptor
func NewLocalStubChannelEndpoint(
	logger Logger,
	env LocalChannelEnv,
	ced *ChannelEndpointDescriptor,
) (LocalStubChannelEndpoint, error) {
	var ep LocalStubChannelEndpoint
	var err error

	if ced.Role != ChannelEndpointRoleStub {
		err = fmt.Errorf("%s: Role must be stub: %s", logger.Prefix(), ced.LongString())
	} else if ced.Type == ChannelEndpointTypeStdio {
		if env.IsServer() {
			err = fmt.Errorf("%s: stdio endpoints are not allowed on the server side: %s", logger.Prefix(), ced.LongString())
		} else {
			ep, err = NewStdioStubEndpoint(logger, ced)
		}
	} else if ced.Type == ChannelEndpointTypeLoop {
		loopServer := env.GetLoopServer()
		if loopServer == nil {
			err = fmt.Errorf("%s: Loop endpoints are disabled: %s", logger.Prefix(), ced.LongString())
		} else {
			ep, err = NewLoopStubEndpoint(logger, ced, loopServer)
		}
	} else if ced.Type == ChannelEndpointTypeTCP {
		ep, err = NewTCPStubEndpoint(logger, ced)
	} else if ced.Type == ChannelEndpointTypeUnix {
		ep, err = NewUnixStubEndpoint(logger, ced)
	} else if ced.Type == ChannelEndpointTypeSocks {
		err = fmt.Errorf("%s: Socks endpoint Role must be skeleton: %s", logger.Prefix(), ced.LongString())
	} else {
		err = fmt.Errorf("%s: Unsupported endpoint type '%s': %s", logger.Prefix(), ced.Type, ced.LongString())
	}

	return ep, err
}

// NewLocalSkeletonChannelEndpoint creates a LocalSkeletonChannelEndpoint from its descriptor
func NewLocalSkeletonChannelEndpoint(
	logger Logger,
	env LocalChannelEnv,
	ced *ChannelEndpointDescriptor,
) (LocalSkeletonChannelEndpoint, error) {
	var ep LocalSkeletonChannelEndpoint
	var err error

	if ced.Role != ChannelEndpointRoleSkeleton {
		err = fmt.Errorf("%s: Role must be skeleton: %s", logger.Prefix(), ced.LongString())
	} else if ced.Type == ChannelEndpointTypeStdio {
		if env.IsServer() {
			err = fmt.Errorf("%s: stdio endpoints are not allowed on the server side: %s", logger.Prefix(), ced.LongString())
		} else {
			ep, err = NewStdioSkeletonEndpoint(logger, ced)
		}
	} else if ced.Type == ChannelEndpointTypeLoop {
		loopServer := env.GetLoopServer()
		if loopServer == nil {
			err = fmt.Errorf("%s: Loop endpoints are disabled: %s", logger.Prefix(), ced.LongString())
		} else {
			ep, err = NewLoopSkeletonEndpoint(logger, ced, loopServer)
		}
	} else if ced.Type == ChannelEndpointTypeTCP {
		ep, err = NewTCPSkeletonEndpoint(logger, ced)
	} else if ced.Type == ChannelEndpointTypeUnix {
		ep, err = NewUnixSkeletonEndpoint(logger, ced)
	} else if ced.Type == ChannelEndpointTypeSocks {
		socksServer := env.GetSocksServer()
		if socksServer == nil {
			err = fmt.Errorf("%s: socks endpoints are disabled: %s", logger.Prefix(), ced.LongString())
		} else {
			ep, err = NewSocksSkeletonEndpoint(logger, ced, socksServer)
		}
	} else {
		err = fmt.Errorf("%s: Unsupported endpoint type '%s': %s", logger.Prefix(), ced.Type, ced.LongString())
	}

	return ep, err
}
