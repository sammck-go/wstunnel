package chshare

import (
	"context"
	"net"
	"net/http"
)

//HTTPServer extends net/http Server and
//adds graceful shutdowns
type HTTPServer struct {
	ShutdownHelper
	*http.Server
	listener net.Listener
}

//NewHTTPServer creates a new HTTPServer
func NewHTTPServer(logger Logger) *HTTPServer {
	h := &HTTPServer{
		Server:   &http.Server{},
		listener: nil,
	}
	h.InitShutdownHelper(logger, h)
	return h
}

// HandleOnceShutdown will be called exactly once, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
func (h *HTTPServer) HandleOnceShutdown(completionErr error) error {
	h.DLogf("HandleOnceShutdown")
	err := h.listener.Close()
	if err != nil {
		h.DLogf("HTTPserver: close of listener failed, ignoring: %s", err)
	}
	if completionErr == nil {
		completionErr = err
	}
	return completionErr
}

// ListenAndServe Runs the HTTP server
// on the given bind address, invoking the provided handler for each
// request. It returns after the server has shutdown. The server can be
// shutdown either by cancelling the context or by calling Shutdown().
func (h *HTTPServer) ListenAndServe(ctx context.Context, addr string, handler http.Handler) error {

	err := h.DoOnceActivate(
		func() error {
			h.ShutdownOnContext(ctx)

			l, err := net.Listen("tcp", addr)
			if err != nil {
				return h.DLogErrorf("Listen failed: %s", err)
			}
			h.Handler = handler
			h.listener = l

			go func() {
				h.Shutdown(h.Serve(l))
			}()

			return nil
		},
		true,
	)
	if err == nil {
		err = h.WaitShutdown()
	}
	return err
}

// Shutdown completely shuts down the server, then returns the final completion code
func (h *HTTPServer) Shutdown(completionError error) error {
	return h.ShutdownHelper.Shutdown(completionError)
}

// Close completely shuts down the server, then returns the final completion code
func (h *HTTPServer) Close() error {
	return h.ShutdownHelper.Close()
}
