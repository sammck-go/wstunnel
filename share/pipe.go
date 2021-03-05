package chshare

import (
	"io"
	"sync"
)

// Pipe concurrently copies in both directions betweeen two socket-like
// objects, returning after all data has been copied and both src
// and dst have been closed.
func Pipe(src io.ReadWriteCloser, dst io.ReadWriteCloser) (int64, int64) {
	var sent, received int64
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		received, _ = io.Copy(src, dst)
		whc, _ := dst.(WriteHalfCloser)
		if whc != nil {
			whc.CloseWrite()
		}
		wg.Done()
	}()
	go func() {
		sent, _ = io.Copy(dst, src)
		whc, _ := dst.(WriteHalfCloser)
		if whc != nil {
			whc.CloseWrite()
		}
		wg.Done()
	}()
	wg.Wait()
	src.Close()
	dst.Close()
	return sent, received
}
