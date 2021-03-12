package wstchannel

import (
	"io"

	"github.com/sammck-go/asyncobj"
)

// ChannelConn is a virtual open bidirectional stream "socket", either
//      1) created by a ChannelEndpoint to wrap communication with a local network resource
//      2) created by the proxy session to wrap a single ssh.Conn communication channel with a remote endpoint
type ChannelConn interface {
	io.ReadWriteCloser
	WriteHalfCloser
	asyncobj.AsyncShutdowner

	// WaitForClose blocks until the Close() method has been called and completed. The error returned
	// from the first Close() is returned
	WaitForClose() error

	// GetNumBytesRead returns the number of bytes read so far on a ChannelConn
	GetNumBytesRead() int64

	// GetNumBytesWritten returns the number of bytes written so far on a ChannelConn
	GetNumBytesWritten() int64
}
