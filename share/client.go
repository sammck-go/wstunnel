package chshare

import (
	"context"
	"encoding/json"
	"fmt"
	socks5 "github.com/armon/go-socks5"
	"github.com/gorilla/websocket"
	"github.com/jpillora/backoff"
	"golang.org/x/crypto/ssh"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

//Config represents a client configuration
type Config struct {
	shared           *SessionConfigRequest
	Debug            bool
	Fingerprint      string
	Auth             string
	KeepAlive        time.Duration
	MaxRetryCount    int
	MaxRetryInterval time.Duration
	Server           string
	HTTPProxy        string
	ChdStrings       []string
	HostHeader       string
}

//Client represents a client instance
type Client struct {
	ShutdownHelper
	config       *Config
	sshConfig    *ssh.ClientConfig
	sshConn      ssh.Conn
	sshConnReady chan struct{}
	sshConnErr   error
	httpProxyURL *url.URL
	server       string
	running      bool
	runningc     chan error
	connStats    ConnStats
	socksServer  *socks5.Server
	loopServer   *LoopServer
}

//NewClient creates a new client instance
func NewClient(config *Config) (*Client, error) {
	//apply default scheme
	logLevel := LogLevelInfo
	if config.Debug {
		logLevel = LogLevelDebug
	}

	logger := NewLogger("client", logLevel)

	if !strings.HasPrefix(config.Server, "http") {
		config.Server = "http://" + config.Server
	}
	if config.MaxRetryInterval < time.Second {
		config.MaxRetryInterval = 5 * time.Minute
	}
	u, err := url.Parse(config.Server)
	if err != nil {
		return nil, err
	}
	//apply default port
	if !regexp.MustCompile(`:\d+$`).MatchString(u.Host) {
		if u.Scheme == "https" || u.Scheme == "wss" {
			u.Host = u.Host + ":443"
		} else {
			u.Host = u.Host + ":80"
		}
	}
	//swap to websockets scheme
	u.Scheme = strings.Replace(u.Scheme, "http", "ws", 1)
	shared := &SessionConfigRequest{}
	for _, s := range config.ChdStrings {
		chd, err := ParseChannelDescriptor(s)
		if err != nil {
			return nil, fmt.Errorf("%s: Failed to parse channel descriptor string '%s': %s", logger.Prefix(), s, err)
		}
		shared.ChannelDescriptors = append(shared.ChannelDescriptors, chd)
	}
	config.shared = shared
	loopServer, err := NewLoopServer(logger)
	if err != nil {
		return nil, fmt.Errorf("%s: Failed to start loop server", logger.Prefix())
	}
	client := &Client{
		config:       config,
		sshConnReady: make(chan struct{}),
		server:       u.String(),
		//running:      true,
		//runningc:     make(chan error, 1),
		loopServer: loopServer,
	}
	client.InitShutdownHelper(logger, client)
	client.PanicOnError(client.PauseShutdown())
	defer client.ResumeShutdown()

	if p := config.HTTPProxy; p != "" {
		client.httpProxyURL, err = url.Parse(p)
		if err != nil {
			return nil, fmt.Errorf("%s: Invalid proxy URL (%s)", logger.Prefix(), err)
		}
	}

	user, pass := ParseAuth(config.Auth)

	client.sshConfig = &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		ClientVersion:   "SSH-" + ProtocolVersion + "-client",
		HostKeyCallback: client.verifyServer,
		Timeout:         30 * time.Second,
	}

	return client, nil
}

// Implement LocalChannelEnv interface

// IsServer returns true if this is a proxy server; false if it is a cliet
func (c *Client) IsServer() bool {
	return false
}

// GetSSHConn waits for and returns the main ssh.Conn that this proxy is using to
// communicate with the remote proxy. It is possible that goroutines servicing
// local stub sockets will ask for this before it is available (if for example
// a listener on the client accepts a connection before the server has ackknowledged
// configuration.
func (c *Client) GetSSHConn() (ssh.Conn, error) {
	<-c.sshConnReady
	return c.sshConn, c.sshConnErr
}

// GetLoopServer returns the shared LoopServer if loop protocol is enabled; nil otherwise
func (c *Client) GetLoopServer() *LoopServer {
	return c.loopServer
}

// GetSocksServer returns the shared socks5 server if socks protocol is enabled;
// nil otherwise
func (c *Client) GetSocksServer() *socks5.Server {
	return c.socksServer
}

