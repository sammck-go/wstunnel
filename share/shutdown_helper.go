package chshare

import (
	"context"
	"sync"
)

// OnceActivateHandler is a function that is called exactly once with shutdown paused
// to activate the object that supports shutdown.
// If it returns nil, the object will be activated. If it returns an error, the object will not be activated,
// and shutdown will be immediately started.
// If shutdown has already started before DoOnceActivate is called, this function will not be invoked.
type OnceActivateHandler func() error

// OnceShutdownHandler is an interface that must be implemented by the object managed by ShutdownHelper
type OnceShutdownHandler interface {
	// Shutdown will be called exactly once, in its own goroutine. It should take completionError
	// as an advisory completion value, actually shut down, then return the real completion value.
	// This method will never be called while shutdown is paused.
	HandleOnceShutdown(completionError error) error
}

// AsyncShutdowner is an interface implemented by objects that provide
// asynchronous shutdown capability.
type AsyncShutdowner interface {
	// StartShutdown schedules asynchronous shutdown of the object. If the object
	// has already been scheduled for shutdown, it has no effect.
	// completionErr is an advisory error (or nil) to use as the completion status
	// from WaitShutdown(). The implementation may use this value or decide to return
	// something else.
	StartShutdown(completionErr error)

	// ShutdownDoneChan returns a chan that is closed after shutdown is complete.
	// After this channel is closed, it is guaranteed that IsDoneShutdown() will
	// return true, and WaitForShutdown will not block.
	ShutdownDoneChan() <-chan struct{}

	// IsDoneShutdown returns false if the object is not yet completely
	// shut down. Otherwise it returns true with the guarantee that
	// ShutDownDoneChan() will be immediately closed and WaitForShutdown
	// will immediately return the final status.
	IsDoneShutdown() bool

	// WaitShutdown blocks until the object is completely shut down, and
	// returns the final completion status
	WaitShutdown() error
}

// ShutdownHelper is a base that manages clean asynchronous object shutdown for an
// object that implements OnceShutdownHandler
type ShutdownHelper struct {
	// Logger is the Logger that will be used for log output from this helper
	Logger

	// Lock is a general-purpose fine-grained mutex for this helper; it may be used
	// as a general-purpose lock by derived objects as well
	Lock sync.Mutex

	// The object that is being managed by this helper, which is called exactly once
	// to perform synchronous shutdown.
	shutdownHandler OnceShutdownHandler

	// shutdownPauseCount is the number of times ResumeShutdown() must be called before
	// shutdown can commence
	shutdownPauseCount int

	// isActivated is set to true when Activate is called
	isActivated bool

	// isScheduledShutdown is set to true when StartShutdown is called
	isScheduledShutdown bool

	// isStartedShutdown is set to true when we begin shutting down
	isStartedShutdown bool

	// isDoneShutdown is set to true when shutdown is completely done
	isDoneShutdown bool

	// shutdownErr contains the final completion statis after isDoneShutdown is true
	shutdownErr error

	// shutdownStartedChan is a chan that is closed when shutdown is started
	shutdownStartedChan chan struct{}

	// shutdownHandlerDoneChan is a chan that is closed after shutdownHandler
	// returns, before we begin waiting on the waitgroup. It is used
	// to wake up goroutiness that actively shutdown children.
	shutdownHandlerDoneChan chan struct{}

	// shutdownDoneChan is a chan that is closed when shutdown is completely done
	shutdownDoneChan chan struct{}

	// wg is a sync.WaitGroup that this helper will wait on before it considers shutdown
	// to be complete. it is incremented for each child that we are waiting on.
	wg sync.WaitGroup
}

// InitShutdownHelper initializes a new ShutdownHelper in place
func (h *ShutdownHelper) InitShutdownHelper(
	logger Logger,
	shutdownHandler OnceShutdownHandler,
) {
	h.Logger = logger
	h.shutdownHandler = shutdownHandler
	h.shutdownStartedChan = make(chan struct{})
	h.shutdownHandlerDoneChan = make(chan struct{})
	h.shutdownDoneChan = make(chan struct{})
}

