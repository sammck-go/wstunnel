package wstnet

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"
	"syscall"

	"github.com/sammck-go/logger"
)

// LockedUnixSocketListener is a transparent wrapper around a unix domain socket net.Listener
// That holds a flock-style file lock on a parallel ".lock" file that can be
// used to prevent collisions of unix-domain socket listeners but still allow
// orphaned socket files to be deleted.
type lockedUnixSocketListener struct {
	logger.Logger
	lock         sync.Mutex
	name         string
	path         string
	lockPath     string
	lockFd       *os.File
	unixListener net.Listener
	closed       bool
	closeErr     error
	done         chan struct{}
}

// NewLockedUnixSocketListener Implements a low-level net.Listener for a unix domain socket that creates and
// locks a ".lock" lockfile next to the unix domain socket path, to prevent multiple listeners
// on the same pathname but still allow orphaned domain sockets to be deleted. Requires
// other players to follow the same rules.
// Automatically makes a best effort to delete the domain socket file when the listener is closed or
// garbage-collected. Of course, if this process terminates before Close() or gc, the zombie domain socket
// file will leak. This is relatively harmless--it will be reset the next time a listener is started with
// the same path. However, if random/temporary socket paths are used, it is a good idea to locate them
// on a tmpfs file system so any leaks will be cleaned up on system restart.
// The .lock files are never deleted (keeping is the only way to ensure atomicity/mutual exclusion of lock acquisition);
// for this reason, it is a good idea to locate the socket files in a directory on a tmpfs filesystem.
// If the path argument is relative, it is interpreted as relative to the current working directory.
func NewLockedUnixSocketListener(log logger.Logger, path string) (net.Listener, error) {
	name := fmt.Sprintf("<LockedUnixSocketListener(\"%s\")>", path)
	if log == nil {
		log = logger.NilLogger
	} else {
		log = log.ForkLogStr(name)
	}

	l := &lockedUnixSocketListener{
		Logger: log,
		name:   name,
		done:   make(chan struct{}),
	}

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

func (l *lockedUnixSocketListener) String() string {
	return l.name
}

// finalize is called in case a lockedUnixSocketListener object is gc'd without closing it. It improves the odds
// that the socket file is deleted, and that the flock is expeditiously released, in this case.
// However, it should not be depended upon; the calling app should explicitly Close() the listener before exiting.
func finalize(l *lockedUnixSocketListener) {
	// locking is not necessary
	if l.lockFd != nil {
		l.WLog("Listener garbage collected without Close; cleaning up")
		_ = l.Close()
	}
}

// Close implements net.Listener Close method, releasing the socket lockfile
// after closing the listen socket and deleting the socket file.
func (l *lockedUnixSocketListener) Close() error {
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
			// Remove the lockfile immediately before we release the lock. This will allow someone else
			// to immediately recreate the lockfile and claim a lock on it, which is fine.
			l.DLogf("unlocking/removing unix domain socket lockfile")
			_ = os.Remove(l.lockPath)
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

// Accept implements net.Listener Accept method, delegating to Unix listen socket
func (l *lockedUnixSocketListener) Accept() (net.Conn, error) {
	return l.unixListener.Accept()
}

// Addr implements net.Listener Addr method, delegating to Unix listen socket
func (l *lockedUnixSocketListener) Addr() net.Addr {
	return l.unixListener.Addr()
}
