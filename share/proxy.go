package chshare

import (
	"context"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/ssh"
)

// GetSSHConn is a callback that is used to defer fetching of the ssh.Conn
// until after it is established
type GetSSHConn func() ssh.Conn

// TCPProxy proxies a single channel between a local stub endpoint
// and a remote skeleton endpoint
type TCPProxy struct {
	ShutdownHelper
	localChannelEnv LocalChannelEnv
	id              int
	strname         string
	count           int
	chd             *ChannelDescriptor
	ep              LocalStubChannelEndpoint
}

// NewTCPProxy creates a new TCPProxy
func NewTCPProxy(logger Logger, localChannelEnv LocalChannelEnv, index int, chd *ChannelDescriptor) *TCPProxy {
	id := index + 1
	strname := fmt.Sprintf("proxy#%d:%s", id, chd)
	myLogger := logger.Fork("%s", strname)
	p := &TCPProxy{
		localChannelEnv: localChannelEnv,
		id:              id,
		strname:         strname,
		chd:             chd,
	}
	p.InitShutdownHelper(myLogger, p)
	return p
}

func (p *TCPProxy) String() string {
	return p.strname
}

// HandleOnceShutdown will be called exactly once, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
func (p *TCPProxy) HandleOnceShutdown(completionErr error) error {
	return completionErr
}

// Start starts a listener for the local stub endpoint in the backgroud
func (p *TCPProxy) Start(ctx context.Context) error {
	// TODO this should be synchronous and not return until done, or
	// acceptLoop should not be included
	err := p.DoOnceActivate(
		func() error {
			ep, err := NewLocalStubChannelEndpoint(p.Logger, p.localChannelEnv, p.chd.Stub)
			if err != nil {
				return p.Errorf("Unable to create Stub endpoint from descriptor %s: %s", p.chd.Stub, err)
			}
			p.AddShutdownChild(ep)
			p.ShutdownOnContext(ctx)
			err = ep.StartListening()
			if err != nil {
				return p.Errorf("StartListening failed for %s: %s", p.chd.Stub, err)
			}
			p.ep = ep

			go p.acceptLoop(ctx)

			return nil
		},
		true,
	)
	return err
}

func (p *TCPProxy) acceptLoop(ctx context.Context) {
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			p.ILogf("Forcing close of listening endpoint %s: %s", p.chd.Stub, ctx.Err())
			p.ep.Close()
			p.DLogf("Done forcing close of listening endpoint")
		case <-done:
		}
	}()
	for {
		callerConn, err := p.ep.Accept(ctx)
		if err != nil {
			select {
			case <-ctx.Done():
				//listener closed
			default:
				p.ILogf("Accept error from %s, shutting down accept loop: %s", p.chd.Stub, err)
			}
			close(done)
			return
		}
		go p.runWithLocalCallerConn(ctx, callerConn)
	}
}

func (p *TCPProxy) runWithLocalCallerConn(ctx context.Context, callerConn ChannelConn) error {
	subCtx, subCtxCancel := context.WithCancel(ctx)
	defer subCtxCancel()

	p.count++

	p.DLogf("TCPProxy Open, getting remote connection")
	sshPrimaryConn, err := p.localChannelEnv.GetSSHConn()
	if err != nil {
		return p.DLogErrorf("Unable to fetch sshPrimaryConn , exiting proxy: %s", err)
	}

	if sshPrimaryConn == nil {
		callerConn.Close()
		return p.DLogErrorf("SSH primary connection, exiting proxy")
	}

	//ssh request for tcp connection for this proxy's remote skeleton endpoint
	skeletonEndpointJSON, err := json.Marshal(p.chd.Skeleton)
	if err != nil {
		callerConn.Close()
		return p.DLogErrorf("Unable to serialize endpoint descriptor '%s': %s", p.chd.Skeleton, err)
	}

	serviceSSHConn, reqs, err := sshPrimaryConn.OpenChannel("chisel", skeletonEndpointJSON)
	if err != nil {
		callerConn.Close()
		return p.DLogErrorf("SSH open channel to remote endpoint %s failed: %s", p.chd.Skeleton, err)
	}

	// will terminate when serviceSSHConn is closed
	go ssh.DiscardRequests(reqs)

	serviceConn, err := NewSSHConn(p.Logger, serviceSSHConn)
	if err != nil {
		sshCloseErr := serviceSSHConn.Close()
		if sshCloseErr != nil {
			p.DLogf("Cose of ssh.Conn failed, ignoring: %s", sshCloseErr)
		}
		callerConn.Close()
		return p.DLogErrorf("SSH open channel to remote endpoint %s failed: %s", p.chd.Skeleton, err)
	}

	callerToService, serviceToCaller, err := BasicBridgeChannels(subCtx, p.Logger, callerConn, serviceConn)
	if err == nil {
		p.DLogf("Proxy Connection for %s ended normally, caller sent %d bytes, service sent %d bytes",
			p.chd, callerToService, serviceToCaller)
	} else {
		return p.DLogErrorf("Proxy conn for %s failed after %d bytes to service, %d bytes to caller: %s",
			p.chd, callerToService, serviceToCaller, err)
	}
	return nil
}
