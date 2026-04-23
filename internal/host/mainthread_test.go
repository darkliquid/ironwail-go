package host

import (
	"sync"
	"testing"
	"time"
)

func TestInvokeOnMainThreadDrainsOnFrame(t *testing.T) {
	h := NewHost()
	var mu sync.Mutex
	var called []string

	push := func(tag string) {
		h.InvokeOnMainThread(func() {
			mu.Lock()
			called = append(called, tag)
			mu.Unlock()
		})
	}

	push("a")
	push("b")

	// Before drain, nothing executes.
	mu.Lock()
	if len(called) != 0 {
		mu.Unlock()
		t.Fatalf("expected no executions before drain, got %v", called)
	}
	mu.Unlock()

	if n := h.drainMainThreadQueue(); n != 2 {
		t.Fatalf("expected 2 drained, got %d", n)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(called) != 2 || called[0] != "a" || called[1] != "b" {
		t.Fatalf("fifo order broken: %v", called)
	}
}

func TestTryInvokeOnMainThreadNonBlocking(t *testing.T) {
	h := NewHost()
	for i := 0; i < 10; i++ {
		if !h.TryInvokeOnMainThread(func() {}) {
			t.Fatalf("trypush %d rejected within capacity", i)
		}
	}
	if n := h.drainMainThreadQueue(); n != 10 {
		t.Fatalf("expected 10 drained, got %d", n)
	}
}

func TestShutdownMainThreadQueueReleasesProducers(t *testing.T) {
	h := NewHost()
	// Fill queue to capacity 1024 then attempt a blocking push from
	// another goroutine to make sure shutdown releases it.
	q := h.MainThreadQueue()
	for i := 0; i < 1024; i++ {
		q.TryPush(func() {})
	}
	done := make(chan struct{})
	go func() {
		h.InvokeOnMainThread(func() {})
		close(done)
	}()
	time.Sleep(10 * time.Millisecond)
	h.shutdownMainThreadQueue()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not unblock main-thread producer")
	}
}

func TestInvokeOnMainThreadNilReceiverSafe(t *testing.T) {
	var h *Host
	if h.InvokeOnMainThread(func() {}) {
		t.Fatal("nil host should reject push")
	}
	if h.TryInvokeOnMainThread(func() {}) {
		t.Fatal("nil host should reject trypush")
	}
	if h.drainMainThreadQueue() != 0 {
		t.Fatal("nil host should drain 0")
	}
	h.shutdownMainThreadQueue()
}
