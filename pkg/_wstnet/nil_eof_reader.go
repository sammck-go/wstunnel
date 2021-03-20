package wstnet

import (
	"io"
)

// nilEOFReader implements Stringer, ReadCloser, WriterTo, and CloseRead, and simply returns io.EOF for any read request
type nilEOFReader struct{}

// sharedNilEOFReader is a precreated object that implements Stringer, ReadCloser, WriterTo, and CloseRead, and
// simply returns io.EOF for any read request
var sharedNilEOFReader ReadCloseHalfCloser = &nilEOFReader{}

// NewNilEOFReader returns a precreated NilEOFReader object, which implements Stringer,
// ReadCloser, WriterTo, and CloseRead, and simply returns io.EOF for any read request.
func NewNilEOFReader() ReadCloseHalfCloser {
	return sharedNilEOFReader
}

func (rc *nilEOFReader) String() string {
	return "<nilEOFReader>"
}

// Close shuts down the object
func (rc *nilEOFReader) Close() error {
	return nil
}

// CloseRead closes the read side.
func (rc *nilEOFReader) CloseRead() error {
	return nil
}

// Read always returns io.EOF
func (rc *nilEOFReader) Read(p []byte) (n int, err error) {
	return 0, io.EOF
}

// WriteTo implements io.WriterTo which is more efficient than Read() when copying between a Reader
// and a Writer in that it avoids buffer allocation. It is used by io.Copy if available.
// This implementation simply returns success with 0 bytes transferred
func (rc *nilEOFReader) WriteTo(w io.Writer) (n int64, err error) {
	return 0, nil
}