//Run starts client and blocks while connected
func (c *Client) Run(ctx context.Context) error {
	subCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	err := c.DoOnceActivate(
		func() error {
			return c.Start(subCtx)
		},
		true,
	)
	if err != nil {
		return err
	}
	c.ShutdownOnContext(ctx)
	return c.WaitShutdown()
}

func (c *Client) verifyServer(hostname string, remote net.Addr, key ssh.PublicKey) error {
	expect := c.config.Fingerprint
	got := FingerprintKey(key)
	if expect != "" && !strings.HasPrefix(got, expect) {
		return fmt.Errorf("Invalid fingerprint (%s)", got)
	}
	//overwrite with complete fingerprint
	c.ILogf("Fingerprint %s", got)
	return nil
}

//Start client and does not block
func (c *Client) Start(ctx context.Context) error {
	c.ShutdownOnContext(ctx)
	via := ""
	if c.httpProxyURL != nil {
		via = " via " + c.httpProxyURL.String()
	}
	//prepare non-reverse proxies (other than stdio proxy, which we defer til we have a good connection)
	for i, chd := range c.config.shared.ChannelDescriptors {
		if !chd.Reverse && chd.Stub.Type != ChannelEndpointTypeStdio {
			proxy := NewTCPProxy(c.Logger, c, i, chd)
			c.AddShutdownChild(proxy)
			if err := proxy.Start(ctx); err != nil {
				return err
			}
		}
	}
	c.ILogf("Connecting to %s%s\n", c.server, via)
	//optional keepalive loop
	if c.config.KeepAlive > 0 {
		go c.keepAliveLoop()
	}
	//connection loop
	go c.connectionLoop(ctx)
	return nil
}

func (c *Client) keepAliveLoop() {
	pingDelay := time.NewTimer(c.config.KeepAlive)
	defer pingDelay.Stop()
	for {
		select {
		case <-c.ShutdownStartedChan():
			return
		case <-pingDelay.C:
			if c.sshConn != nil {
				c.sshConn.SendRequest("ping", true, nil)
			}
			pingDelay.Reset(c.config.KeepAlive)
		}
	}
}

func (c *Client) connectionLoop(ctx context.Context) {
	//connection loop!
	var connerr error
	// stdioStarted := false
	b := &backoff.Backoff{Max: c.config.MaxRetryInterval}
	for !c.IsStartedShutdown() {
		if connerr != nil {
			attempt := int(b.Attempt())
			maxAttempt := c.config.MaxRetryCount
			d := b.Duration()
			//show error and attempt counts
			msg := fmt.Sprintf("Connection error: %s", connerr)
			if attempt > 0 {
				msg += fmt.Sprintf(" (Attempt: %d", attempt)
				if maxAttempt > 0 {
					msg += fmt.Sprintf("/%d", maxAttempt)
				}
				msg += ")"
			}
			c.DLogf(msg)
			//give up?
			if maxAttempt >= 0 && attempt >= maxAttempt {
				break
			}
			c.ILogf("Retrying in %s...", d)
			connerr = nil
			SleepSignal(d)
		}
		d := websocket.Dialer{
			ReadBufferSize:   1024,
			WriteBufferSize:  1024,
			HandshakeTimeout: 45 * time.Second,
			Subprotocols:     []string{ProtocolVersion},
		}
		//optionally CONNECT proxy
		if c.httpProxyURL != nil {
			d.Proxy = func(*http.Request) (*url.URL, error) {
				return c.httpProxyURL, nil
			}
		}
		wsHeaders := http.Header{}
		if c.config.HostHeader != "" {
			wsHeaders = http.Header{
				"Host": {c.config.HostHeader},
			}
		}
		wsConn, _, err := d.Dial(c.server, wsHeaders)
		if err != nil {
			connerr = err
			continue
		}
		conn := NewWebSocketConn(wsConn)
		// perform SSH handshake on net.Conn
		c.DLogf("Handshaking...")
		sshConn, chans, reqs, err := ssh.NewClientConn(conn, "", c.sshConfig)
		if err != nil {
			c.sshConnErr = err
			if strings.Contains(err.Error(), "unable to authenticate") {
				c.ILogf("Authentication failed")
				c.DLogf(err.Error())
			} else {
				c.ILogf(err.Error())
			}
			break
		}
		c.config.shared.Version = BuildVersion
		conf, _ := c.config.shared.Marshal()
		c.DLogf("Sending session config request")
		t0 := time.Now()
		_, configerr, err := sshConn.SendRequest("config", true, conf)
		if err != nil {
			c.sshConnErr = err
			c.ILogf("Session config verification failed")
			break
		}
		if len(configerr) > 0 {
			c.ILogf(string(configerr))
			c.sshConnErr = fmt.Errorf("SSH server returned binary config error: %v", configerr)
			break
		}
		c.ILogf("Connected (Latency %s)", time.Since(t0))
		//connected
		b.Reset()
		go ssh.DiscardRequests(reqs)
		c.sshConn = sshConn

		// wake up anyone waiting for our ssh connection to be ready
		close(c.sshConnReady)

		go c.connectStreams(ctx, chans)
		err = sshConn.Wait()

		//disconnected

		// sammck: it is *not* ok to reset c.sshConn to nil after we have stub endpoints running
		//    The safest thing is to shut down here
		// c.sshConn = nil
		// if err != nil && err != io.EOF {
		//   connerr = err
		//   continue
		//   }
		c.ILogf("Disconnected\n")
		c.Shutdown(c.Errorf("Proxy Server disconnected"))

		break
	}
	c.Close()
}

