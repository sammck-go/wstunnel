package wstchannel

import (
	"fmt"
	"sync/atomic"
)

// BasicConn is a base common implementation for local ChannelConn. It is not intended to be instanciated
// directly, but rather is impledded in more concrete implementations.
type BasicConn struct {
	// ChannelConn
	AsyncHelper
	ID              uint64
	Strname         string
	NumBytesRead    uint64
	NumBytesWritten uint64
}

// InitBasicConn initializes the BasicConn portion of a new connection object.  Does not Activate().
func (c *BasicConn) InitBasicConn(
	o HandleOnceActivateShutdowner,
	logger Logger,
	namef string, args ...interface{},
) {
	c.ID = AllocBasicConnID()
	c.Strname = fmt.Sprintf("[%d]", c.ID) + fmt.Sprintf(namef, args...)
	c.NumBytesRead = 0
	c.NumBytesWritten = 0
	c.AsyncHelper.InitHelper(logger.ForkLogStr(c.Strname), o)
}

// InitAndActivateBasicConn initializes the BasicConn portion of a new connection object, then calls Activate().
func (c *BasicConn) InitAndActivateBasicConn(
	o HandleOnceActivateShutdowner,
	logger Logger,
	namef string, args ...interface{},
) error {
	c.InitBasicConn(o, logger, namef, args...)
	return o.Activate()
}

// NewBasicConn Creates and initializes the BasicConn portion of a new connection object.  Does not Activate().
func NewBasicConn(
	o HandleOnceActivateShutdowner,
	logger Logger,
	namef string, args ...interface{},
) *BasicConn {
	id := AllocBasicConnID()
	c := &BasicConn{
		ID:              id,
		Strname:         fmt.Sprintf("[%d]", id) + fmt.Sprintf(namef, args...),
		NumBytesRead:    0,
		NumBytesWritten: 0,
	}
	c.AsyncHelper.InitHelper(logger.ForkLogStr(c.Strname), o)
	return c
}

// GetConnID returns a unique identifier of this connection. Identifiers are never reused for the life of the process.
func (c *BasicConn) GetConnID() uint64 {
	return c.ID
}

// GetNumBytesRead returns the number of bytes read so far on a ChannelConn
func (c *BasicConn) GetNumBytesRead() uint64 {
	return atomic.LoadUint64(&c.NumBytesRead)
}

// GetNumBytesWritten returns the number of bytes written so far on a ChannelConn
func (c *BasicConn) GetNumBytesWritten() uint64 {
	return atomic.LoadUint64(&c.NumBytesWritten)
}

// AddNumBytesRead adds to the number of bytes read, in a threadsafe way
// returns the new value.
func (c *BasicConn) AddNumBytesRead(delta uint64) uint64 {
	return atomic.AddUint64(&c.NumBytesRead, delta)
}

// AddNumBytesWritten adds to the number of bytes written, in a threadsafe way
// returns the new value.
func (c *BasicConn) AddNumBytesWritten(delta uint64) uint64 {
	return atomic.AddUint64(&c.NumBytesWritten, delta)
}

// String returns a short descriptive name for the connection, suitable for logging
func (c *BasicConn) String() string {
	return c.Strname
}

var nextBasicConnID uint64

// AllocBasicConnID allocates a unique ChannelConn ID number, for logging purposes
func AllocBasicConnID() uint64 {
	return atomic.AddUint64(&nextBasicConnID, 1)
}
