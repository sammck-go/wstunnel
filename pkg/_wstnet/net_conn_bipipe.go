package wstnet

import (
	"fmt"
	"net"

	"github.com/sammck-go/asyncobj"
	"github.com/sammck-go/logger"
)

// netConnBipipe wraps a net.Conn "socket" with a Bipipe interface
type netConnBipipe struct {
	// implements Bipipe
	net.Conn
	*asyncobj.Helper
	name string
}

// NewNetConnBipipe wraps an existing net.Conn "socket" with a Bipipe interface. The returned
// Bipipe becomes the owner of the net.Conn and is responsible for closing it.
func NewNetConnBipipe(logger logger.Logger, conn net.Conn) Bipipe {
	name := fmt.Sprintf("<NetConnBipipe %v>", conn)
	bp := &netConnBipipe{
		Conn: conn,
		name: name,
	}
	bp.Helper = asyncobj.NewHelper(logger.ForkLogStr(name), bp)

	bp.SetIsActivated()

	return bp
}

func (bp *netConnBipipe) String() string {
	return bp.name
}

// Close shuts down the bipipe and waits for shutdown to complete
func (bp *netConnBipipe) Close() error {
	return bp.Helper.Close()
}

// CloseWrite closes the write side of the Bipipe, causing the remote reader to receive EOF. Does not affect the
// read side of the Bipipe. If the underlying net.Conn supports CloseWrite() (e.g., TCPConn or UnixConn), then it is
// called.  Otherwise, this method does nothing.
func (bp *netConnBipipe) CloseWrite() error {
	err := bp.DeferShutdown()
	if err == nil {
		rhc, ok := bp.Conn.(WriteHalfCloser)
		if ok {
			err = rhc.CloseWrite()
		}
		bp.UndeferShutdown()
	}
	return err
}

// HandleOnceShutdown will be called exactly once, in StateShuttingDown, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
// This method will never be called while shutdown is deferred.
func (bp *netConnBipipe) HandleOnceShutdown(completionError error) error {
	err := bp.Conn.Close()
	if completionError == nil {
		completionError = err
	}
	return completionError
}
