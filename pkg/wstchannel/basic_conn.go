package wstchannel

import (
	"fmt"
	"sync/atomic"
)

var nextBasicConnID int32

// AllocBasicConnID allocates a unique ChannelConn ID number, for logging purposes
func AllocBasicConnID() int32 {
	return atomic.AddInt32(&nextBasicConnID, 1)
}

// BasicConn is a base common implementation for local ChannelConn
type BasicConn struct {
	// ChannelConn
	ShutdownHelper
	ID              int32
	Strname         string
	NumBytesRead    int64
	NumBytesWritten int64
}

// InitBasicConn initializes the BasicConn portion of a new connection object
func (c *BasicConn) InitBasicConn(
	logger Logger,
	shutdownHandler OnceShutdownHandler,
	namef string, args ...interface{}) {
	c.ID = AllocBasicConnID()
	c.Strname = fmt.Sprintf("[%d]", c.ID) + fmt.Sprintf(namef, args...)
	c.InitShutdownHelper(logger.Fork("%s", c.Strname), shutdownHandler)
	c.PanicOnError(c.Activate())
}

// GetNumBytesRead returns the number of bytes read so far on a ChannelConn
func (c *BasicConn) GetNumBytesRead() int64 {
	return atomic.LoadInt64(&c.NumBytesRead)
}

// GetNumBytesWritten returns the number of bytes written so far on a ChannelConn
func (c *BasicConn) GetNumBytesWritten() int64 {
	return atomic.LoadInt64(&c.NumBytesWritten)
}

// GetNumBytesWritten returns the number of bytes written so far on a ChannelConn
func (c *BasicConn) String() string {
	return c.Strname
}
