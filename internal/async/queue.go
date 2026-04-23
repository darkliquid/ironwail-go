// Package async provides a minimal thread-safe work queue used to marshal
// work from background goroutines back onto a single "main" thread/frame
// pump, matching the semantics of C Ironwail's host.c AsyncQueue.
//
// Idiomatic Go would often just use an unbounded channel, but the C
// reference defines a bounded queue that blocks producers when full and
// atomically drains on the consumer's tick. We mirror that exactly so
// background systems (save worker, mod downloader) can push completion
// callbacks that Host.Frame drains once per frame without racing client
// state.
package async

import (
	"sync"
)

// Queue is a bounded FIFO of work items. Push blocks when the queue is
// full, unless the queue has been shut down. Drain is intended to be
// invoked by the main thread (e.g. once per frame) and runs pending
// work synchronously in FIFO order.
type Queue struct {
	mu       sync.Mutex
	cond     *sync.Cond
	items    []func()
	head     int
	capacity int
	shutdown bool
}

// NewQueue creates a queue with the given capacity. Capacity is
// clamped to a minimum of 1. Push callers will block while the queue
// contains capacity items.
func NewQueue(capacity int) *Queue {
	if capacity < 1 {
		capacity = 1024
	}
	q := &Queue{
		capacity: capacity,
	}
	q.cond = sync.NewCond(&q.mu)
	return q
}

// Push enqueues a work item. If the queue is full, Push blocks until
// space is available or the queue is shut down. Pushing to a
// shut-down queue is a no-op and returns false.
func (q *Queue) Push(fn func()) bool {
	if fn == nil {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	for !q.shutdown && q.lenLocked() >= q.capacity {
		q.cond.Wait()
	}
	if q.shutdown {
		return false
	}
	q.items = append(q.items, fn)
	return true
}

// TryPush enqueues a work item without blocking. Returns false if the
// queue is full or shut down.
func (q *Queue) TryPush(fn func()) bool {
	if fn == nil {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.shutdown || q.lenLocked() >= q.capacity {
		return false
	}
	q.items = append(q.items, fn)
	return true
}

// Drain invokes all currently pending work items in FIFO order on the
// calling goroutine. Items pushed while Drain is running are deferred
// to the next call. Returns the number of items executed.
func (q *Queue) Drain() int {
	q.mu.Lock()
	pending := q.items[q.head:]
	q.items = nil
	q.head = 0
	q.mu.Unlock()

	for _, fn := range pending {
		fn()
	}

	if len(pending) > 0 {
		q.mu.Lock()
		q.cond.Broadcast()
		q.mu.Unlock()
	}
	return len(pending)
}

// Len returns the current queued item count.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.lenLocked()
}

func (q *Queue) lenLocked() int {
	return len(q.items) - q.head
}

// Shutdown marks the queue closed, unblocking any producers. Pending
// items are drained once by the caller and then ignored.
func (q *Queue) Shutdown() {
	q.mu.Lock()
	q.shutdown = true
	q.mu.Unlock()
	q.cond.Broadcast()
	q.Drain()
}

// IsShutdown reports whether Shutdown has been called.
func (q *Queue) IsShutdown() bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.shutdown
}
