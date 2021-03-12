package wstnet

import (
	"io"

	"github.com/sammck-go/asyncobj"
)

// Bipipe is a virtual open bidirectional stream "socket", either:
//      1) a local net.Conn socket or similar abstraction created by a ChannelEndpoint to wrap communication with a local network resource
//      2) a virtual tunnel created by the proxy session to wrap a single ssh.Conn communication channel with a remote endpoint
//
// When a channel is opened, the local proxy creates one of each of these two types of Bipipes and forwards traffic between them
// in both directions. The remote proxy does something similar, with the result being a complete bidirectional connection between
// a local resource and a remote resource.
//
// As a special case, the local proxy forwards doirectly between two virtual tunnel Bipipes (case (2) above) to achieve loopback
// channels.
//
// The Bipipe interface intentionally looks ands acts like a TCP socket and overlaps substantially with net.Conn, so that a net.Conn
// can be wrapped into a wstnet.Channel trivially. Like a net.Conn, the write side of the channel may
// be closed before the read side is closed, to allow for patterns like HTTP 1.0's write request, close write, read response model.
//
type Bipipe interface {
	// Standard bidirectional i/o. Read() will return 0 bytes at end of stream.
	io.ReadWriteCloser

	// Allows the write side of the Bipipe to be closed (so remove reader will get 0 bytes) while
	// allowing local reads to continue. reads to continue
	WriteHalfCloser

	// Allows asynchronous shutdown/close of the connection with an advisory error to be subsequently
	// returned from Close() or Shutdown. After shutdown is started, reads and writes should complete quickly
	// with or without errors. On completion of Shutdown(), all resources should be freed.
	asyncobj.AsyncShutdowner
}
