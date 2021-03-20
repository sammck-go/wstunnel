package wstnet

import (
	"context"
	"errors"

	"github.com/sammck-go/asyncobj"
)

// ErrBipipeListenerClosed is a well-known error value that is returned from BipipeListener.Accept() if the listener is cleanly shut down.
// it can be tested for as a special case with ==, to distinguish between clean shutdown of the Listener and a true failure.
var ErrBipipeListenerClosed = errors.New("BipipeListener was Closed")

// BipipeListener is a virtual Bipipe factory/manager that can produce Bipipes by "listening" for abstract connection
// requests. It can be either:
//      1) a wrapped local net.Listener or similar abstraction created by a ChannelEndpoint to receive connections from local network clients
//      2) a listener created by the proxy to receive virtual tunnel connect requests, producing a ssh.Conn communication channel to a remote endpoint
//         for each connection
type BipipeListener interface {
	// Allows asynchronous shutdown/close of the listener with an advisory error to be subsequently
	// returned from Close() or Shutdown. After shutdown is started, Accept() should complete quickly
	// with or without an error. On completion of Shutdown(), all resources should be freed.
	asyncobj.AsyncShutdowner

	// StartListening begins responding to dialing clients in anticipation of Accept() calls. It
	// is implicitly called by the first call to Accept() if not already called. It is only necessary to call
	// this method if you need to ensure success of setting up the listener (e.g., exclusive open of
	// the listen port) and begin accepting Callers before you make the first Accept call. Once this call is
	// made, there is no way to stop listening or prevent this BipipeListener from acknowledging connections
	// from dialing clients, other than shutting down the listener.--if noone calls Accept, then clients
	// will successfully connect but find the connection unresponsive. When the listener is shut down, any clients that
	// connected but have not yet been accepted will be rudely disconnected.
	// If this method returns an error, the BipipeListener is persistently failed and will be automatically shut down
	StartListening() error

	// AcceptWithContext accepts a single new incoming connection from a dialing client and presents it as a Bipipe.
	// To pipeline processing of incoming connect requests, there may be multiple goroutines concurrently calling this method,
	// and the completion order of concurrent requests is not deterministic, though it is typical for incoming connections to
	// be assigned to callers in the order that the calls were issued. One Accept request receives one connected Bipipe, and
	// each connected Bipipe is delivered to only one Accept() caller.
	// A returned error with errIsPersistent==true indicates that the BipipeListener has persistently failed or has been shut down.
	// In this case, all other incomplete and future calls to Accept() will fail quickly with the same error code, and the
	// BipipeListener will automatically be shut down (asynchronously).
	// After BipipeDialer.StartShutdown() is called, all inflight and future invocations of this method will quickly return with an error.
	// As a special case, if the error returned is ErrBipipeListenerClosed, the caller is assured that the reason for failure of the Accept()
	// call is that the BipipeListener was Closed or was ShutDown with no error code, which is typically an expected event during clean
	// shutdown.
	// On success, a new, active Bipipe is returned, as well as an optional listener-specific BipipeConnectionInfo containing metadata
	// about the connection--e.g., server certificate info, ip address, etc..
	// "ctx" allows a particular Accept call to be cancelled before an incoming connection is received; however, this does not actually
	// prevent this BipipeListener from acknowledging incoming connections from dialing clients--if
	// noone else calls Accept, then clients will successfully connect but find the connections unresponsive. When the listener
	// is shut down, any clients that have connected but have not yet been accepted will be rudely disconnected.
	AcceptWithContext(ctx context.Context) (bp Bipipe, info BipipeConnectionInfo, err error, errIsPersistent bool)
}
