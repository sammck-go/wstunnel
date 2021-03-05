package chshare

// Implementation of a wrapper turning ssh.Channel into a ChannelConn

import (
	"fmt"
	"golang.org/x/crypto/ssh"
	"sync/atomic"
)

// SSHConn implements a local TCP or Unix Domain ChannelConn
type SSHConn struct {
	BasicConn
	rawSSHConn ssh.Channel
}

// NewSSHConn creates a new SSHConn
func NewSSHConn(logger Logger, rawSSHConn ssh.Channel) (*SSHConn, error) {
	c := &SSHConn{
		rawSSHConn: rawSSHConn,
	}
	c.InitBasicConn(logger, c, "SSHConn")
	return c, nil
}

// CloseWrite shuts down the writing side of the "socket". Corresponds to net.TCPConn.CloseWrite().
// this method is called when end-of-stream is reached reading from the other ChannelConn of a pair
// pair are connected via a ChannelPipe. It allows for protocols like HTTP 1.0 in which a client
// sends a request, closes the write side of the socket, then reads the response, and a server reads
// a request until end-of-stream before sending a response. Part of the ChannelConn interface
func (c *SSHConn) CloseWrite() error {
	err := c.rawSSHConn.CloseWrite()
	if err != nil {
		err = fmt.Errorf("%s: %s", c.Logger.Prefix(), err)
	}
	return err
}

// HandleOnceShutdown will be called exactly once, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
func (c *SSHConn) HandleOnceShutdown(completionErr error) error {
	err := c.rawSSHConn.Close()
	if err != nil {
		err = c.Errorf("%s", err)
	}
	if completionErr == nil {
		completionErr = err
	}
	return completionErr
}

// WaitForClose blocks until the Close() method has been called and completed
func (c *SSHConn) WaitForClose() error {
	return c.WaitShutdown()
}

// Read implements the Reader interface
func (c *SSHConn) Read(p []byte) (n int, err error) {
	n, err = c.rawSSHConn.Read(p)
	atomic.AddInt64(&c.NumBytesRead, int64(n))
	return n, err
}

// Write implements the Writer interface
func (c *SSHConn) Write(p []byte) (n int, err error) {
	n, err = c.rawSSHConn.Write(p)
	atomic.AddInt64(&c.NumBytesWritten, int64(n))
	return n, err
}
