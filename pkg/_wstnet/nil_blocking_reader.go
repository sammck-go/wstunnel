package wstnet

import (
	"io"
	"sync"
)

// nilBlockingReader implements Stringer, ReadCloser, WriterTo, and CloseRead, and simply blocks any read request until
// Close() or CloseRead() is called, at which time io.EOF is returned.
type nilBlockingReader struct {
	lock        sync.Mutex
	isClosed    bool
	closingChan chan struct{}
}

// NewNilBlockingReader creates an object that implements Stringer, ReadCloser, WriterTo, and CloseRead, and simply blocks any read request until
// Close() or CloseRead() is called, at which time io.EOF is returned.
func NewNilBlockingReader() ReadCloseHalfCloser {
	r := &nilBlockingReader{
		isClosed:    false,
		closingChan: make(chan struct{}),
	}

	return r
}

func (rc *nilBlockingReader) String() string {
	return "<nilBlockingReader>"
}

// Close shuts down the object. All Reads will return immediately with io.EOF
func (rc *nilBlockingReader) Close() error {
	rc.lock.Lock()
	defer rc.lock.Unlock()
	if !rc.isClosed {
		rc.isClosed = true
		close(rc.closingChan)
	}
	return nil
}

// CloseRead closes the read side.
func (rc *nilBlockingReader) CloseRead() error {
	return rc.Close()
}

// Read always blocks until Close or CloseRead, then returns io.EOF
func (rc *nilBlockingReader) Read(p []byte) (n int, err error) {
	<-rc.closingChan
	return 0, io.EOF
}

// WriteTo implements io.WriterTo, which is more efficient than Read() when copying between a Reader
// and a Writer, in that it avoids buffer allocation. It is used by io.Copy if available.
// This implementation simply blocks until Close or CloseRead, then returns success with 0 bytes transferred
func (rc *nilBlockingReader) WriteTo(w io.Writer) (n int64, err error) {
	<-rc.closingChan
	return 0, nil
}
