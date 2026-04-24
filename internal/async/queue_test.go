package async

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestQueuePushDrainFIFO(t *testing.T) {
	q := NewQueue(8)
	var out []int
	for i := 0; i < 5; i++ {
		i := i
		if !q.Push(func() { out = append(out, i) }) {
			t.Fatalf("push %d rejected", i)
		}
	}
	if n := q.Drain(); n != 5 {
		t.Fatalf("expected drain 5 items, got %d", n)
	}
	for i := 0; i < 5; i++ {
		if out[i] != i {
			t.Fatalf("fifo violated: %v", out)
		}
	}
}

func TestQueueTryPushRespectsCapacity(t *testing.T) {
	q := NewQueue(2)
	if !q.TryPush(func() {}) {
		t.Fatal("first trypush should succeed")
	}
	if !q.TryPush(func() {}) {
		t.Fatal("second trypush should succeed")
	}
	if q.TryPush(func() {}) {
		t.Fatal("third trypush should fail on full queue")
	}
	q.Drain()
	if !q.TryPush(func() {}) {
		t.Fatal("after drain, trypush should succeed")
	}
}

func TestQueuePushBlocksUntilDrain(t *testing.T) {
	q := NewQueue(1)
	_ = q.TryPush(func() {})
	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		q.Push(func() {})
		close(done)
	}()
	<-started
	select {
	case <-done:
		t.Fatal("push should block while queue is full")
	case <-time.After(20 * time.Millisecond):
	}
	q.Drain()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("push did not unblock after drain")
	}
}

func TestQueueShutdownUnblocksProducers(t *testing.T) {
	q := NewQueue(1)
	_ = q.TryPush(func() {})
	started := make(chan struct{})
	done := make(chan struct{})
	var pushOK atomic.Bool
	pushOK.Store(true)
	go func() {
		close(started)
		ok := q.Push(func() {})
		pushOK.Store(ok)
		close(done)
	}()
	<-started
	time.Sleep(10 * time.Millisecond)
	q.Shutdown()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("shutdown did not unblock producer")
	}
	if pushOK.Load() {
		t.Fatal("push after shutdown should return false")
	}
	if !q.IsShutdown() {
		t.Fatal("IsShutdown should be true after Shutdown")
	}
	if q.TryPush(func() {}) {
		t.Fatal("trypush after shutdown should fail")
	}
}

func TestQueueConcurrentProducers(t *testing.T) {
	q := NewQueue(16)
	var wg sync.WaitGroup
	var counter atomic.Int32
	producers := 8
	perProducer := 100
	for p := 0; p < producers; p++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perProducer; i++ {
				q.Push(func() { counter.Add(1) })
			}
		}()
	}
	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()
	total := producers * perProducer
	deadline := time.After(5 * time.Second)
	for counter.Load() < int32(total) {
		select {
		case <-deadline:
			t.Fatalf("timed out at %d/%d items drained", counter.Load(), total)
		default:
		}
		q.Drain()
		time.Sleep(time.Millisecond)
	}
	<-done
	q.Drain()
	if got := counter.Load(); got != int32(total) {
		t.Fatalf("expected %d executions, got %d", total, got)
	}
}

func TestQueueNilFuncIsNoop(t *testing.T) {
	q := NewQueue(1)
	if q.Push(nil) {
		t.Fatal("nil push should report false")
	}
	if q.TryPush(nil) {
		t.Fatal("nil trypush should report false")
	}
	if q.Len() != 0 {
		t.Fatal("nil pushes should not enqueue")
	}
}
