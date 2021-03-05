package chshare

// ReadHalfCloser is an interface for bidirectional io streams that implement CloseRead()
type ReadHalfCloser interface {
	// CloseRead shuts down the reading half of a bidirectional io stream (e.g., "socket").
	// Corresponds to net.TCPConn.CloseRead(). This method is called by the reader to
	// indicate no further reads will be coming after this call. The remote writer, if
	// any, will receive an error on further attempts to write to the stream. However, the
	// write half of the bidirectional stream remains active. This method has few practical
	// uses (unlike CloseWrite), but is included here for completeness.
	CloseRead() error
}

// WriteHalfCloser is an interface for bidirectional io streams that implement CloseWrite()
type WriteHalfCloser interface {
	// CloseWrite shuts down the writing half of a bidirectional io stream (e.g., "socket").
	// Corresponds to net.TCPConn.CloseWrite(). This method is called by the writer to
	// indicate end-of-stream; no further writes are possible after this call. However, the
	// read half of the bidirectional stream remains active. It allows for protocols
	// like HTTP 1.0 in which a client sends a request, closes the write side of the socket,
	// then reads the response, and a server reads a request until end-of-stream before
	// sending a response.
	CloseWrite() error
}

// ReadWriteHalfCloser is an interface for bidirectional io streams that implement
// CloseRead() and CloseWrite()
type ReadWriteHalfCloser interface {
	ReadHalfCloser
	WriteHalfCloser
}
