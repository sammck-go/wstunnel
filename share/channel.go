package chshare

import (
	"context"
	"io"
	"sync"
	"sync/atomic"

	socks5 "github.com/armon/go-socks5"
	"golang.org/x/crypto/ssh"
)

// To distinguish between the various entities in a tunneled
// network environment, we adopt the vocabulary of distributed object
// communication, in particular "Stub" for the proxy's impersonation
// of a network service, and "Skeleton" for the proxy's impersonation
// of a network client.
//   (see https://en.wikipedia.org/wiki/Distributed_object_communication).
//
// A "Chisel Proxy" is an instance of this chisel application, running as either a
// "Chisel Proxy Client" or a "Chisel Proxy Server". A "Chisel Proxy Session" consists
// of a single Chisel Proxy Client and a single Chisel Proxy Server, communicating with
// one another over a single TCP connection using a encrypted SSH protocol over WebSockets.
// A Chisel Proxy Client participates in exactly on Chisel Proxy Session, but a Chisel Proxy
// Server may have many concurrent Chisel Proxy Sessions with different Chisel Proxy Clients.
// All traffic through a Chisel Proxy Session, including proxied network service
// traffic, is encrypted with the SSH protocol. Provided the private key of the Chisel Proxy
// Server is kept private, and the public key fingerprint of the Chisel Proxy Server is known
// in advance by all Chisel Proxy Clients, there is no need for additional encryption of
// proxied traffic (it may still be desirable to encrypt proxied traffic between Chisel Proxies and
// the applications that connect to the proxy).
//
// A Local Chisel Proxy is the Chisel Proxy that is directly network-reachable in the context
// of a particular discussion. It may be either a Chisel Proxy Client or a Chisel Proxy Server.
//
// A Remote Chisel Proxy is the other Chisel Proxy in the same Chisel Proxy Session as a
// given Local Chisel Proxy.
//
// A "Caller" is an originating application that wishes to connect to a logical
// network "Called Service" that can not be reached locally, but which must be reached
// through the Chisel Proxy Session. Typically this is a TCP client, though other protocols
// are supported.
//
// A "Called Service" is a logical network service that needs to be accessed by
// "Callers" that cannot reach it locally, but must reach it through a Chisel Proxy Session.
// Typically this is a TCP service listening at a particular host/port, though other
// protocols are supported.
//
// A ChannelEndpoint is a logical network endpoint on a Local Chisel Proxy that
// is paired with another ChannelEndpoint on the Remote Chisel Proxy.
// A ChannelEndpoint is either a "Stub", meaning that it impersonates a remote Called Service
// and listens for and accepts connection requests from local Callers and forwards them
// to the peered ChannelEndpoint on the Remote Chisel Proxy, or a Skeleton, meaning that it responds to
// connection requests from the Remote Chisel Proxy and impersonates the remote Caller
// by connecting to locally available Called Services.
//
// A Stub looks like a Called Service to a Caller, and a Skeleton looks like
// a Caller to a Called Service.
//
// We will refer to the combination of a Stub ChannelEndpoint and its peered remote
// Skeleton ChannelEndpoint as a "Channel Endpoint Pair". Since it is a distributed
// entity, a Channel Endpoint Pair has no direct representation in code. A Channel Endpoint
// Pair in which Stub ChannelEndpoint is on the Chisel Proxy Client is referred to
// as operating in "forward proxy mode". A Channel Endpoint Pair in which Stub
// ChannelEndpoint is on the Chisel Proxy Server is referred to as operating in
// "reverse proxy mode".
//
// Just as a socket service listener can accept and service multiple concurrent
// connections from clients, a Channel Endpoint Pair can accept and service multiple
// concurrent connections from clients at the Stub side, and proxy them to multiple
// concurrent service connections at the Skeleton side. Each individual proxied
// connection of this type is called a Channel. Traffic on each Channel is completely
// independent of and asynchronous to traffic on other Channels. all Channel
// traffic is multiplexed through a Chisel Proxy Session on the single TCP connection
// used by the Chisel Proxy Session; flow control is employed to ensure that a delay
// in reading from one Channel has no effect on the availability of other channels or on
// traffic in the opposite direction on the same channel.
//
// A ChannelEndpoint is described in a serializable form by a ChannelEndpointDescriptor.
//
// A Channel Endpoint Pair is described in serialized form by a ChannelDescriptor.
//
// One type of ChannelEndpoint that deserves special mention is a "Loop" ChannelEndpoint.
// A Loop Stub ChannelEndpoint operates very much like a Unix Domain Socket Stub (it
// listens on a specified name in a string-based namespace), except that it can only
// accept connections from Loop Skeleton ChannelEndpoints on the Same Chisel Proxy (it has
// no visibility outside of the Chisel Proxy). The advantage of Loop Channels is that
// traffic is directly forwarded between a Caller's Channel and a Called service's
// Channel, when both the Caller and the Called Service are reachable only through
// a Chisel Proxy Session. This effectively saves two network hops for traffic
// in both directions (writing to a unix domain socket and reading back from the
// same unix domain socket)
//
//                           +-----------------------------+             +--------------------------------+
//                           |     Chisel Proxy Client     |    Chisel   |     Chisel Proxy Server        |
//                           |                             |   Session   |                                |
//                           |                             | ==========> |                                |
//   +----------------+      |    +-------------------|    |             |    +--------------------+      |      +----------------+
//   | Client-side    |      |    |  forward-proxy    |    |             |    |  forward-proxy     |      |      | Server-side    |
//   | Caller App     | =====|==> |  Stub             | :::|:::::::::::::|::> |  Skeleton          | =====|====> | Called Service |
//   |                |      |    |  ChannelEndpoint  |    |  Channel(s) |    |  ChannelEndpoint   |      |      |                |
//   +----------------+      |    +-------------------+    |             |    +--------------------+      |      +----------------+
//                           |                             |             |                                |
//   +----------------+      |    +-------------------|    |             |    +--------------------+      |      +----------------+
//   | Client-side    |      |    |  reverse-proxy    |    |             |    |  reverse-proxy     |      |      | Server-side    |
//   | Called Service | <====|=== |  Skeleton         | <::|:::::::::::::|::: |  Stub              | <====|===== | Caller App     |
//   |                |      |    |  ChannelEndpoint  |    |  Channel(s) |    |  ChannelEndpoint   |      |      |                |
//   +----------------+      |    +-------------------+    |             |    +--------------------+      |      +----------------+
//                           |                             |             |                                |
//   +----------------+      |    +-------------------|    |             |    +--------------------+      |
//   | Client-side    |      |    |  forward-proxy    |    |             |    |  forward-proxy     |      |
//   | Caller App     | =====|==> |  Stub             | :::|:::::::::::::|::> |  Skeleton "Loop"   | ===\ |
//   |                |      |    |  ChannelEndpoint  |    |  Channel(s) |    |  ChannelEndpoint   |    | |
//   +----------------+      |    +-------------------+    |             |    +--------------------+    | |
//                           |                             |             |                              | |
//                           +-----------------------------|             |                              | |
//                                                                       |                              | |
//                           +-----------------------------+             |                              | |
//                           |     Chisel Proxy Client     |    Chisel   |                              | |
//                           |                             |   Session   |                              | |
//                           |                             | ==========> |                              | |
//   +----------------+      |    +-------------------|    |             |    +--------------------+    | |
//   | Client-side    |      |    |  reverse-proxy    |    |             |    |  reverse-proxy     |    | |
//   | Called Service | <====|=== |  Skeleton         | <::|:::::::::::::|::: |  Stub "Loop"       | <==/ |
//   |                |      |    |  ChannelEndpoint  |    |  Channel(s) |    |  ChannelEndpoint   |      |
//   +----------------+      |    +-------------------+    |             |    +--------------------+      |
//                           |                             |             |                                |
//                           +-----------------------------+             +--------------------------------+
//