// NewShutdownHelper creates a new ShutdownHelper on the heap
func NewShutdownHelper(
	logger Logger,
	shutdownHandler OnceShutdownHandler,
) *ShutdownHelper {
	h := &ShutdownHelper{
		Logger:                  logger,
		shutdownHandler:         shutdownHandler,
		shutdownStartedChan:     make(chan struct{}),
		shutdownHandlerDoneChan: make(chan struct{}),
		shutdownDoneChan:        make(chan struct{}),
	}
	return h
}

// asyncDoStartedShutdown starts background processing of shutdown *after*
// h.isStartedShutdown has already been set to true and h.shutdownErr has been set
// to the advisory completion error
func (h *ShutdownHelper) asyncDoStartedShutdown() {
	h.DLogf("->shutdownStarted")
	close(h.shutdownStartedChan)
	go func() {
		// Wait til init is done before shutting down
		h.shutdownErr = h.shutdownHandler.HandleOnceShutdown(h.shutdownErr)
		h.DLogf("->shutdownHandlerDone")
		close(h.shutdownHandlerDoneChan)
		h.wg.Wait()
		h.isDoneShutdown = true
		h.DLogf("->shutdownDone")
		close(h.shutdownDoneChan)
	}()
}

// PauseShutdown increments the shutdown pause count, preventing shutdown from starting. Returns an error
// if shutdown has already started. Note that pausing does not prevent shutdown from being scheduled
// with StartShutDown(), it just prevents actual async shutdown from beginning. Each successful call
// to PauseShutdown must pair with a matching call to ResumeShutdown.
func (h *ShutdownHelper) PauseShutdown() error {
	h.Lock.Lock()
	defer h.Lock.Unlock()
	if h.isStartedShutdown {
		return h.Errorf("Shutdown already started; cannot pause")
	}
	h.shutdownPauseCount++
	return nil
}

// IsActivated returns true if this helper has been activated
func (h *ShutdownHelper) IsActivated() bool {
	return h.isActivated
}

// Activate Sets the "activated" flag for this helper. Does nothing
// if already activated. Fails if shutdown has already been started.
func (h *ShutdownHelper) Activate() error {
	h.Lock.Lock()
	defer h.Lock.Unlock()

	if !h.isActivated {
		if h.isStartedShutdown {
			return h.Errorf("Cannot activate; shutdown already initiated")
		}
		h.isActivated = true
	}

	return nil
}

// DoOnceActivate takes steps to activate the object:
//
//     if already activated, returns nil
//     if not activated and already started shutting down:
//        if waitOnFail is true, waits for shutdown to complete
//        returns an error
//     if not activated and not shutting down:
//        pauses shutdown
//        invokes the OnceActivateHandler
//        resumes shutdown
//        if handler returns nil:
//           activates the object
//           if activation succeeds, returns nil
//        if handler or activation returns an error:
//           starts shutting down with that error
//           if waitOnFail is true, waits for shutdown to complete
//           returns an error
//        returns nil
func (h *ShutdownHelper) DoOnceActivate(onceActivateHandler OnceActivateHandler, waitOnFail bool) error {
	var err error
	h.Lock.Lock()
	if h.isActivated {
		h.Lock.Unlock()
		return nil
	}
	if h.isStartedShutdown {
		h.Lock.Unlock()
		if waitOnFail {
			err = h.WaitShutdown()
		}
		if err == nil {
			err = h.Errorf("Shutdown already started; cannot Activate")
		}
		return err
	}
	h.shutdownPauseCount++
	h.Lock.Unlock()
	err = onceActivateHandler()
	if err == nil {
		err = h.Activate()
	}
	if err != nil {
		h.StartShutdown(err)
	}
	h.ResumeShutdown()
	if err != nil && waitOnFail {
		h.WaitShutdown()
	}
	return err
}

