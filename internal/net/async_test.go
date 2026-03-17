package net

import (
	"sync/atomic"
	"testing"
	"time"
)

func TestAsyncReceiver_ReceivesMessages(t *testing.T) {
	var callCount atomic.Int32
	poll := func() (int, []byte) {
		n := callCount.Add(1)
		if n <= 3 {
			return 1, []byte{byte(n)}
		}
		return 0, nil // no more messages
	}

	recv := NewAsyncReceiver(poll, 8, time.Millisecond)
	defer recv.Stop()

	// Read 3 messages
	for i := 0; i < 3; i++ {
		select {
		case msg := <-recv.Messages():
			if msg.Type != 1 {
				t.Errorf("message %d: expected type 1, got %d", i, msg.Type)
			}
		case <-time.After(time.Second):
			t.Fatalf("timeout waiting for message %d", i)
		}
	}
}

func TestAsyncReceiver_Drain(t *testing.T) {
	var callCount atomic.Int32
	poll := func() (int, []byte) {
		n := callCount.Add(1)
		if n <= 5 {
			return 1, []byte{byte(n)}
		}
		return 0, nil
	}

	recv := NewAsyncReceiver(poll, 16, time.Millisecond)
	// Give goroutine time to buffer messages
	time.Sleep(50 * time.Millisecond)

	msgs := recv.Drain()
	recv.Stop()

	if len(msgs) == 0 {
		t.Error("expected at least some drained messages")
	}
	for _, m := range msgs {
		if m.Type != 1 {
			t.Errorf("unexpected message type: %d", m.Type)
		}
	}
}

func TestAsyncReceiver_StopIdempotent(t *testing.T) {
	recv := NewAsyncReceiver(func() (int, []byte) { return 0, nil }, 4, time.Millisecond)
	recv.Stop()
	recv.Stop() // must not panic
}

func TestAsyncReceiver_DataCopied(t *testing.T) {
	shared := []byte{1, 2, 3}
	poll := func() (int, []byte) {
		return 1, shared
	}

	recv := NewAsyncReceiver(poll, 4, time.Millisecond)

	select {
	case msg := <-recv.Messages():
		// Mutate shared buffer
		shared[0] = 99
		// Received message should be unaffected
		if msg.Data[0] != 1 {
			t.Error("message data was not copied — mutation leaked")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
	recv.Stop()
}
