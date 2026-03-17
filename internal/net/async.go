package net

import (
	"sync"
	"time"
)

// ReceivedMessage represents a message received from the network.
// The game loop reads these from a channel instead of polling.
type ReceivedMessage struct {
	Type int    // Message type (0=none, 1=reliable, 2=unreliable)
	Data []byte // Raw message bytes (owned by receiver — safe to retain)
}

// AsyncReceiver runs a background goroutine that polls a socket for incoming
// messages and delivers them via a channel. This decouples network I/O timing
// from the game frame loop, allowing messages to be buffered between frames.
//
// The Quake engine traditionally polls for messages synchronously each frame.
// AsyncReceiver preserves this semantic but moves the polling to a goroutine,
// so the game thread can process buffered messages without blocking on I/O.
//
// Usage:
//
// recv := net.NewAsyncReceiver(sock, pollFunc)
// defer recv.Stop()
//
//	for msg := range recv.Messages() {
//	   processMessage(msg)
//	}
type AsyncReceiver struct {
	messages chan ReceivedMessage
	stop     chan struct{}
	done     chan struct{}
	once     sync.Once
}

// PollFunc is the function called to poll for a message. Returns message type
// and data. Type 0 means no message available. This matches the signature
// of GetMessage().
type PollFunc func() (int, []byte)

// NewAsyncReceiver starts a background goroutine that polls for messages
// using the provided poll function. Messages are buffered in a channel
// of the given capacity. The poll loop sleeps for pollInterval between
// empty polls to avoid busy-waiting.
func NewAsyncReceiver(poll PollFunc, bufferSize int, pollInterval time.Duration) *AsyncReceiver {
	if bufferSize <= 0 {
		bufferSize = 32
	}
	if pollInterval <= 0 {
		pollInterval = time.Millisecond
	}
	r := &AsyncReceiver{
		messages: make(chan ReceivedMessage, bufferSize),
		stop:     make(chan struct{}),
		done:     make(chan struct{}),
	}
	go r.run(poll, pollInterval)
	return r
}

func (r *AsyncReceiver) run(poll PollFunc, interval time.Duration) {
	defer close(r.done)
	defer close(r.messages)

	for {
		select {
		case <-r.stop:
			return
		default:
		}

		msgType, data := poll()
		if msgType == 0 {
			// No message — sleep briefly to avoid busy-waiting
			select {
			case <-r.stop:
				return
			case <-time.After(interval):
			}
			continue
		}

		// Copy data to ensure receiver owns the bytes
		owned := make([]byte, len(data))
		copy(owned, data)

		select {
		case r.messages <- ReceivedMessage{Type: msgType, Data: owned}:
		case <-r.stop:
			return
		}
	}
}

// Messages returns the channel from which received messages can be read.
func (r *AsyncReceiver) Messages() <-chan ReceivedMessage {
	return r.messages
}

// Drain reads and returns all currently buffered messages without blocking.
// This is useful for frame-locked processing: drain all buffered messages
// at the start of each frame, process them, then let the goroutine continue
// buffering for the next frame.
func (r *AsyncReceiver) Drain() []ReceivedMessage {
	var msgs []ReceivedMessage
	for {
		select {
		case msg, ok := <-r.messages:
			if !ok {
				return msgs
			}
			msgs = append(msgs, msg)
		default:
			return msgs
		}
	}
}

// Stop shuts down the receive goroutine. Safe to call multiple times.
func (r *AsyncReceiver) Stop() {
	r.once.Do(func() {
		close(r.stop)
		<-r.done
	})
}
