package chshare

// Simple wrapper to make any ChannelConn look enough like a
// net.Conn to satisfy the socks5 server, which only takes
// net.Conn connections.
//

import (
	"fmt"
	"net"
	"time"
)

type channelWrapper struct {
	ChannelConn
	buff []byte
}

// NewChannelConnToNetConnWrapper thinly wraps a ChannelConn so it looks
// enough like net.Conn to fool a socks5 server. Also implements
// CloseWrite, which is not part of net.Conn but which is explicitly
// checked for by the socks5 server. Not a full wrapping
func NewChannelConnToNetConnWrapper(channelConn ChannelConn) net.Conn {
	c := channelWrapper{
		ChannelConn: channelConn,
	}
	return &c
}

func (c *channelWrapper) LocalAddr() net.Addr {
	return c
}

func (c *channelWrapper) RemoteAddr() net.Addr {
	return c
}

func (c *channelWrapper) Network() string {
	return "tcp"
}

func (c *channelWrapper) String() string {
	return fmt.Sprintf("%v", c.ChannelConn)
}

func (c *channelWrapper) SetDeadline(t time.Time) error {
	return nil //no-op
}

func (c *channelWrapper) SetReadDeadline(t time.Time) error {
	return nil //no-op
}

func (c *channelWrapper) SetWriteDeadline(t time.Time) error {
	return nil //no-op
}
