package wstnet

import (
	"context"

	"github.com/sammck-go/asyncobj"
)

// BipipeDialParams is an optional object containing parameters for dialing--e.g., an IP address and port. It's
// meaning is specific to the PipipeDialer implementation.
type BipipeDialParams interface{}

// BipipeDialer is a virtual Bipipe factory that can produce Bipipes on demand by "dialing" to an abstract service. It can be either:
//      1) a wrapped local net.Dialer or similar abstraction created by a ChannelEndpoint to connect to local network services
//      2) a dialer created by the proxy to issue virtual tunnel connect requests, producing a ssh.Conn communication channel to a remote endpoint
//         for each connection
//
//
type BipipeDialer interface {
	// Allows asynchronous shutdown/close of the listener with an advisory error to be subsequently
	// returned from Close() or Shutdown. After shutdown is started, DialContext() should complete quickly
	// with an error. On completion of Shutdown(), all resources should be freed.
	asyncobj.AsyncShutdowner

	// Connects to the abstract service, creating a new Bipipe.
	// ctx allows a Dial request to be cancelled or timed out.
	// dialParams is an optional object containing dialer-specific information, such as an ip address and port.
	// A returned error indicates that the connection failed, but does not prevent future connections from succeeding.
	// After StartShutdown() is called, this will quickly return with an error.
	DialContext(ctx context.Context, dialParams BipipeDialParams) (Bipipe, error)
}
