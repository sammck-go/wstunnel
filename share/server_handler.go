package chshare

import (
	"context"
	"github.com/gorilla/websocket"
	"io"
	"net/http"
	"strings"
)

// handleClientHandler is the main http websocket handler for the chisel server
func (s *Server) handleClientHandler(ctx context.Context, w http.ResponseWriter, r *http.Request) {
	//websockets upgrade AND has chisel prefix
	upgrade := strings.ToLower(r.Header.Get("Upgrade"))
	if upgrade == "websocket" {
		protocol := r.Header.Get("Sec-WebSocket-Protocol")
		if strings.HasPrefix(protocol, "sammck-chisel-") {
			if protocol == ProtocolVersion {
				s.DLogf("Upgrading to websocket, URL tail=\"%s\", protocol=\"%s\"", r.URL.String(), protocol)
				wsConn, err := upgrader.Upgrade(w, r, nil)
				if err != nil {
					err = s.DLogErrorf("Failed to upgrade to websocket: %s", err)
					http.Error(w, err.Error(), 503)
					return
				}

				go func() {
					s.handleWebsocket(ctx, wsConn)
					wsConn.Close()
				}()

				return
			}

			s.ILogf("Client connection using unsupported websocket protocol '%s', expected '%s'",
				protocol, ProtocolVersion)

			http.Error(w, "Not Found", 404)
			return
		}
	}

	//proxy target was provided
	if s.reverseProxy != nil {
		s.reverseProxy.ServeHTTP(w, r)
		return
	}

	//no proxy defined, provide access to health/version checks
	switch r.URL.String() {
	case "/health":
		w.Write([]byte("OK\n"))
		return
	case "/version":
		w.Write([]byte(BuildVersion))
		return
	}

	http.Error(w, "Not Found", 404)
}

// handleWebsocket handles an incoming client request that is intended tois responsible for handling the websocket connection
// It upgrades . It is guaranteed on return
//
func (s *Server) handleWebsocket(ctx context.Context, wsConn *websocket.Conn) {
	session, err := NewServerSSHSession(s)
	if err != nil {
		session.DLogf("Failed to create ServerSSHSession: %s", err)
		return
	}
	s.AddShutdownChild(session)
	session.ShutdownOnContext(ctx)
	conn := NewWebSocketConn(wsConn)
	session.Run(ctx, conn)
	conn.Close() // closes the websocket too
	session.Close()
}

func (s *Server) handleSocksStream(l Logger, src io.ReadWriteCloser) {
	conn := NewRWCConn(src)
	s.connStats.Open()
	l.DLogf("%v Opening", s.connStats)
	err := s.socksServer.ServeConn(conn)
	s.connStats.Close()
	if err != nil && !strings.HasSuffix(err.Error(), "EOF") {
		l.DLogf("%v: Closed (error: %s)", s.connStats, err)
	} else {
		l.DLogf("%v: Closed", s.connStats)
	}
}
