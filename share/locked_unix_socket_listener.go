package chshare

import (
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"
)

// LockedUnixSocketListener is a wrapper around a unix domain socket listener
// That holds a flock-style file lock on a parallel ".lock" file that can be
// used to prevent collisions of unix-domain socket listeners but still allow
// orphaned socket files to be deleted.
type LockedUnixSocketListener struct {
	Logger
	lock         sync.Mutex
	path         string
	lockPath     string
	lockFd       *os.File
	unixListener net.Listener
	closed       bool
	closeErr     error
	done         chan struct{}
}

// Close implements net.Listener Close method, releasing the socket lockfile
// after closing the listen socket
func (l *LockedUnixSocketListener) Close() error {
	l.lock.Lock()
	closed := l.closed
	l.closed = true
	l.lock.Unlock()

	if closed {
		<-l.done
	} else {
		var ucloseErr error
		var unlockErr error
		if l.unixListener != nil {
			os.Remove(l.path)
			l.DLogf("Closing actual unix listensocket")
			ucloseErr = l.unixListener.Close()
			l.DLogf("Actual unix listen socket")
		}
		if l.lockFd != nil {
			// Remove the lockfile before we release the lock. This will allow someone else
			// to immediately recreate the lockfile and claim a lock on it, which is fine.
			l.DLogf("unlocking/removing unix domain socket lockfile")
			os.Remove(l.lockPath)
			// ignore error from remove
			err := syscall.Flock(int(l.lockFd.Fd()), syscall.LOCK_UN)
			if err != nil {
				l.lockFd.Close()
				unlockErr = l.DLogErrorf("Unlock of lockfile \"%s\" failed: %s)", l.lockPath, err)
			} else {
				err = l.lockFd.Close()
				if err != nil {
					unlockErr = l.DLogErrorf("Close of lockfile \"%s\" failed: %s)", l.lockPath, err)
				}
			}
			l.DLogf("DONE unlocking/removing unix domain socket lockfile")
		}
		l.closeErr = ucloseErr
		if l.closeErr == nil {
			l.closeErr = unlockErr
		}

		close(l.done)
	}

	return l.closeErr
}

// NewLockedUnixSocketListener Implements a listener for a unix domain socket that creates and
// locks a ".lock" lockfile next to the unix domain socket path, to prevent multiple listeners
// on the same pathname but still allow orphaned domain sockets to be deleted. Requires
// other players to follow the same rules.
func NewLockedUnixSocketListener(logger Logger, path string) (*LockedUnixSocketListener, error) {
	l := &LockedUnixSocketListener{
		Logger: logger.Fork("LockedUnixSocketListener(\"%s\")", path),
	}
	l.done = make(chan struct{})
	if path == "" {
		return nil, l.Errorf("Empty unix domain docket path")
	}
	abspath, err := filepath.Abs(path)
	if err != nil {
		return nil, l.Errorf("Invalid unix domain socket pathname \"%s\": %s", path, err)
	}
	l.path = abspath
	lockPath := abspath + ".lock"
	l.lockPath = lockPath

	info, err := os.Stat(abspath)
	if err != nil && !os.IsNotExist(err) {
		return nil, l.Errorf("Could not stat unix domain socket pathname \"%s\": %s", abspath, err)
	}

	if info != nil && (info.Mode()&os.ModeSocket) == 0 {
		return nil, l.Errorf("Path \"%s\" exists and is not a unix domain socket", abspath)
	}

	lockFd, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		return nil, l.Errorf("Unable to open unix domain socket lockfile \"%s\": %s", lockPath, err)
	}

	err = syscall.Flock(int(lockFd.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		lockFd.Close()
		return nil, l.Errorf("Unix domain socket in use (lockfile \"%s\" is locked): %s", lockPath, err)
	}

	l.lockFd = lockFd

	if info != nil {
		err = os.Remove(abspath)
		if err != nil {
			l.Close()
			return nil, l.Errorf("Unable to remove orphaned unix domain socket \"%s\"", abspath)
		}
	}

	unixListener, err := net.Listen("unix", abspath)
	if err != nil {
		l.Close()
		return nil, l.Errorf("Unix domain socket listen failed for path '%s': %s", path, err)
	}

	l.DLogf("Listening on unix domain socket path \"%s\"", abspath)

	l.unixListener = unixListener

	return l, nil
}

func (l *LockedUnixSocketListener) String() string {
	return l.Logger.Prefix()
}

// Accept implements net.Listener Accept method, delegating to Unix listen socket
func (l *LockedUnixSocketListener) Accept() (net.Conn, error) {
	return l.unixListener.Accept()
}

// Addr implements net.Listener Addr method, delegating to Unix listen socket
func (l *LockedUnixSocketListener) Addr() net.Addr {
	return l.unixListener.Addr()
}
