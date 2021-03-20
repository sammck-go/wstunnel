package wstnet

import (
	"context"
	"fmt"
	"net"

	"github.com/sammck-go/asyncobj"
	"github.com/sammck-go/logger"
)

// =========================================================

type startNetBipipeListenerCallback func() (net.Listener, error)

type NetBipipeAcceptConnectionInfo struct {
	localAddress  string
	remoteAddress string
}

type netBipipeListener struct {
	// Implements bipipeListener
	*asyncobj.Helper
	nl   net.Listener
	name string
	slcb startNetBipipeListenerCallback

	// newConns is a chan through which new connections created by the low-level acceptor
	// goroutine are delivered to Accept() calls. It is unbuffered, so at most one connection
	// is created ahead of Accept().  In the future, a buffer size could be configured
	// to allow a backlog of connections to be accepted.
	newConns chan net.Conn

	// acceptorDone is signalled when the lowlevel acceptor goroutine exits. It is nil until
	// the goroutine is started
	acceptorDone chan struct{}

	// cleanClose is set to true when shutdown is started if shutdown was not caused by an error. This
	// is used in error handling for Accept()
	cleanClose bool
}

func NewNetBipipeListenerWithStartCallback(
	logger logger.Logger,
	name string,
	startCallback startNetBipipeListenerCallback,
) *netBipipeListener {
	l := &netBipipeListener{
		nl:           nil,
		name:         name,
		slcb:         startCallback,
		newConns:     make(chan net.Conn),
		acceptorDone: nil,
		cleanClose:   false,
	}
	l.Helper = asyncobj.NewHelper(logger, l)
	return l
}

// NewNetBipipeListener creates a BipipeListener that will accept incomming net.Conn connections.
// The network must be "tcp", "tcp4", "tcp6", or "unix".
//
// For TCP networks, if the host in the address parameter is empty or a literal unspecified IP address,
// the listener listens on all available unicast and anycast IP addresses of the local system. To only
// use IPv4, use network "tcp4". The address can use a host name, but this is not recommended,
// because it will create a listener for at most one of the host's IP addresses. If the port in the
// address parameter is empty or "0", as in "127.0.0.1:" or "[::1]:0", a port number is automatically chosen
// at StartListening time.
//
// For Unix domain sockets (network "unix"), the address parameter is the local filesystem pathname
// of the unix domain socket. if network "unix" is specified and lockUnixSocket is true, then an additional
// file with a ".lock" extension will be created and locked with flock
func NewNetBipipeListener(
	logger logger.Logger,
	network string,
	address string,
	lockUnixSocket bool,
) *netBipipeListener {
	name := fmt.Sprintf("%s:%s", network, address)
	return NewNetBipipeListenerWithStartCallback(
		logger,
		name,
		func() (net.Listener, error) {
			return net.Listen(network, address)
		},
	)
	l := &netBipipeListener{
		nl:           nil,
		name:         name,
		slcb:         startCallback,
		newConns:     make(chan net.Conn),
		acceptorDone: nil,
		cleanClose:   false,
	}
	l.Helper = asyncobj.NewHelper(logger, l)
	return l
}

func (l *netBipipeListener) String() string {
	return l.name
}

// HandleOnceShutdown will be called exactly once, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
func (l *netBipipeListener) HandleOnceShutdown(completionErr error) error {
	l.Lock.Lock()
	l.cleanClose = (completionErr == nil)
	l.Lock.Unlock()

	if l.nl != nil {
		err := l.nl.Close()
		if completionErr == nil {
			completionErr = err
		}

		if l.acceptorDone != nil {
			// Wait for the acceptor goroutine to exit. It will exit
			// quickly because we closed the net.Listener already.
			// After this, no new pre-accepted connections will be added.
			<-l.acceptorDone

			// drain and abandon any pre-accepted connections
		DRAIN:
			for {
				select {
				case nc := <-l.newConns:
					nc.Close()
				default:
					break DRAIN
				}
			}
		}
	}

	close(l.newConns)

	return completionErr
}

func (l *netBipipeListener) HandleOnceActivate() error {
	nl, err := l.slcb()
	l.Lock.Lock()
	l.nl = nl
	l.Lock.Unlock()
	if err == nil {
		l.acceptorDone = make(chan struct{})
		go func() {
			for {
				// nl.Accept will block indefinitely, until a client connects or the net.Listener is
				// closed by HandleOnceShutdown. It is not cancellable.
				nc, err := nl.Accept()
				if err != nil {
					l.StartShutdown(err)
					break
				} else {
					// This will block until someone calls Accept or the chan is drained by HandleOnceShutdown
					l.newConns <- nc
				}

			}
			close(l.acceptorDone)
		}()
	}
	return err
}

// StartListening begins responding to dialing clients in anticipation of Accept() calls. It
// is implicitly called by the first call to Accept() if not already called. It is only necessary to call
// this method if you need to ensure success of setting up the listener (e.g., exclusive open of
// the listen port) and begin accepting Callers before you make the first Accept call. Once this call is
// made, there is no way to stop listening or prevent this BipipeListener from acknowledging connections
// from dialing clients, other than shutting down the listener.--if noone calls Accept, then clients
// will successfully connect but find the connection unresponsive. When the listener is shut down, any clients that
// connected but have not yet been accepted will be rudely disconnected.
func (l *netBipipeListener) StartListening() error {
	return l.DoOnceActivate(nil, false)
}

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
func (l *netBipipeListener) AcceptWithContext(ctx context.Context) (bp Bipipe, info BipipeConnectionInfo, err error, errIsPersistent bool) {
	err = l.StartListening()
	if err != nil {
		return nil, nil, err, true
	}
	// It's important not to DeferShutDown() here, since shutting done is the way we get unblocked.
	select {
	case <-ctx.Done():
		return nil, nil, ctx.Err(), false
	case nc := <-l.newConns:
		if nc == nil {
			// We are being shut down...
			err = l.WaitLocalShutdown()
			if err == nil {
				return nil, nil, ErrBipipeListenerClosed, true
			}
			return nil, nil, err, true
		}
		bp = NewNetConnBipipe(l.Logger, nc)
		return bp, nc.RemoteAddr, nil, false
	}
}
