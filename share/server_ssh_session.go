package chshare

import (
	"context"
	socks5 "github.com/armon/go-socks5"
	"golang.org/x/crypto/ssh"
	"net"
	"time"
)

// ServerSSHSession wraps a primary SSH connection with a single client proxy
type ServerSSHSession struct {
	SSHSession

	// Server is the chisel proxy server on which this session is running
	server *Server
}

// NewServerSSHSession creates a server-side proxy session object
func NewServerSSHSession(server *Server) (*ServerSSHSession, error) {
	s := &ServerSSHSession{
		server: server,
	}
	s.InitSSHSession(server.Logger, s)
	return s, nil
}

// Implement LocalChannelEnv

// IsServer returns true if this is a proxy server; false if it is a cliet
func (s *ServerSSHSession) IsServer() bool {
	return true
}

// GetLoopServer returns the shared LoopServer if loop protocol is enabled; nil otherwise
func (s *ServerSSHSession) GetLoopServer() *LoopServer {
	return s.server.loopServer
}

// GetSocksServer returns the shared socks5 server if socks protocol is enabled;
// nil otherwise
func (s *ServerSSHSession) GetSocksServer() *socks5.Server {
	return s.server.socksServer
}

// GetSSHConn waits for and returns the main ssh.Conn that this proxy is using to
// communicate with the remote proxy. It is possible that goroutines servicing
// local stub sockets will ask for this before it is available (if for example
// a listener on the client accepts a connection before the server has ackknowledged
// configuration. An error response indicates that the SSH connection failed to initialize.
func (s *ServerSSHSession) GetSSHConn() (ssh.Conn, error) {
	return s.sshConn, nil
}

// startWithSSHConn startss a proxy session runing in the background, given
// an incoming ssh.ServerConn.
func (s *ServerSSHSession) startWithSSHConn(
	ctx context.Context,
	sshConn *ssh.ServerConn,
	newSSHChannels <-chan ssh.NewChannel,
	sshRequests <-chan *ssh.Request,
) error {
	err := s.PauseShutdown()
	if err != nil {
		return s.DLogErrorf("runWithSSHConn() failed: %s", err)
	}
	defer s.ResumeShutdown()
	s.ShutdownOnContext(ctx)

	s.sshConn = sshConn
	s.newSSHChannels = newSSHChannels
	s.sshRequests = sshRequests

	// pull the users from the session map
	var user *User
	if s.server.users.Len() > 0 {
		sid := string(sshConn.SessionID())
		user, _ = s.server.sessions.Get(sid)
		s.server.sessions.Del(sid)
	}

	//verify configuration
	s.DLogf("Receiving configuration")
	// wait for configuration request, with timeout
	cfgCtx, cfgCtxCancel := context.WithTimeout(ctx, 10*time.Second)
	r, err := s.receiveSSHRequest(cfgCtx)
	cfgCtxCancel()
	if err != nil {
		err = s.DLogErrorf("receiveSSHRequest failed: %s", err)
		s.StartShutdown(err)
		return err
	}

	s.DLogf("Received SSH Req")

	// convenience function to send an error reply and return
	// the original error. Ignores failures sending the reply
	// since we will be bailing out anyway
	failed := func(err error) error {
		s.sendSSHErrorReply(ctx, r, err)
		s.StartShutdown(err)
		return err
	}

	if r.Type != "config" {
		return failed(s.DLogErrorf("Expecting \"config\" request, got \"%s\"", r.Type))
	}

	c := &SessionConfigRequest{}
	err = c.Unmarshal(r.Payload)
	if err != nil {
		return failed(s.DLogErrorf("Invalid session config request encoding: %s", err))
	}

	//print if client and server  versions dont match
	if c.Version != BuildVersion {
		v := c.Version
		if v == "" {
			v = "<unknown>"
		}
		s.ILogf("WARNING: Chisel Client version (%s) differs from server version (%s)", v, BuildVersion)
	}

	//confirm reverse tunnels are allowed
	for _, chd := range c.ChannelDescriptors {
		if chd.Reverse && !s.server.reverseOk {
			return failed(s.DLogErrorf("Reverse port forwarding not enabled on server"))
		}
	}
	//if user is provided, ensure they have
	//access to the desired remotes
	if user != nil {
		for _, chd := range c.ChannelDescriptors {
			chdString := chd.String()
			if !user.HasAccess(chdString) {
				return failed(s.DLogErrorf("Access to \"%s\" denied", chdString))
			}
		}
	}

	//set up reverse port forwarding
	for i, chd := range c.ChannelDescriptors {
		if chd.Reverse {
			s.DLogf("Reverse-mode route[%d] %s; starting stub listener", i, chd.String())
			proxy := NewTCPProxy(s.Logger, s, i, chd)
			s.AddShutdownChild(proxy)
			if err := proxy.Start(ctx); err != nil {
				return failed(s.DLogErrorf("Unable to start stub listener %s: %s", chd.String(), err))
			}
		} else {
			s.DLogf("Forward-mode route[%d] %s; connections will be created on demand", i, chd.String())
		}
	}

	//success!
	err = s.sendSSHReply(ctx, r, true, nil)
	if err != nil {
		err = s.DLogErrorf("Failed to send SSH config success response: %s", err)
		s.StartShutdown(err)
		return err
	}

	go s.handleSSHRequests(ctx, sshRequests)
	go s.handleSSHChannels(ctx, newSSHChannels)

	s.DLogf("SSH session up and running")

	go func() {
		err := sshConn.Wait()
		s.StartShutdown(err)
	}()
	return nil
}

// runWithSSHConn runs a proxy session from a client from start to end, given
// an incoming ssh.ServerConn. On exit, the incoming ssh.ServerConn still
// needs to be closed.
func (s *ServerSSHSession) runWithSSHConn(
	ctx context.Context,
	sshConn *ssh.ServerConn,
	newSSHChannels <-chan ssh.NewChannel,
	sshRequests <-chan *ssh.Request,
) error {
	err := s.startWithSSHConn(ctx, sshConn, newSSHChannels, sshRequests)
	if err != nil {
		return err
	}
	return s.WaitShutdown()
}

// Run runs an SSH server session to completion from an incoming
// just-connected client socket (which has already been wrapped on a websocket)
func (s *ServerSSHSession) Run(ctx context.Context, conn net.Conn) error {
	err := s.PauseShutdown()
	if err != nil {
		err = s.DLogErrorf("Run() failed: %s", err)
		return err
	}

	s.DLogf("SSH Handshaking...")
	sshConn, newSSHChannels, sshRequests, err := ssh.NewServerConn(conn, s.server.sshConfig)
	if err != nil {
		return s.ResumeAndShutdown(s.DLogErrorf("Failed to handshake (%s)", err))
	}

	s.ResumeShutdown()

	err = s.runWithSSHConn(ctx, sshConn, newSSHChannels, sshRequests)
	if err != nil {
		return s.Shutdown(s.DLogErrorf("SSH session failed: %s", err))
	}

	s.DLogf("Closing SSH connection")
	return s.Close()
}
