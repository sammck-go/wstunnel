# wstunnel

[![GoDoc](https://godoc.org/github.com/sammck-go/wstunnel?status.svg)](https://godoc.org/github.com/sammck-go/wstunnel)

**wstunnel** is a collection of tools for building secure network communication channels between nodes that have no direct or secure network connectivity,
by relaying traffic through a shared HTTP/Websocket server. Multiple tunnelled application channels are multiplexed through a single authenticated, encrypted websocket
session between a node ad the share proxy server. **wstunnel** is useful when one or more endpoints is behind a firewall that restricts access to
outbound HTTP only, or to secure traffic between components that use insecure protocols.

**wstunnel** is both a golang package for building extensible native clients and servers, and a commandline tool that allows out-of-the box provisioning
of a shared proxy server and local proxy clients (TCP dialers and listeners).

**wstunnel** is derived from [chisel](https://github.com/jpillora.com/) and inherits from its license. However, it is substantially different and as
such is not a proper fork, and does not track changes in the **chisel** project.

![overview](./docs/network_architecture.svg)

### Features

- Easy to use
- [Encrypted connections](#security) using the SSH protocol (via `crypto/ssh`)
- [Authenticated/authorized connections](#authentication); Client connections using SSH protocol. Extensible authorization, including
  builtins for simple whitelisting.
- Client auto-reconnects with [exponential backoff](https://github.com/sammck-go/backoff)
- Client can multiplex tunnel endpoints over one authenticated Websocket
- Support for reverse port forwarding (socket listeners)
- Client can optionally pass through HTTP CONNECT proxies
- Server optionally doubles as a [reverse proxy](http://golang.org/pkg/net/http/httputil/#NewSingleHostReverseProxy)
- Server optionally allows [SOCKS5](https://en.wikipedia.org/wiki/SOCKS) connections (See [guide below](#socks5-guide))

### Install

**Go**
```sh
$ go install github.com/sammck-go/wstunnel
```

<!--
**Binaries**
[![Releases](https://img.shields.io/github/release/sammck-go/wstunnel.svg)](https://github.com/sammck-go/wstunnel/releases) [![Releases](https://img.shields.io/github/downloads/sammck-go/wstunnel/total.svg)](https://github.com/sammck-go/wstunnel/releases)

See [the latest release](https://github.com/sammck-go/wstunnel/releases/latest) or download and install it now with `curl https://i.sammck-go.com/wstunnel! | bash`

**Docker**

[![Docker Pulls](https://img.shields.io/docker/pulls/sammck-go/wstunnel.svg)](https://hub.docker.com/r/sammck-go/wstunnel/) [![Image Size](https://images.microbadger.com/badges/image/sammck-go/wstunnel.svg)](https://microbadger.com/images/sammck-go/wstunnel)

```sh
docker run --rm -it sammck-go/wstunnel --help
```
-->


**Source**

```sh
$ go get -v github.com/sammck-go/wstunnel
```

<!--
### Demo

A [demo app](https://wstunnel-demo.herokuapp.com) on Heroku is running this `wstunnel server`:

```sh
$ wstunnel server --port $PORT --proxy http://example.com
# listens on $PORT, proxy web requests to http://example.com
```

This demo app is also running a [simple file server](https://www.npmjs.com/package/serve) on `:3000`, which is normally inaccessible due to Heroku's firewall. However, if we tunnel in with:

```sh
$ wstunnel client https://wstunnel-demo.herokuapp.com 3000
# connects to wstunnel server at https://wstunnel-demo.herokuapp.com,
# tunnels your localhost:3000 to the server's localhost:3000
```

and then visit [localhost:3000](http://localhost:3000/), we should see a directory listing. Also, if we visit the [demo app](https://wstunnel-demo.herokuapp.com) in the browser we should hit the server's default proxy and see a copy of [example.com](http://example.com).
-->

### Usage

<!-- render these help texts by hand,
  or use https://github.com/jpillora/md-tmpl
    with $ md-tmpl -w README.md -->

<!--tmpl,code=plain:echo "$ wstunnel --help" && ( go build ./cmd/wstunnel && ./wstunnel --help ) -->
``` plain 
$ wstunnel --help
stat /home/sam/go/src/github.com/sammck-go/wstunnel/cmd/wstunnel: directory not found
```
<!--/tmpl-->

<!--tmpl,code=plain:echo "$ wstunnel server --help" && ./wstunnel server --help -->
``` plain 
$ wstunnel server --help

  Usage: wstunnel server [options]

  Options:

    --host, Defines the HTTP listening host – the network interface
    (defaults the environment variable HOST and falls back to 0.0.0.0).

    --port, -p, Defines the HTTP listening port (defaults to the environment
    variable PORT and fallsback to port 8080).

    --key, An optional string to seed the generation of a ECDSA public
    and private key pair. All communications will be secured using this
    key pair. Share the subsequent fingerprint with clients to enable detection
    of man-in-the-middle attacks (defaults to the WSTUNNEL_KEY environment
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
    wstunnel receives a normal HTTP request. Useful for hiding wstunnel in
    plain sight.

		--noloop, Disable clients from creating or connecting to "loop"
		endpoints.

    --socks5, Allow clients to access the internal SOCKS5 proxy. See
    wstunnel client --help for more information.

    --reverse, Allow clients to specify reverse port forwarding remotes
    in addition to normal remotes.

    --pid Generate pid file in current working directory

    -v, Enable verbose logging

    --help, This help text

  Signals:
    The wstunnel process is listening for:
      a SIGUSR2 to print process stats, and
      a SIGHUP to short-circuit the client reconnect timer

  Version:
    1.0.0-src

  Read more:
    https://github.com/sammck-go/wstunnel

```
<!--/tmpl-->

<!--tmpl,code=plain:echo "$ wstunnel client --help" && ./wstunnel client --help -->
``` plain 
$ wstunnel client --help

  Usage: wstunnel client [options] <server> <remote> [remote] [remote] ...

  <server> is the URL to the wstunnel server.

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

    When the wstunnel server has --socks5 enabled, remotes can
    specify "socks" in place of remote-host and remote-port.
    The default local host and port for a "socks" remote is
    127.0.0.1:1080. Connections to this remote will terminate
    at the server's internal SOCKS5 proxy.

    When the wstunnel server has --reverse enabled, remotes can
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
    the wstunnel server. Authentication can be specified inside the URL.
    For example, http://admin:password@my-server.com:8081

    --hostname, Optionally set the 'Host' header (defaults to the host
    found in the server url).

    --pid Generate pid file in current working directory

    -v, Enable verbose logging

    --help, This help text

  Signals:
    The wstunnel process is listening for:
      a SIGUSR2 to print process stats, and
      a SIGHUP to short-circuit the client reconnect timer

  Version:
    1.0.0-src

  Read more:
    https://github.com/sammck-go/wstunnel

```
<!--/tmpl-->

### Security

Encryption is always enabled. In the default implementation, When you start up a wstunnel server,
it will generate an in-memory ECDSA public/private key pair. The public key fingerprint will be displayed as the server starts. Instead of generating
a random key, the server may optionally specify a key seed, using the `--key` option, which will be used to seed the key generation. When clients connect,
they will also display the server's public key fingerprint. The client can force a particular fingerprint using the `--fingerprint` option.
See the `--help` above for more information.

### Authentication/authorization

Using the `--authfile` option, the server may optionally provide a `user.json` configuration file to create a list of accepted users. The client then authenticates using the `--auth` option. See [users.json](example/users.json) for an example authentication configuration file. See the `--help` above for more information.

Internally, this is done using the _Password_ authentication method provided by SSH. Learn more about `crypto/ssh` here http://blog.gopheracademy.com/go-and-ssh/.

### SOCKS5 Guide

1. Start your wstunnel server

```sh
docker run \
  --name wstunnel -p 9312:9312 \
  -d --restart always \
  sammck/wstunnel server -p 9312 --socks5 --key supersecret
```

2. Connect your wstunnel client (using server's fingerprint)

```sh
wstunnel client --fingerprint ab:12:34 server-address:9312 socks
```

3. Point your SOCKS5 clients (e.g. OS/Browser) to:

```
localhost:1080
```

4. Now you have an encrypted, authenticated SOCKS5 connection over HTTP

### Known Issues

- WebSockets support is required
  _ IaaS providers all will support WebSockets
  _ Unless an unsupporting HTTP proxy has been forced in front of you, in which case I'd argue that you've been downgraded to PaaS.
  _ PaaS providers vary in their support for WebSockets
  _ Heroku has full support
  _ Openshift has full support though connections are only accepted on ports 8443 and 8080
  _ Google App Engine has **no** support (Track this on [their repo](https://code.google.com/p/googleappengine/issues/detail?id=2535))

### Contributing

- http://golang.org/doc/code.html
- http://golang.org/doc/effective_go.html
- `github.com/sammck-go/wstunnel/share` contains the shared package
- `github.com/sammck-go/wstunnel/server` contains the server package
- `github.com/sammck-go/wstunnel/client` contains the client package

### Changelog

- `1.0` - Initial release. Not yet ready for general use.

### Todo

- Better tests
- Refactor code into reusable packages
- Public key authentication

#### MIT License

Copyright © 2017 Jaime Pillora &lt;dev@sammck-go.com&gt;\
Copyright © 2021 Sam McKelvie &lt;dev@mckelvie.org&gt;

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
'Software'), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED 'AS IS', WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY
CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT,
TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.
