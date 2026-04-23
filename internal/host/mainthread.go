package host

import "github.com/darkliquid/ironwail-go/internal/async"

// InvokeOnMainThread enqueues fn to run during the next Host.Frame
// pump on the goroutine that drives the frame loop. This mirrors
// Host_InvokeOnMainThread in C Ironwail (host.c), which background
// worker threads use to marshal completion callbacks back to the
// main thread without racing client state.
//
// Returns false if the queue has been shut down or fn is nil.
func (h *Host) InvokeOnMainThread(fn func()) bool {
	if h == nil || h.mainThreadQueue == nil {
		return false
	}
	return h.mainThreadQueue.Push(fn)
}

// TryInvokeOnMainThread is the non-blocking variant of
// InvokeOnMainThread. Returns false if the queue is full or shut down.
func (h *Host) TryInvokeOnMainThread(fn func()) bool {
	if h == nil || h.mainThreadQueue == nil {
		return false
	}
	return h.mainThreadQueue.TryPush(fn)
}

// MainThreadQueue returns the async queue used for main-thread
// marshaling. Primarily exposed for tests and subsystems that need
// to share the queue (e.g. a save worker) without reaching into
// the Host struct.
func (h *Host) MainThreadQueue() *async.Queue {
	if h == nil {
		return nil
	}
	return h.mainThreadQueue
}

// drainMainThreadQueue runs any pending main-thread callbacks.
// Called at the top of Host.Frame before simulation advances.
func (h *Host) drainMainThreadQueue() int {
	if h == nil || h.mainThreadQueue == nil {
		return 0
	}
	n := h.mainThreadQueue.Drain()
	if n > 0 && hostDebugSysEnabled(2) {
		hostDebugSysLogfAt(2, "mainthread", "drained=%d", n)
	}
	return n
}

// shutdownMainThreadQueue is invoked during host teardown to unblock
// any background producers and drain remaining work.
func (h *Host) shutdownMainThreadQueue() {
	if h == nil || h.mainThreadQueue == nil {
		return
	}
	h.mainThreadQueue.Shutdown()
}
