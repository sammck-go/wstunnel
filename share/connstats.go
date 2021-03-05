package chshare

import (
	"fmt"
	"sync/atomic"
)

// ConnStats keep track of both currently open and total connection counts for an entity
type ConnStats struct {
	count int32
	open  int32
}

// New adds one to the total connection count in a ConnStats
func (c *ConnStats) New() int32 {
	return atomic.AddInt32(&c.count, 1)
}

// Open adds one to the current open connection count in a ConnStats
func (c *ConnStats) Open() {
	atomic.AddInt32(&c.open, 1)
}

// Close subtracts one from the current open connection count in a ConnStats
func (c *ConnStats) Close() {
	atomic.AddInt32(&c.open, -1)
}

func (c *ConnStats) String() string {
	return fmt.Sprintf("[%d/%d]", atomic.LoadInt32(&c.open), atomic.LoadInt32(&c.count))
}
