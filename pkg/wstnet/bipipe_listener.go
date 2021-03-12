package wstnet

import (
	"github.com/sammck-go/asyncobj"
)

// BipipeListener is a virtual Bipipe factory that can produce Bipipes by "listening" for abstract connection
// requests. It can be either:
//      1) a wrapped local net.Listener or similar abstraction created by a ChannelEndpoint to receive connections from local network clients
//      2) a listener created by the proxy to receive virtual tunnel connect requests, producing a ssh.Conn communication channel to a remote endpoint
//         for each connection
//
//
type BipipeListener interface {
	// Allows asynchronous shutdown/close of the listener with an advisory error to be subsequently
	// returned from Close() or Shutdown. After shutdown is started, Accept() should complete quickly
	// with or without an error. On completion of Shutdown(), all resources should be freed.
	asyncobj.AsyncShutdowner

	// Accepts a single new incoming connection from a dialing client. Any returned error indicates permanent
	// failure of the listener and will result in the listener being shut down if it is not already. After
	// StartShutdown() is called, this will quickly return with an error.
	Accept() (Bipipe, error)
}
