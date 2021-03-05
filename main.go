package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"

	chshare "github.com/sammck-go/wstunnel/share"
)

var help = `
  Usage: chisel [command] [--help]

  Version: ` + chshare.BuildVersion + `

  Commands:
    server - runs chisel in server mode
    client - runs chisel in client mode

  Read more:
    https://github.com/sammck-go/wstunnel

`

func sigIntHandler(ctx context.Context, cancel context.CancelFunc) {
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT)
	for {
		select {
		case <-sig:
			log.Printf("SIGINT received; cancelling main ctx")
		case <-ctx.Done():
		}
		signal.Stop(sig)
		cancel()
	}
}

func main() {
	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()
	version := flag.Bool("version", false, "")
	v := flag.Bool("v", false, "")
	flag.Bool("help", false, "")
	flag.Bool("h", false, "")
	flag.Usage = func() {}
	flag.Parse()

	if *version || *v {
		fmt.Println(chshare.BuildVersion)
		os.Exit(1)
	}

	args := flag.Args()

	subcmd := ""
	if len(args) > 0 {
		subcmd = args[0]
		args = args[1:]
	}

	switch subcmd {
	case "server":
		go sigIntHandler(ctx, ctxCancel)
		server(ctx, args)
		log.Printf("Exiting proxy server")
	case "client":
		go sigIntHandler(ctx, ctxCancel)
		client(ctx, args)
		log.Printf("Exiting proxy client")
	default:
		fmt.Fprintf(os.Stderr, help)
		os.Exit(1)
	}

}

var commonHelp = `
    --pid Generate pid file in current working directory

    -v, Enable verbose logging

    --help, This help text

  Signals:
    The chisel process is listening for:
      a SIGUSR2 to print process stats, and
      a SIGHUP to short-circuit the client reconnect timer

  Version:
    ` + chshare.BuildVersion + `

  Read more:
    https://github.com/sammck-go/wstunnel

`

func generatePidFile() {
	pid := []byte(strconv.Itoa(os.Getpid()))
	if err := ioutil.WriteFile("chisel.pid", pid, 0644); err != nil {
		log.Fatal(err)
	}
}

var serverHelp = `
  Usage: chisel server [options]

  Options:

    --host, Defines the HTTP listening host – the network interface
    (defaults the environment variable HOST and falls back to 0.0.0.0).

    --port, -p, Defines the HTTP listening port (defaults to the environment
    variable PORT and fallsback to port 8080).

    --key, An optional string to seed the generation of a ECDSA public
    and private key pair. All communications will be secured using this
    key pair. Share the subsequent fingerprint with clients to enable detection
    of man-in-the-middle attacks (defaults to the CHISEL_KEY environment
    variable, otherwise a new key is generate each run).

    --authfile, An optional path to a users.json file. This file should
    be an object with users defined like:
      {
        "<user:pass>": ["<addr-regex>","<addr-regex>"]
      }
    when <user> connects, their <pass> will be verified and then
    each of the remote addresses will be compared against the list
    of address regular expressions for a match. Addresses will
    always come in the form "<remote-host>:<remote-port>" for normal remotes
    and "R:<local-interface>:<local-port>" for reverse port forwarding
    remotes. This file will be automatically reloaded on change.

    --auth, An optional string representing a single user with full
    access, in the form of <user:pass>. This is equivalent to creating an
    authfile with {"<user:pass>": [""]}.

    --proxy, Specifies another HTTP server to proxy requests to when
    chisel receives a normal HTTP request. Useful for hiding chisel in
    plain sight.

		--noloop, Disable clients from creating or connecting to "loop"
		endpoints.

    --socks5, Allow clients to access the internal SOCKS5 proxy. See
    chisel client --help for more information.

    --reverse, Allow clients to specify reverse port forwarding remotes
    in addition to normal remotes.
` + commonHelp

func server(ctx context.Context, args []string) {

	flags := flag.NewFlagSet("server", flag.ContinueOnError)

	host := flags.String("host", "", "")
	p := flags.String("p", "", "")
	port := flags.String("port", "", "")
	key := flags.String("key", "", "")
	authfile := flags.String("authfile", "", "")
	auth := flags.String("auth", "", "")
	proxy := flags.String("proxy", "", "")
	noLoop := flags.Bool("noloop", false, "")
	socks5 := flags.Bool("socks5", false, "")
	reverse := flags.Bool("reverse", false, "")
	pid := flags.Bool("pid", false, "")
	verbose := flags.Bool("v", false, "")

	flags.Usage = func() {
		fmt.Print(serverHelp)
		os.Exit(1)
	}
	flags.Parse(args)

	if *host == "" {
		*host = os.Getenv("HOST")
	}
	if *host == "" {
		*host = "0.0.0.0"
	}
	if *port == "" {
		*port = *p
	}
	if *port == "" {
		*port = os.Getenv("PORT")
	}
	if *port == "" {
		*port = "8080"
	}
	if *key == "" {
		*key = os.Getenv("CHISEL_KEY")
	}
	s, err := chshare.NewServer(&chshare.ProxyServerConfig{
		KeySeed:  *key,
		AuthFile: *authfile,
		Auth:     *auth,
		Proxy:    *proxy,
		Socks5:   *socks5,
		NoLoop:   *noLoop,
		Reverse:  *reverse,
		Debug:    *verbose,
	})
	if err != nil {
		log.Fatal(err)
	}
	if *pid {
		generatePidFile()
	}
	go chshare.GoStats()
	if err = s.Run(ctx, *host, *port); err != nil {
		log.Printf("Proxy server exited with: %s -- closing", err)
		err = s.Close()
		log.Printf("Proxy server has closed: %s", err)
	}
}

