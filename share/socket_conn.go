package chshare

import (
	"fmt"
	"net"
	"sync/atomic"
)

// SocketConn implements a local TCP or Unix Domain ChannelConn
type SocketConn struct {
	BasicConn
	netConn net.Conn
}

// NewSocketConn creates a new SocketConn
func NewSocketConn(logger Logger, netConn net.Conn) (*SocketConn, error) {
	c := &SocketConn{
		netConn: netConn,
	}
	c.InitBasicConn(logger, c, "SocketConn(%s)", netConn.RemoteAddr())
	return c, nil
}

// CloseWrite shuts down the writing side of the "socket". Corresponds to net.TCPConn.CloseWrite().
// this method is called when end-of-stream is reached reading from the other ChannelConn of a pair
// pair are connected via a ChannelPipe. It allows for protocols like HTTP 1.0 in which a client
// sends a request, closes the write side of the socket, then reads the response, and a server reads
// a request until end-of-stream before sending a response. Part of the ChannelConn interface
func (c *SocketConn) CloseWrite() error {
	var err error
	whc, _ := c.netConn.(WriteHalfCloser)
	if whc != nil {
		err = whc.CloseWrite()
		if err != nil {
			err = c.Errorf("CloseWrite falied: %s", err)
		}
	} else {
		c.DLogf("CloseWrite() ignored--not implemented by net.Conn implementer")
	}
	return err
}

// HandleOnceShutdown will be called exactly once, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
func (c *SocketConn) HandleOnceShutdown(completionErr error) error {
	err := c.netConn.Close()
	if err != nil {
		err = fmt.Errorf("%s: %s", c.Logger.Prefix(), err)
	}
	if completionErr == nil {
		completionErr = err
	}
	return completionErr
}

// WaitForClose blocks until the Close() method has been called and completed
func (c *SocketConn) WaitForClose() error {
	return c.WaitShutdown()
}

// Read implements the Reader interface
func (c *SocketConn) Read(p []byte) (n int, err error) {
	n, err = c.netConn.Read(p)
	atomic.AddInt64(&c.NumBytesRead, int64(n))
	return n, err
}

// Write implements the Writer interface
func (c *SocketConn) Write(p []byte) (n int, err error) {
	n, err = c.netConn.Write(p)
	atomic.AddInt64(&c.NumBytesWritten, int64(n))
	return n, err
}
