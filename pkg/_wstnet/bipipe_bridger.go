package wstnet

import (
	"fmt"
	"io"
	"sync"

	"github.com/sammck-go/asyncobj"
	"github.com/sammck-go/logger"
)

// BipipeBridger is the interface to a background task that bridges traffic in both directions between two Bipipes. It takes ownersip
// of the two Bipipes and automatically shuts itself and both Bipipes down when streams are complete in both directions, or when an
// error occurs.
type BipipeBridger interface {
	fmt.Stringer

	// Allows asynchronous shutdown/close of both Bipipes with an advisory error to be subsequently
	// returned from current and future APIs. After shutdown is started, forwarding background workers
	// will complete quickly. On completion of Shutdown(), all resources will be freed. Shutdown() will
	// return nil if all traffic to EOS was successfully forwarded in both directions and shutdown was clean.
	asyncobj.AsyncShutdowner

	asyncobj.HandleOnceShutdowner

	// GetNumBytesWritten returns the number of bytes successfully written to one of the Bipipes being bridged.
	// edgeIndex must be 0 or 1. This method may be called after shutdown is complete to determine the final
	// transfer count. Depending on bridge optimizations, the values returned from this method may not be
	// updated (e.g., always return 0) until the entire stream in a given direction has been transferred (see NewBipipeBridger).
	GetNumBytesWritten(edgeIndex int) uint64
}

// bipipeBridgeEdge
type bipipeBridgeEdge struct {
	pipe      Bipipe
	nbWritten uint64
}

// DefaultBipipeBufferSize is the default buffer size used for forwarding data between two Bipipes.
const DefaultBipipeBufferSize = 32 * 1024

// NewBipipeBridger starts a new background bridging task that forwards traffic in both directions between two Bipipes.
// On return, the bridge is already activated.
// If one or both of the Bipipes implements either io.ToWriter or io.FromReader, then an optimization may be made by the bridge
// to use them to forward traffic in one or both directions, eliminating a buffer copy and improving performance. If such
// an optimization is not possible, then buffered forwarding will be used, with bufferSize specifying the size of buffer
// to use. If bufferSize is 0, then DefaultBipipeBufferSize will be used.
// If publishProgress is true, then buffered forwarding will always be used, even if one of the Bipipes implements
// io.ToWriter or io.FromReader. In this case, the values returned from GetNumBytesWritten() will always be updated after each
// buffered write. This allows the calling application to monitor bandwidth during the life of the bridge, at the
// possible expense of bridge performance.
func NewBipipeBridger(
	logger logger.Logger,
	pipe0 Bipipe,
	pipe1 Bipipe,
	bufferSize int,
	publishProgress bool,
) BipipeBridger {
	name := fmt.Sprintf("[Bridge %v <=> %v]", pipe0, pipe1)
	sublogger := logger.ForkLog(name)
	pipes := []Bipipe{pipe0, pipe1}
	edges := make([]*bipipeBridgeEdge, 0, 2)
	for _, pipe := range pipes {
		edge := &bipipeBridgeEdge{
			pipe:      pipe,
			nbWritten: 0,
		}
		edges = append(edges, edge)
	}
	bridge := &BipipeBridge{
		edges: edges,
	}
	bridge.Helper = asyncobj.NewHelper(sublogger, bridge)

	bridge.forwarderWg.Add(2)
	bridge.DLog("Activating")
	bridge.SetIsActivated()
	go bridge.forwardOneBridgeEdgeDirection(edges[0], edges[1], bufferSize, publishProgress)
	go bridge.forwardOneBridgeEdgeDirection(edges[1], edges[0], bufferSize, publishProgress)
	go func() {
		bridge.forwarderWg.Wait()
		bridge.DLog("Both forwarding goroutines completed; cleaning up")
		bridge.StartShutdown(nil)
	}()

	return bridge
}

// BipipeBridge is a background task that bridges traffic in both directions between two Bipipes. It automatically shuts itself down
// when streams are complete in both directions, or when an error occurs.
type BipipeBridge struct {
	*asyncobj.Helper

	// name is the friently name of the bridge, used for string()
	name string

	// edges is an array of length 2 holding the two Bipipes that are being bridged, and associated state for each
	edges []*bipipeBridgeEdge

	// forwarderWg is a wait group that becomes unblocked when
	// both directional forwarding goroutines have completed
	forwarderWg sync.WaitGroup
}

// String returns a friendly name for the bridge.
func (bb *BipipeBridge) String() string {
	return bb.name
}

// GetNumBytesWritten returns the number of bytes successfully written to one of the Bipipes being bridged.
// edgeIndex must be 0 or 1. This method may be called after shutdown is complete to determine the final
// transfer count. Depending on bridge optimizations, the values returned from this method may not be
// updated (e.g., always return 0) until the entire stream in a given direction has been transferred (see NewBipipeBridger).
func (bb *BipipeBridge) GetNumBytesWritten(edgeIndex int) uint64 {
	bb.Lock.Lock()
	defer bb.Lock.Unlock()
	return bb.edges[edgeIndex].nbWritten
}