// ResumeShutdown decrements the shutdown pause count, and if it becomes zero, allows shutdown to start
func (h *ShutdownHelper) ResumeShutdown() {
	h.Lock.Lock()
	if h.shutdownPauseCount < 1 {
		h.Panic("ResumeShutdown before PauseShutdown")
		return
	}
	h.shutdownPauseCount--
	doShutdownNow := h.shutdownPauseCount == 0 && h.isScheduledShutdown && !h.isStartedShutdown
	if doShutdownNow {
		h.isStartedShutdown = true
	}
	h.Lock.Unlock()

	if doShutdownNow {
		h.asyncDoStartedShutdown()
	}
}

// ResumeAndShutdown decrements the shutdown pause count and immediately shuts down.
// returns the final completion code. This method is suitable for use in a defer
// statement after PauseShutdown
func (h *ShutdownHelper) ResumeAndShutdown(completionErr error) error {
	h.ResumeShutdown()
	return h.Shutdown(completionErr)
}

// ResumeAndStartShutdownIfNotActivated decrements the shutdown pause count and then
// immediately starts shutting down if the helper has not yet been activated.
// This method is suitable for use in a defer statement after PauseShutdown
func (h *ShutdownHelper) ResumeAndStartShutdownIfNotActivated(completionErr error) error {
	h.ResumeShutdown()
	return h.Shutdown(completionErr)
}

// ResumeAndWaitShutdown decrements the shutdown pause count and waits for shutdown.
// returns the final completion code. This method is suitable for use in a defer
// statement after PauseShutdown
func (h *ShutdownHelper) ResumeAndWaitShutdown(completionErr error) error {
	h.ResumeShutdown()
	return h.WaitShutdown()
}

// ShutdownOnContext begins background monitoring of a context.Context, and
// will begin asynchronously shutting down this helper with the context's error
// if the context is completed. This method does not block, it just
// constrains the lifetime of this object to a context.
func (h *ShutdownHelper) ShutdownOnContext(ctx context.Context) {
	go func() {
		select {
		case <-h.shutdownStartedChan:
		case <-ctx.Done():
			h.StartShutdown(ctx.Err())
		}
	}()
}

// IsScheduledShutdown returns true if StartShutdown() has been called. It continues to return true after shutdown
// is started and completes
func (h *ShutdownHelper) IsScheduledShutdown() bool {
	return h.isScheduledShutdown
}

// IsStartedShutdown returns true if shutdown has begun. It continues to return true after shutdown
// is complete
func (h *ShutdownHelper) IsStartedShutdown() bool {
	return h.isStartedShutdown
}

// IsDoneShutdown returns true if shutdown is complete.
func (h *ShutdownHelper) IsDoneShutdown() bool {
	return h.isDoneShutdown
}

// ShutdownWG returns a sync.WaitGroup that you can call Add() on to
// defer final completion of shutdown until the specified number of calls to
// ShutdownWG().Done() are made
func (h *ShutdownHelper) ShutdownWG() *sync.WaitGroup {
	return &h.wg
}

// ShutdownStartedChan returns a channel that will be closed as soon as shutdown is initiated
func (h *ShutdownHelper) ShutdownStartedChan() <-chan struct{} {
	return h.shutdownDoneChan
}

// ShutdownHandlerDoneChan returns a channel that will be closed after shutdownHandler
// returns, but before children are shut down and waited for
func (h *ShutdownHelper) ShutdownHandlerDoneChan() <-chan struct{} {
	return h.shutdownHandlerDoneChan
}

// ShutdownDoneChan returns a channel that will be closed after shutdown is done
func (h *ShutdownHelper) ShutdownDoneChan() <-chan struct{} {
	return h.shutdownDoneChan
}

