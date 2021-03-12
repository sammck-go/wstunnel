package wstnet

import (
	"fmt"
	"io"
	"math/rand"
	"os"
	"testing"

	"github.com/sammck-go/asyncobj"
	"github.com/sammck-go/logger"
)

type testBipipe struct {
	*asyncobj.Helper
	t                     *testing.T
	id                    int
	name                  string
	readableData          []byte
	remainingReadableData []byte
	writtenData           []byte
	writeClosed           bool
}

func NewTestBipipe(t *testing.T, logger logger.Logger, id int) *testBipipe {
	nbReadable := rand.Intn(128*1024) + 16*1024
	readableData := make([]byte, nbReadable)
	rand.Read(readableData)
	name := fmt.Sprintf("<TestBipipe %d>", id)
	sublogger := logger.ForkLog(name)
	bp := &testBipipe{
		t:                     t,
		id:                    id,
		name:                  name,
		readableData:          readableData,
		remainingReadableData: readableData,
		writtenData:           make([]byte, 0, 16*1024),
		writeClosed:           false,
	}
	bp.Helper = asyncobj.NewHelper(sublogger, bp)

	bp.DLog("Activating")
	bp.SetIsActivated()
	bp.DLog("Activated")

	return bp
}

func (bp *testBipipe) String() string {
	return bp.name
}

func (bp *testBipipe) HandleOnceShutdown(completionErr error) error {
	return completionErr
}

func (bp *testBipipe) Read(p []byte) (n int, err error) {
	err = bp.DeferShutdown()
	nr := 0
	if err == nil {
		nb := len(p)
		bp.Lock.Lock()
		if nb > 0 && len(bp.remainingReadableData) == 0 {
			err = io.EOF
		} else {
			if nb > len(bp.remainingReadableData) {
				nb = len(bp.remainingReadableData)
			}
			copy(p, bp.remainingReadableData[:nb])
			bp.remainingReadableData = bp.remainingReadableData[nb:]
			nr = nb
		}
		bp.Lock.Unlock()
	}
	bp.UndeferShutdown()
	return nr, err
}

func (bp *testBipipe) Write(p []byte) (n int, err error) {
	err = bp.DeferShutdown()
	nw := 0
	if err == nil {
		bp.Lock.Lock()
		if bp.writeClosed {
			err = bp.WLogErrorf("Write side of Bipipe has already been closed")
			bp.t.Error(err)
		} else {
			bp.writtenData = append(bp.writtenData, p...)
			nw = len(p)
		}
		bp.Lock.Unlock()
	}
	bp.UndeferShutdown()
	return nw, err
}

func (bp *testBipipe) CloseWrite() error {
	err := bp.DeferShutdown()
	if err == nil {
		bp.Lock.Lock()
		bp.writeClosed = true
		bp.Lock.Unlock()
	}
	bp.UndeferShutdown()
	return err
}

func TestBipipeBridge(t *testing.T) {
	var err error

	/*
		d := t.TempDir()
		lfname := filepath.Join(d, "TestBipipeBridge.log")
		lf, err := os.Create(lfname)
		if err != nil {
			t.Fatalf("os.Create(\"%s\") returned error: %s", lfname, err)
		}
		defer func() {
			if lf != nil {
				lf.Close()
			}
		}()
	*/

	lf := os.Stderr

	lg, err := logger.New(
		logger.WithWriter(lf),
		logger.WithLogLevel(logger.LogLevelDebug),
		logger.WithPrefix("TestBipipeBridge"),
	)

	if err != nil {
		t.Fatalf("logger.New() returned error: %s", err)
	}

	bp0 := NewTestBipipe(t, lg, 0)
	bp1 := NewTestBipipe(t, lg, 1)
	bps := []*testBipipe{bp0, bp1}

	// start bridging between our fake Bipipes
	bb := NewBipipeBridger(lg, bp0, bp1, 32*1024, true)

	// give the bridge 4 seconds to finish
	// ctx := context.Background()
	// ctx, cancel := context.WithTimeout(ctx, time.Second*4)
	// bb.ShutdownOnContext(ctx)

	// Wait for everything to wrap up
	err = bb.WaitShutdown()

	if err != nil {
		t.Errorf("Bipipe bridge failed: %v", err)
	}
	for _, bp := range bps {
		if !bp.IsDoneShutdown() {
			t.Errorf("%v was not shut down by bridge", bp)
		}
		err = bp.WaitShutdown()
		if err != nil {
			t.Errorf("%v had final completion error: %v", bp, err)
		}
	}
	for _, bp := range bps {
		otherbp := bps[1-bp.id]
		nbUnread := uint64(len(bp.remainingReadableData))
		if nbUnread != 0 {
			t.Errorf("%v had %v unread bytes of data out of %v", bp, nbUnread, len(bp.readableData))
		}
		if !bp.writeClosed {
			t.Errorf("Write side of %v was never explicitly closed", bp)
		}
		anbw := uint64(len(bp.writtenData))
		expectedNbw := uint64(len(otherbp.readableData))
		nbw := bb.GetNumBytesWritten(bp.id)

		if anbw != nbw {
			t.Errorf("%v actual written byte count (%v) does not match GetNumBytesWritten(%d) (%v)", bp, anbw, bp.id, nbw)
		}
		if anbw != expectedNbw {
			t.Errorf("%v only received %v bytes out of %v available from %v", bp, anbw, expectedNbw, otherbp)
		} else {
			for i, b := range bp.writtenData {
				expected := otherbp.readableData[i]
				if b != expected {
					t.Errorf("Bipipe %d had incorrect byte %v written at offset %d; expected %v", bp.id, b, i, expected)
					break
				}
			}
		}
	}
}
