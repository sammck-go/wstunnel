package wstchannel

import (
	socks5 "github.com/armon/go-socks5"
	"golang.org/x/crypto/ssh"
)

// LocalChannelEnv provides necessary context for initialization of local channel endpoints
type LocalChannelEnv interface {
	// IsServer returns true if this is a proxy server; false if it is a cliet
	IsServer() bool

	// GetLoopServer returns the shared LoopServer if loop protocol is enabled; nil otherwise
	GetLoopServer() *LoopServer

	// GetSocksServer returns the shared socks5 server if socks protocol is enabled;
	// nil otherwise
	GetSocksServer() *socks5.Server

	// GetSSHConn waits for and returns the main ssh.Conn that this proxy is using to
	// communicate with the remote proxy. It is possible that goroutines servicing
	// local stub sockets will ask for this before it is available (if for example
	// a listener on the client accepts a connection before the server has ackknowledged
	// configuration. An error response indicates that the SSH connection failed to initialize.
	GetSSHConn() (ssh.Conn, error)
}