// WaitShutdown waits for the shutdown to complete, then returns the shutdown status
// It does not initiate shutdown, so it can be used to wait on an object that
// will shutdown at an unspecified point in the future.
func (h *ShutdownHelper) WaitShutdown() error {
	<-h.shutdownDoneChan
	return h.shutdownErr
}

// Shutdown performs a synchronous shutdown, It initiates shutdown if it has
// not already started, waits for the shutdown to comlete, then returns
// the final shutdown status
func (h *ShutdownHelper) Shutdown(completionError error) error {
	h.StartShutdown(completionError)
	return h.WaitShutdown()
}

// StartShutdown shedules asynchronous shutdown of the object. If the object
// has already been scheduled for shutdown, it has no effect. If shutting down has
// been paused, actual starting of the shutdown process is deferred.
// "completionError" is an advisory error (or nil) to use as the completion status
// from WaitShutdown(). The implementation may use this value or decide to return
// something else.
//
// Asynchronously, this will help kick off the following, only the first time it is called:
//
//  -   Signal that shutdown has been scheduled
//  -   Wait for shutdown pause count to reach 0
//  -   Signal that shutdown has started
//  -   Invoke HandleOnceShutdown with the provided avdvisory completion status. The
//       return value will be used as the final completion status for shutdown
//  -   Signal that HandleOnceShutdown has completed
//  -   For each registered child, call StartShutdown, using the return value from
//       HandleOnceShutdown as an advirory completion status.
//  -   For each registered child, wait for the
//       child to finish shuting down
//  -   For each manually added child done chan, wait for the
//       child done chan to be closed
//  -   Wait for the wait group count to reach 0
//  -   Signals shutdown complete, using the return value from HandleOnceShutdown
//  -    as the final completion code
func (h *ShutdownHelper) StartShutdown(completionErr error) {
	var doShutdownNow bool
	h.Lock.Lock()
	if !h.isScheduledShutdown {
		if h.isStartedShutdown {
			h.Panic("shutdown started before scheduled")
		}
		h.shutdownErr = completionErr
		h.isScheduledShutdown = true
		doShutdownNow = (h.shutdownPauseCount == 0)
		h.isStartedShutdown = doShutdownNow
	}
	h.Lock.Unlock()

	if doShutdownNow {
		h.asyncDoStartedShutdown()
	}
}

// Close is a default implementation of Close(), which simply shuts down
// with an advisory completion status of nil, and returns the final completion
// status
func (h *ShutdownHelper) Close() error {
	h.DLogf("Close()")
	return h.Shutdown(nil)
}

// AddShutdownChildChan adds a chan that will be waited on before this object's shutdown is
// considered complete. The helper will not take any action to cause the chan to be
// closed; it is the caller's responsibility to do that.
func (h *ShutdownHelper) AddShutdownChildChan(childDoneChan <-chan struct{}) {
	h.DLogf("AddShutdownChildChan()")
	h.wg.Add(1)
	go func() {
		<-childDoneChan
		h.wg.Done()
	}()
}

// AddShutdownChild adds a child object to the set of objects that will be
// actively shut down by this helper after HandleOnceShutdown() returns, before this
// object's shutdown is considered complete. The child will be shut down with an advisory
// completion status equal to the status returned from HandleOnceShutdown
func (h *ShutdownHelper) AddShutdownChild(child AsyncShutdowner) {
	h.DLogf("AddShutdownChild(\"%s\")", child)
	h.wg.Add(1)
	go func() {
		select {
		case <-child.ShutdownDoneChan():
			h.DLogf("Shutdown of child done, signalling wg: \"%s\"", child)
		case <-h.shutdownHandlerDoneChan:
			h.DLogf("Shutdown handler done, shutting down child child \"%s\"", child)
			child.StartShutdown(h.shutdownErr)
			child.WaitShutdown()
			h.DLogf("Shutdown of child done, signalling wg: \"%s\"", child)
		}
		h.wg.Done()
	}()
}