var lastBasicBridgeNum int64 = 0

// BasicBridgeChannels connects two ChannelConn's together, copying betweeen them bi-directionally
// until end-of-stream is reached in both directions. Both channels are closed before this function
// returns. Three values are returned:
//    Number of bytes transferred from caller to calledService
//    Number of bytes transferred from calledService to caller
//    If io.Copy() returned an error in either direction, the error value.
//
// CloseWrite() is called on each channel after transfer to that channel is complete.
//
// Currently the context is not used and there is no way to cancel the bridge without closing
// one of the ChannelConn's.
func BasicBridgeChannels(
	ctx context.Context,
	logger Logger,
	caller ChannelConn,
	calledService ChannelConn,
) (int64, int64, error) {
	bridgeNum := atomic.AddInt64(&lastBasicBridgeNum, 1)
	logger = logger.Fork("BasicBridge#%d (%s->%s)", bridgeNum, caller, calledService)
	logger.DLogf("Starting")
	var callerToServiceBytes, serviceToCallerBytes int64
	var callerToServiceErr, serviceToCallerErr error
	var wg sync.WaitGroup
	wg.Add(2)
	copyFunc := func(src ChannelConn, dst ChannelConn, bytesCopied *int64, copyErr *error) {
		// Copy from caller to calledService
		*bytesCopied, *copyErr = io.Copy(dst, src)
		if *copyErr != nil {
			logger.DLogf("io.Copy(%s->%s) returned error: %s", src, dst, *copyErr)
		}
		logger.DLogf("Done with io.Copy(%s->%s); shutting down write side", src, dst)
		dst.CloseWrite()
		logger.DLogf("Done with write side shutdown of %s->%s", src, dst)
		wg.Done()
	}
	go copyFunc(caller, calledService, &callerToServiceBytes, &callerToServiceErr)
	go copyFunc(calledService, caller, &serviceToCallerBytes, &serviceToCallerErr)
	wg.Wait()
	logger.DLogf("Wait complete")
	logger.DLogf("callerToService=%d, err=%s", callerToServiceBytes, callerToServiceErr)
	logger.DLogf("serviceToCaller=%d, err=%s", serviceToCallerBytes, serviceToCallerErr)
	logger.DLogf("Closing calledService")
	calledService.Close()
	logger.DLogf("Closing caller")
	caller.Close()
	err := callerToServiceErr
	if err == nil {
		err = serviceToCallerErr
	}
	logger.DLogf("Exiting, callerToService=%d, serviceToCaller=%d, err=%s", callerToServiceBytes, serviceToCallerBytes, err)
	return callerToServiceBytes, serviceToCallerBytes, err
}

// LocalChannelEnv provides necessary context for initialization of local channel endpoints
type LocalChannelEnv interface {
	// IsServer returns true if this is a proxy server; false if it is a cliet
	IsServer() bool

	// GetLoopServer returns the shared LoopServer if loop protocol is enabled; nil otherwise
	GetLoopServer() *LoopServer

	// GetSocksServer returns the shared socks5 server if socks protocol is enabled;
	// nil otherwise
	GetSocksServer() *socks5.Server

	// GetSSHConn waits for and returns the main ssh.Conn that this proxy is using to
	// communicate with the remote proxy. It is possible that goroutines servicing
	// local stub sockets will ask for this before it is available (if for example
	// a listener on the client accepts a connection before the server has ackknowledged
	// configuration. An error response indicates that the SSH connection failed to initialize.
	GetSSHConn() (ssh.Conn, error)
}
