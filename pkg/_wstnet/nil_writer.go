package wstnet

import (
	"io"
)

// nilWriter implements Stringer, WriteCloser and WriteHalfCloser, and ReaderFrom, and simply discards all data written.
type nilWriter struct{}

// sharedNilWriter is a precreated object that implements Stringer, WriteCloser and WriteHalfCloser, and ReaderFrom, and
// simply discards all data written.
var sharedNilWriter WriteCloseHalfCloser = &nilWriter{}

// NewNilWriter returns a shared NilWriter object, which implements Stringer, WriteCloser,  WriteHalfCloser,
// and ReaderFrom, and simply discards all data written.
func NewNilWriter() WriteCloseHalfCloser {
	return sharedNilWriter
}

func (wc *nilWriter) String() string {
	return "<nilWriter>"
}

// Close shuts down the bipipe and waits for shutdown to complete
func (wc *nilWriter) Close() error {
	return nil
}

// CloseWrite closes the write side of the Bipipe, causing the remote reader to receive EOF. Does not affect the
// read side of the Bipipe. If the underlying net.Conn supports CloseWrite() (e.g., TCPConn or UnixConn), then it is
// called.  Otherwise, this method does nothing.
func (wc *nilWriter) CloseWrite() error {
	return nil
}

// CloseWrite closes the write side of the Bipipe, causing the remote reader to receive EOF. Does not affect the
// read side of the Bipipe. If the underlying net.Conn supports CloseWrite() (e.g., TCPConn or UnixConn), then it is
// called.  Otherwise, this method does nothing.
func (wc *nilWriter) Write(p []byte) (n int, err error) {
	return len(p), nil
}

// ReadFrom implements io.ReaderFrom which is more efficient than Write() when copying between a Reader
// and a Writer, in that it avoids buffer allocation. It is used by io.Copy if available.
// This implementation simply reads from the provided Reader until an error or EOF occurs, discarding
// all received data.
func (wc *nilWriter) ReadFrom(r io.Reader) (n int64, err error) {
	if wt, ok := r.(io.WriterTo); ok {
		// If the reader supports WriterTo, just call it, so no buffer at all is needed in the case where
		// both the reader and writer are nil stubs. There is a potential for an infinite loop here if the
		// reader plays a simmilar buck-passing trick, but we are the only ones doing it.
		n, err = wt.WriteTo(wc)
	} else {
		n = 0
		buffer := make([]byte, 32*1024)
		for {
			var np int
			np, err = r.Read(buffer)
			n += int64(np)
			if err != nil {
				if err == io.EOF {
					err = nil
				}
				break
			}
		}
	}
	return n, err
}
