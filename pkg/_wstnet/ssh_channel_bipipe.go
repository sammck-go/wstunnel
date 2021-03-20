package wstnet

import (
	"fmt"

	"github.com/sammck-go/asyncobj"
	"github.com/sammck-go/logger"
	"golang.org/x/crypto/ssh"
)

// sshChannelBipipe wraps an ssh.Channel with a Bipipe interface
type sshChannelBipipe struct {
	// implements Bipipe
	ssh.Channel
	*asyncobj.Helper
	name string
}

// NewSshChannelBipipe wraps an existing ssh.Channel with a Bipipe interface. The returned
// Bipipe becomes the owner of the ssh.Channel and is responsible for closing it when the bipipe is closed.
func NewSshChanBipipe(logger logger.Logger, sshChannel ssh.Channel) Bipipe {
	name := fmt.Sprintf("<SshChannelBipipe %v>", sshChannel)
	bp := &sshChannelBipipe{
		Channel: sshChannel,
		name:    name,
	}
	bp.Helper = asyncobj.NewHelper(logger.ForkLogStr(name), bp)

	bp.SetIsActivated()

	return bp
}

func (bp *sshChannelBipipe) String() string {
	return bp.name
}

// Close shuts down the bipipe and waits for shutdown to complete
func (bp *sshChannelBipipe) Close() error {
	return bp.Helper.Close()
}

// HandleOnceShutdown will be called exactly once, in StateShuttingDown, in its own goroutine. It should take completionError
// as an advisory completion value, actually shut down, then return the real completion value.
// This method will never be called while shutdown is deferred.
func (bp *sshChannelBipipe) HandleOnceShutdown(completionError error) error {
	err := bp.Channel.Close()
	if completionError == nil {
		completionError = err
	}
	return completionError
}
