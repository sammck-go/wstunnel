package wstnet

import (
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/sammck-go/asyncobj"
	"github.com/sammck-go/logger"
	"golang.org/x/crypto/ssh"
)

// mergedBipipe Implements a Bipipe interface around independent Writer and Reader interfaces
// (e.g., stin and stdout)
type mergedBipipe struct {
	// implements Bipipe
	io.Reader
	io.Writer
	*asyncobj.Helper

	// name is a short name for the bipipe, for logging, etc.
	name string

	// closeReaderOnShutdown will cause the reader to be closed on shutdown if it implements Closer
	closeReaderOnShutdown bool

	// closeWriterOnShutdown will cause the writer to be closed on shutdown if it implements Closer,
	// and on CloseWrite() if it implements WriteHalfCloser.
	closeWriterOnShutdown bool

	// writerIsClosing is set to true if the writer is closed or being closed before shutdown due to WriteClose
	writerIsClosing bool

	// writerIsClosed is set to true when writerCloseErr is available
	writerIsClosed bool

	// writerIsReallyClosed is set to true when writer.Close() has been called early
	writerIsClosed bool

	// writerCloseErr holds the error returned from closing the writer, if it is closed early.
	writerCloseErr error

	// writerCloseChan is a chan that is closed when writerCloseError becomes available
	writerCloseChan chan struct{}
}

// NewMergedBipipe Implements a Bipipe interface around independent Writer and Reader interfaces
// (e.g., stdin and stdout).
// If reader is nil, NewNilEOFReader() is used to create a reader that always returns EOF.
// If writer is nil, NewNilWriter() is used to create a writer that discards everything
// If closeReaderOnShutdown is true, the new bipipe becomes the owner of reader and is responsible for closing it.
// If closeWriterOnShutdown is true, the new bipipe becomes the owner of writer and is responsible for closing it.
func NewMergedBipipe(
	logger logger.Logger,
	name string,
	reader io.Reader,
	writer io.Writer,
	closeReaderOnShutdown bool,
	closeWriterOnShutdown bool,
) Bipipe {
	if reader == nil {
		reader = NewNilEOFReader()
		closeReaderOnShutdown = true
	}
	if writer == nil {
		writer = NewNilWriter()
		closeWriterOnShutdown = true
	}
	bp := &mergedBipipe{
		Reader: reader,
		Writer: writer,
		name: fmt.Sprintf("<Bipipe %s>", name),
		closeReaderOnShutdown: closeReaderOnShutdown,
		closeWriterOnShutdown: closeWriterOnShutdown,
		writerIsClosing: false,
		writerIsClosed: false,
		writerIsReallyClosed: false,
		writerCloseErr: nil,
		writerCloseChan: make(chan struct{})
	}
	bp.Helper = asyncobj.NewHelper(logger.ForkLogStr(bp.name), bp)

	bp.SetIsActivated()

	return bp
}

func (bp *mergedBipipe) String() string {
	return bp.name
}

// CloseWrite closes the write side of the Bipipe, if closeWriterOnShutdown is true
func (bp *mergedBipipe) CloseWrite() error {
	err := bp.DeferShutdown()
	if err == nil {
		bp.Lock.Lock()
		if bp.writerIsClosing {
			bp.Lock.Unlock()
			<-bp.writerCloseChan
			err = bp.writerCloseErr
		} else {
			bp.writerIsClosing = true
			bp.Lock.Unlock()
			if bp.closeWriterOnShutdown {
				if rhc, ok := bp.Writer.(WriteHalfCloser); ok {
					err = rhc.CloseWrite()
				} else if c, ok := bp.Writer.(Closer); ok {
					err = c.Close()
					if err == nil {
						bp.Lock.Lock()
						bp.writerIsReallyClosed = true
						bp.Lock.Unlock()
					}
				}
			}
			bp.Lock.Lock()
			bp.writerCloseErr = err
			bp.writerIsClosed = true
			close(bp.writerCloseChan)
			bp.Lock.Unlock()
		}
		bp.UndeferShutdown()
	}
	return err
}

// HandleOnceShutdown will be called exactly once, in StateShuttingDown, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
// This method will never be called while shutdown is deferred.
func (bp *mergedBipipe) HandleOnceShutdown(completionError error) error {
	var err error = nil

	if bp.closeReaderOnShutdown {
		if c, ok := bp.Reader.(Closer); ok {
			err = c.Close()
			if completionError == nil {
				completionError = err
			}
		}
	}

	if bp.closeWriterOnShutdown {
		bp.Lock.Lock()
		if bp.writerIsClosing && !bp.writerIsClosed {
			bp.Panic("Shutdown occurred while WriteClose was in progress; should not be possible")
		}
		if bp.writerIsReallyClosed && bp.writerCloseErr != nil && completionError == nil {
			completionError = bp.writerCloseErr
		}
		needCloseWriter = !bp.writerIsClosed
		needRealCloseWriter = !bp.writerIsReallyClosed
		bp.writerIsClosing = true
		bp.Lock.Unlock()

		if needCloseWriter {
			if needRealCloseWriter {
				if c, ok := bp.Writer.(Closer); ok {
					err = c.Close()
					if completionError == nil {
						completionError = err
					}
				}
			}
			bp.Lock.Lock()
			bp.writerCloseErr = completionErr
			bp.writerIsClosed = true
			if needRealCloseWriter {
				bp.writerIsReallyClosed = true
			}
			close(bp.writerCloseChan)
			bp.Lock.Unlock()
		}
	}

	return completionError
}
