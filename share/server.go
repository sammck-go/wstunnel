package chshare

import (
	"context"
	"errors"
	"fmt"
	socks5 "github.com/armon/go-socks5"
	"github.com/gorilla/websocket"
	"github.com/jpillora/requestlog"
	"golang.org/x/crypto/ssh"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"regexp"
)

// ProxyServerConfig is the configuration for the chisel service
type ProxyServerConfig struct {
	KeySeed  string
	AuthFile string
	Auth     string
	Proxy    string
	Socks5   bool
	NoLoop   bool
	Reverse  bool
	Debug    bool
}

// Server respresent a chisel service
type Server struct {
	ShutdownHelper
	connStats    ConnStats
	fingerprint  string
	httpServer   *HTTPServer
	reverseProxy *httputil.ReverseProxy
	sessions     *Users
	socksServer  *socks5.Server
	loopServer   *LoopServer
	sshConfig    *ssh.ServerConfig
	users        *UserIndex
	reverseOk    bool
	httpHandler  http.Handler
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin:     func(r *http.Request) bool { return true },
}

// NewServer creates and returns a new chisel server
func NewServer(config *ProxyServerConfig) (*Server, error) {
	logLevel := LogLevelInfo
	if config.Debug {
		logLevel = LogLevelDebug
	}
	logger := NewLogger("server", logLevel)
	s := &Server{
		httpServer: NewHTTPServer(logger),
		sessions:   NewUsers(),
		reverseOk:  config.Reverse,
	}
	s.InitShutdownHelper(logger, s)
	s.users = NewUserIndex(s.Logger)
	if config.AuthFile != "" {
		if err := s.users.LoadUsers(config.AuthFile); err != nil {
			return nil, err
		}
	}
	if config.Auth != "" {
		u := &User{Addrs: []*regexp.Regexp{UserAllowAll}}
		u.Name, u.Pass = ParseAuth(config.Auth)
		if u.Name != "" {
			s.users.AddUser(u)
		}
	}
	//generate private key (optionally using seed)
	key, _ := GenerateKey(config.KeySeed)
	//convert into ssh.PrivateKey
	private, err := ssh.ParsePrivateKey(key)
	if err != nil {
		log.Fatal("Failed to parse key")
	}
	//fingerprint this key
	s.fingerprint = FingerprintKey(private.PublicKey())
	//create ssh config
	s.sshConfig = &ssh.ServerConfig{
		ServerVersion:    "SSH-" + ProtocolVersion + "-server",
		PasswordCallback: s.authUser,
	}
	s.sshConfig.AddHostKey(private)
	//setup reverse proxy
	if config.Proxy != "" {
		u, err := url.Parse(config.Proxy)
		if err != nil {
			return nil, err
		}
		if u.Host == "" {
			return nil, s.Errorf("Missing protocol (%s)", u)
		}
		s.reverseProxy = httputil.NewSingleHostReverseProxy(u)
		//always use proxy host
		s.reverseProxy.Director = func(r *http.Request) {
			r.URL.Scheme = u.Scheme
			r.URL.Host = u.Host
			r.Host = u.Host
		}
	}
	//setup socks server (not listening on any port!)
	if config.Socks5 {
		socksConfig := &socks5.Config{}
		if s.GetLogLevel() >= LogLevelDebug {
			socksConfig.Logger = log.New(os.Stdout, "[socks]", log.Ldate|log.Ltime)
		} else {
			socksConfig.Logger = log.New(ioutil.Discard, "", 0)
		}
		s.socksServer, err = socks5.New(socksConfig)
		if err != nil {
			return nil, err
		}
		s.ILogf("SOCKS5 server enabled")
	}
	//setup socks server (not listening on any port!)
	if config.NoLoop {
		s.ILogf("Loop server disabled")
	} else {
		s.loopServer, err = NewLoopServer(s.Logger)
		if err != nil {
			return nil, fmt.Errorf("%s: Could not create loopback server: %s", s.Logger.Prefix(), err)
		}
	}

	//print when reverse tunnelling is enabled
	if config.Reverse {
		s.ILogf("Reverse tunnelling enabled")
	}
	return s, nil
}

// Run is responsible for starting the chisel service
func (s *Server) Run(ctx context.Context, host, port string) error {
	err := s.DoOnceActivate(
		func() error {
			s.ShutdownOnContext(ctx)

			s.ILogf("Fingerprint %s", s.fingerprint)

			if s.users.Len() > 0 {
				s.ILogf("User authentication enabled")
			}

			if s.reverseProxy != nil {
				s.ILogf("Reverse proxy enabled")
			}

			s.ILogf("Listening on %s:%s...", host, port)

			h := http.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				s.handleClientHandler(ctx, w, r)
			}))

			if s.GetLogLevel() >= LogLevelDebug {
				h = requestlog.Wrap(h)
			}

			s.httpHandler = h

			return nil
		},
		true,
	)

	if err != nil {
		return err
	}

	s.httpServer.ListenAndServe(ctx, host+":"+port, s.httpHandler)

	return s.Close()
}

// HandleOnceShutdown will be called exactly once, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
func (s *Server) HandleOnceShutdown(completionErr error) error {
	s.DLogf("HandleOnceShutdown")
	err := s.httpServer.Close()

	if completionErr == nil {
		completionErr = err
	}
	return completionErr
}

// GetFingerprint is used to access the server fingerprint
func (s *Server) GetFingerprint() string {
	return s.fingerprint
}

// authUser is responsible for validating the ssh user / password combination
func (s *Server) authUser(c ssh.ConnMetadata, password []byte) (*ssh.Permissions, error) {
	// check if user authenication is enable and it not allow all
	if s.users.Len() == 0 {
		return nil, nil
	}
	// check the user exists and has matching password
	n := c.User()
	user, found := s.users.Get(n)
	if !found || user.Pass != string(password) {
		s.DLogf("Login failed for user: %s", n)
		return nil, errors.New("Invalid authentication for username: %s")
	}
	// insert the user session map
	// @note: this should probably have a lock on it given the map isn't thread-safe??
	s.sessions.Set(string(c.SessionID()), user)
	return nil, nil
}

// AddUser adds a new user into the server user index
func (s *Server) AddUser(user, pass string, addrs ...string) error {
	authorizedAddrs := make([]*regexp.Regexp, 0)

	for _, addr := range addrs {
		authorizedAddr, err := regexp.Compile(addr)
		if err != nil {
			return err
		}

		authorizedAddrs = append(authorizedAddrs, authorizedAddr)
	}

	u := &User{Name: user, Pass: pass, Addrs: authorizedAddrs}
	s.users.AddUser(u)
	return nil
}

// DeleteUser removes a user from the server user index
func (s *Server) DeleteUser(user string) {
	s.users.Del(user)
}
