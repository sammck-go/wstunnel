package wstchannel

import (
	"io"
)

// ChannelConn is a virtual open bidirectional stream "socket", either
//      1) created by a ChannelEndpoint to wrap communication with a local network resource
//      2) created by the proxy session to wrap a single ssh.Conn communication channel with a remote endpoint
type ChannelConn interface {
	io.ReadWriteCloser
	WriteHalfCloser
	AsyncShutdowner

	// GetConnID returns a unique identifier of this connection. Identifiers are never reused for the life of the process.
	GetConnID() uint64

	// GetNumBytesRead returns the number of bytes read so far on a ChannelConn
	GetNumBytesRead() uint64

	// GetNumBytesWritten returns the number of bytes written so far on a ChannelConn
	GetNumBytesWritten() uint64
}