var clientHelp = `
  Usage: chisel client [options] <server> <remote> [remote] [remote] ...

  <server> is the URL to the chisel server.

  <remote>s are remote connections tunneled through the server, each of
  which come in the form:

    <local-host>:<local-port>:<remote-host>:<remote-port>

    ■ local-host defaults to 0.0.0.0 (all interfaces).
    ■ local-port defaults to remote-port.
    ■ remote-port is required*.
    ■ remote-host defaults to 0.0.0.0 (server localhost).

  which shares <remote-host>:<remote-port> from the server to the client
  as <local-host>:<local-port>, or:

    R:<local-interface>:<local-port>:<remote-host>:<remote-port>

  which does reverse port forwarding, sharing <remote-host>:<remote-port>
  from the client to the server's <local-interface>:<local-port>.

    example remotes

      3000
      example.com:3000
      3000:google.com:80
      192.168.0.5:3000:google.com:80
      socks
      5000:socks
      R:2222:localhost:22

    When the chisel server has --socks5 enabled, remotes can
    specify "socks" in place of remote-host and remote-port.
    The default local host and port for a "socks" remote is
    127.0.0.1:1080. Connections to this remote will terminate
    at the server's internal SOCKS5 proxy.

    When the chisel server has --reverse enabled, remotes can
    be prefixed with R to denote that they are reversed. That
    is, the server will listen and accept connections, and they
    will be proxied through the client which specified the remote.

  Options:

    --fingerprint, A *strongly recommended* fingerprint string
    to perform host-key validation against the server's public key.
    You may provide just a prefix of the key or the entire string.
    Fingerprint mismatches will close the connection.

    --auth, An optional username and password (client authentication)
    in the form: "<user>:<pass>". These credentials are compared to
    the credentials inside the server's --authfile. defaults to the
    AUTH environment variable.

    --keepalive, An optional keepalive interval. Since the underlying
    transport is HTTP, in many instances we'll be traversing through
    proxies, often these proxies will close idle connections. You must
    specify a time with a unit, for example '30s' or '2m'. Defaults
    to '0s' (disabled).

    --max-retry-count, Maximum number of times to retry before exiting.
    Defaults to unlimited.

    --max-retry-interval, Maximum wait time before retrying after a
    disconnection. Defaults to 5 minutes.

    --proxy, An optional HTTP CONNECT proxy which will be used reach
    the chisel server. Authentication can be specified inside the URL.
    For example, http://admin:password@my-server.com:8081

    --hostname, Optionally set the 'Host' header (defaults to the host
    found in the server url).
` + commonHelp

func client(ctx context.Context, args []string) {

	flags := flag.NewFlagSet("client", flag.ContinueOnError)

	fingerprint := flags.String("fingerprint", "", "")
	auth := flags.String("auth", "", "")
	keepalive := flags.Duration("keepalive", 0, "")
	maxRetryCount := flags.Int("max-retry-count", -1, "")
	maxRetryInterval := flags.Duration("max-retry-interval", 0, "")
	proxy := flags.String("proxy", "", "")
	pid := flags.Bool("pid", false, "")
	hostname := flags.String("hostname", "", "")
	verbose := flags.Bool("v", false, "")
	flags.Usage = func() {
		fmt.Print(clientHelp)
		os.Exit(1)
	}
	flags.Parse(args)
	//pull out options, put back remaining args
	args = flags.Args()
	if len(args) < 2 {
		log.Fatalf("A server and least one remote is required")
	}
	if *auth == "" {
		*auth = os.Getenv("AUTH")
	}
	c, err := chshare.NewClient(&chshare.Config{
		Debug:            *verbose,
		Fingerprint:      *fingerprint,
		Auth:             *auth,
		KeepAlive:        *keepalive,
		MaxRetryCount:    *maxRetryCount,
		MaxRetryInterval: *maxRetryInterval,
		HTTPProxy:        *proxy,
		Server:           args[0],
		ChdStrings:       args[1:],
		HostHeader:       *hostname,
	})
	if err != nil {
		log.Fatal(err)
	}
	if *pid {
		generatePidFile()
	}
	go chshare.GoStats()
	if err = c.Run(ctx); err != nil {
		log.Printf("Client exited with error: %s, closing", err)
		c.Close()
	}
}
