package wstnet

import (
	"fmt"
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
// An implementation of Bipipe may optionally implement io.WriterTo and/or io.ReaderFrom, and should do so if it adds efficiency (e.g., prevents a buffer
// copy). BipipeBridger will make use of these optimizations when possible and appropriate.
//
type Bipipe interface {
	// Stringer provides A short descriptive string is used for logging; it should be cached for speed
	fmt.Stringer

	// ReadWriteCloser provides Standard bidirectional i/o.
	// Read() will return io.EOF at end of stream.
	// Read and write can proceed concurrently with each other; however, there wiil never be more than
	// one read and one write outstanding and any time.
	// Close() is equivalent to StartShutdown(nil) followed by WaitShutdown(). Repeated calls to Close() are
	// allowed and always return the same result code. A nil response from Close() indicates that all written
	// data was delivered to the remote endpoint, and all data sent by the remote endpoint was returned
	// through Read().
	io.ReadWriteCloser

	// WriteHalfCloser allows the write side of the Bipipe to be closed (so the remote reader will get EOF) while
	// allowing local reads to continue. This supports the TCP socket pattern used by HTTP 1.0 where the client
	// closes his write stream, the server receives EOF, then the server writes the response stream back to the
	// client. May be called multiple times, but not concurrently. CloseWrite after general shutdown has started
	// has no effect and may return an error.
	// Future calls to Write() after calling WriteClose() will return an error.
	// CloseWrite cannot be called while a Write() call is inflight; behavior in such a scenario is undefined.
	WriteHalfCloser

	// Allows asynchronous shutdown/close of the Bipipe with an advisory error to be subsequently
	// returned from Close() or Shutdown. Also allows multiple callers to wait on complete shutdown.
	// After shutdown is initiated, reads and writes should complete quickly with errors.
	// a nil result from WaitShutdown() indicates that all written
	// data was delivered to the remote endpoint, and all data sent by the remote endpoint was returned
	// through Read().
	// On completion of Shutdown(), all resources should be freed.
	asyncobj.AsyncShutdowner
}
