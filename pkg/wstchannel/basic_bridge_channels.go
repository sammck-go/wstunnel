package wstchannel

import (
	"context"
	"io"
	"sync"
	"sync/atomic"
)

var lastBasicBridgeNum int64 = 0

// BasicBridgeChannels connects two ChannelConn's together, copying betweeen them bi-directionally
// until end-of-stream is reached in both directions. Both channels are closed before this function
// returns. Three values are returned:
//    Number of bytes transferred from caller to calledService
//    Number of bytes transferred from calledService to caller
//    If io.Copy() returned an error in either direction, the error value.
//
// CloseWrite() is called on each channel after transfer to that channel is complete.
//
// Currently the context is not used and there is no way to cancel the bridge without closing
// one of the ChannelConn's.
func BasicBridgeChannels(
	ctx context.Context,
	logger Logger,
	caller ChannelConn,
	calledService ChannelConn,
) (int64, int64, error) {
	bridgeNum := atomic.AddInt64(&lastBasicBridgeNum, 1)
	logger = logger.Fork("BasicBridge#%d (%s->%s)", bridgeNum, caller, calledService)
	logger.DLogf("Starting")
	var callerToServiceBytes, serviceToCallerBytes int64
	var callerToServiceErr, serviceToCallerErr error
	var wg sync.WaitGroup
	wg.Add(2)
	copyFunc := func(src ChannelConn, dst ChannelConn, bytesCopied *int64, copyErr *error) {
		// Copy from caller to calledService
		*bytesCopied, *copyErr = io.Copy(dst, src)
		if *copyErr != nil {
			logger.DLogf("io.Copy(%s->%s) returned error: %s", src, dst, *copyErr)
		}
		logger.DLogf("Done with io.Copy(%s->%s); shutting down write side", src, dst)
		dst.CloseWrite()
		logger.DLogf("Done with write side shutdown of %s->%s", src, dst)
		wg.Done()
	}
	go copyFunc(caller, calledService, &callerToServiceBytes, &callerToServiceErr)
	go copyFunc(calledService, caller, &serviceToCallerBytes, &serviceToCallerErr)
	wg.Wait()
	logger.DLogf("Wait complete")
	logger.DLogf("callerToService=%d, err=%s", callerToServiceBytes, callerToServiceErr)
	logger.DLogf("serviceToCaller=%d, err=%s", serviceToCallerBytes, serviceToCallerErr)
	logger.DLogf("Closing calledService")
	calledService.Close()
	logger.DLogf("Closing caller")
	caller.Close()
	err := callerToServiceErr
	if err == nil {
		err = serviceToCallerErr
	}
	logger.DLogf("Exiting, callerToService=%d, serviceToCaller=%d, err=%s", callerToServiceBytes, serviceToCallerBytes, err)
	return callerToServiceBytes, serviceToCallerBytes, err
}