// HandleOnceShutdown will be called exactly once vy asyncobj.Helper, in StateShuttingDown, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
// This method will never be called while shutdown is deferred.
func (bb *BipipeBridge) HandleOnceShutdown(completionErr error) error {
	finalErr := completionErr

	// Start shutting down both bridged Bipipes. This will cause all reads/writes to fail
	// ensuring the forwarding goroutines will exit soon
	for _, edge := range bb.edges {
		edge.pipe.StartShutdown(completionErr)
	}

	// wait for the two forwarding goroutines to exit
	bb.forwarderWg.Wait()

	// wait for the two Bipipes to finish shutting down.
	for _, edge := range bb.edges {
		err := edge.pipe.WaitShutdown()
		if err != nil && finalErr == nil {
			finalErr = err
		}
	}

	return finalErr
}

// forwardOneBridgeEdgeDirection is called in its own goroutine; it forwards bytes in one direction from
// one edge to another, keeping track of byte counts.  If the publishProgress is false, and
// either srcEdge implements io.WriterTo or dstEdge implements io.ReaderFrom, then io.Copy is
// used to reduce buffer copies--in this case bytes writtent to dstEdge will
// appear to be 0 until the stream is complete or an error occurs. bufferSize specifies the maximum
// number of bytes to transfer at once; if 0, the default is used (cu 32KB from source code).
// On successful completion without error, the Write half of dstEdge will have been shut down.
// On any error, the entire bridge is scheduled for shutdown.
// The asyncobj.Helper wait group is signalled when the goroutine completes.
func (bb *BipipeBridge) forwardOneBridgeEdgeDirection(
	srcEdge *bipipeBridgeEdge,
	dstEdge *bipipeBridgeEdge,
	bufferSize int,
	publishProgress bool,
) {
	src := srcEdge.pipe
	dst := dstEdge.pipe
	useCopy := !publishProgress
	if useCopy {
		if _, ok := src.(io.WriterTo); !ok {
			if _, ok := dst.(io.ReaderFrom); !ok {
				useCopy = false
			}
		}
	}
	var err error
	if useCopy {
		var nbw int64
		nbw, err = io.Copy(dst, src)
		bb.TLogf("Bipipe src %v dst %v io.Copy transferred %v bytes, err=%v", src, dst, nbw, err)
		if nbw > 0 {
			bb.Lock.Lock()
			dstEdge.nbWritten += uint64(nbw)
			bb.Lock.Unlock()
		}
	} else {
		if bufferSize == 0 {
			bufferSize = DefaultBipipeBufferSize
		}
		buffer := make([]byte, bufferSize)
		for {
			nbr, rerr := src.Read(buffer)
			bb.TLogf("Bipipe src %v read %v bytes, err=%v", src, nbr, err)
			if nbr > len(buffer) {
				bb.Panicf("Bipipe src %v read more (%d) bytes than requested (%d)", src, nbr, len(buffer))
				nbr = len(buffer)
			} else if nbr < 0 {
				bb.Panicf("Bipipe src %v read less (%d) than zero bytes", src, nbr)
				nbr = 0
			}
			if nbr == 0 && rerr == nil {
				bb.Panicf("Bipipe src %v read 0 bytes but returned no error", src)
				rerr = io.EOF
			}
			var werr error = nil
			var nbw int = 0
			if nbr > 0 {
				nbw, werr = dst.Write(buffer[:nbr])
				bb.TLogf("Bipipe dst %v wrote %v bytes, err=%v", dst, nbw, err)
				if nbw > nbr {
					bb.Panicf("Bipipe dst %v wrote more (%d) bytes than requested (%d)", dst, nbw, nbr)
					nbw = nbr
				} else if nbw < 0 {
					bb.Panicf("Bipipe dst %v wrote less (%d) than zero bytes", dst, nbw)
					nbw = 0
				}
				if werr == nil && nbw < nbr {
					bb.Panicf("Bipipe dst %v wrote fewer (%d) bytes than requested (%d) but returned no error", dst, nbw, nbr)
					werr = io.ErrShortWrite
				}
				if nbw > 0 {
					bb.Lock.Lock()
					dstEdge.nbWritten += uint64(nbw)
					bb.Lock.Unlock()
				}
			}
			if rerr != nil && rerr != io.EOF {
				err = rerr
				break
			}
			if werr != nil {
				err = werr
				break
			}
			if rerr == io.EOF {
				break
			}
		}
	}
	if err == nil {
		bb.DLogf("Closing write side of %v after %v bytes", dst, dstEdge.nbWritten)
		err = dst.CloseWrite()
	}
	if err != nil {
		// It's ok to always call StartShutdown here, but we can make the logs clearer this way
		if bb.IsStartedShutdown() {
			bb.DLogf("Forwarder to %v failed after %v bytes, already shutting down; cleaning up: %s", dst, dstEdge.nbWritten, err)
		} else {
			bb.ILogf("Forwarder to %v failed after %v bytes; shutting down: %s", dst, dstEdge.nbWritten, err)
			bb.StartShutdown(err)
		}
	} else {
		bb.DLogf("Forwarder to %v finished successfully after %v bytes", dst, dstEdge.nbWritten)
	}
	bb.forwarderWg.Done()
}