// HandleOnceShutdown will be called exactly once, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
func (c *Client) HandleOnceShutdown(completionErr error) error {
	var err error
	if c.sshConn != nil {
		err = c.sshConn.Close()
	}
	if completionErr == nil {
		completionErr = err
	}
	return completionErr
}

func (c *Client) connectStreams(ctx context.Context, chans <-chan ssh.NewChannel) {
	for ch := range chans {
		reject := func(reason ssh.RejectionReason, err error) error {
			c.DLogf("Sending SSH NewChannel rejection (reason=%v): %s", reason, err)
			// TODO allow cancellation with ctx
			rejectErr := ch.Reject(reason, err.Error())
			if rejectErr != nil {
				c.DLogf("Unable to send SSH NewChannel reject response, ignoring: %s", rejectErr)
			}
			return err
		}

		epdJSON := ch.ExtraData()
		epd := &ChannelEndpointDescriptor{}
		err := json.Unmarshal(epdJSON, &epd)
		if err != nil {
			reject(ssh.UnknownChannelType, c.Errorf("Bad JSON ExtraData"))
			continue
		}

		// TODO: **MUST** implement access control (whitelist originally configured reverse-proxy skeletons)

		c.DLogf("Remote channel connect request, endpoint ='%s'", epd.LongString())
		if epd.Role != ChannelEndpointRoleSkeleton {
			reject(ssh.Prohibited, c.Errorf("Endpoint role must be skeleton"))
			continue
		}

		ep, err := NewLocalSkeletonChannelEndpoint(c.Logger, c, epd)
		if err != nil {
			reject(ssh.Prohibited, c.Errorf("Failed to create skeleton endpoint for SSH NewChannel: %s", err))
			continue
		}

		c.AddShutdownChild(ep)

		// TODO: The actual local connect request should succeed before we accept the remote request.
		//       Need to refactor code here
		sshChannel, reqs, err := ch.Accept()
		if err != nil {
			c.DLogf("Failed to accept remote SSH Channel: %s", err)
			continue
		}

		// will shutdown when sshChannel is closed
		go ssh.DiscardRequests(reqs)

		// wrap the ssh.Channel to look like a ChannelConn
		sshConn, err := NewSSHConn(c.Logger, sshChannel)
		if err != nil {
			c.DLogf("Failed to wrap SSH Channel: %s", err)
			sshChannel.Close()
			ep.Close()
			continue
		}

		// sshChannel is now wrapped by sshConn, and will be closed when sshConn is closed

		var extraData []byte
		numSent, numReceived, err := ep.DialAndServe(ctx, sshConn, extraData)

		// sshConn and sshChannel have now been closed

		if err != nil {
			c.DLogf("NewChannel session ended with error after %d bytes (caller->called), %d bytes (called->caller): %s", numSent, numReceived, err)
		} else {
			c.DLogf("NewChannel session ended normally after %d bytes (caller->called), %d bytes (called->caller)", numSent, numReceived)
		}
	}
}
