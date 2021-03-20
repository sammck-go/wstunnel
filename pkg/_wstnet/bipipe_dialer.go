package wstnet

import (
	"context"

	"github.com/sammck-go/asyncobj"
)

// BipipeDialParams is an optional immutable object containing parameters for dialing--e.g., a dns name, an IP address and port, authentication cookies, etc. Its
// meaning is specific to the PipipeDialer implementation.
type BipipeDialParams interface{}

// BipipeConnectionInfo is an immutable object containing connection metadata obtained when a connection is established--e.g., an IP address and port, remote
// authenticated identity, etc. Its meaning is specific to a BipipeDialer or BipipeListener implementation. The Bipipe it'self need not retain this
// information, though it may do so.
type BipipeConnectionInfo interface{}

// BipipeDialer is a virtual Bipipe factory that can produce Bipipes on demand by "dialing" to an abstract service. It can be either:
//      1) a wrapped local net.Dialer or similar abstraction created by a ChannelEndpoint to connect to local network services
//      2) a dialer created by the proxy to issue virtual tunnel connect requests, producing a ssh.Conn communication channel to a remote endpoint
//         for each connection
type BipipeDialer interface {
	// AsyncShutdowner Allows asynchronous shutdown/close of the listener with an advisory error to be subsequently
	// returned from Close(), Shutdown(), or WaitShutdown(). After shutdown is started, DialWithContext() should complete quickly
	// with an error. On completion of Shutdown(), all resources should be freed.
	asyncobj.AsyncShutdowner

	// Connects to the abstract service, creating a new Bipipe.
	// There may be multiple goroutines concurrently calling this method, and the completion order of concurrent requests is not
	// deterministic. One Dial request does not block or impede the progress of another.
	// "ctx" allows a Dial request to be cancelled or timed out. Cancelling this context does NOT shut down the entire BipipeDialer or
	// any other in-flight Dial request not explicitly associated with the ctx; it just abandons this particular Dial request.
	// "dialParams" is an optional object containing dialer-specific information, such as an ip address and port.
	// A returned error indicates that the connection failed, but does not affect other inflight Dial requests or prevent future
	// connections from succeeding.
	// After BipipeDialer.StartShutdown() is called, all inflight and future invocations of this method will quickly return with an error.
	// On success, a new, active Bipipe is returned, as well as an optional dialer-specific BipipeConnectionInfo containing metadata
	// about the connection--e.g., server
	// certificate info, ip address, etc..
	DialWithContext(ctx context.Context, dialParams BipipeDialParams) (Bipipe, BipipeConnectionInfo, error)
}
