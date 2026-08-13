// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package server

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
)

// RecordedFrameMessage captures a sequence of server messages sent during a frame.
type RecordedFrameMessage struct {
	Frame   int    `json:"frame"`
	Client  int    `json:"client"`
	Bytes   []byte `json:"bytes"`
	ByteHex string `json:"hex"`
}

// MessageStreamRecorder captures all outgoing datagrams and reliable streams for parity checking.
type MessageStreamRecorder struct {
	mu      sync.Mutex
	enabled bool
	frames  []RecordedFrameMessage
}

// NewMessageStreamRecorder creates a new message recorder.
func NewMessageStreamRecorder() *MessageStreamRecorder {
	return &MessageStreamRecorder{
		frames: make([]RecordedFrameMessage, 0),
	}
}

// SetEnabled enables or disables message recording.
func (r *MessageStreamRecorder) SetEnabled(enabled bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.enabled = enabled
}

// IsEnabled returns whether recording is active.
func (r *MessageStreamRecorder) IsEnabled() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.enabled
}

// Record captures a byte slice sent to a client at a given frame.
func (r *MessageStreamRecorder) Record(frame int, client int, data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.enabled || len(data) == 0 {
		return
	}
	cp := make([]byte, len(data))
	copy(cp, data)
	r.frames = append(r.frames, RecordedFrameMessage{
		Frame:   frame,
		Client:  client,
		Bytes:   cp,
		ByteHex: hex.EncodeToString(cp),
	})
}

// Frames returns a copy of all recorded frame messages.
func (r *MessageStreamRecorder) Frames() []RecordedFrameMessage {
	r.mu.Lock()
	defer r.mu.Unlock()
	res := make([]RecordedFrameMessage, len(r.frames))
	copy(res, r.frames)
	return res
}

// Clear resets the recorded frames.
func (r *MessageStreamRecorder) Clear() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.frames = r.frames[:0]
}

// Hash returns a SHA-256 hash of the entire recorded message stream.
func (r *MessageStreamRecorder) Hash() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	h := sha256.New()
	for _, f := range r.frames {
		h.Write(f.Bytes)
	}
	return hex.EncodeToString(h.Sum(nil))
}

// DiffStreams returns the index of the first differing byte between two message streams, or -1 if identical.
func DiffStreams(a, b []RecordedFrameMessage) (int, string) {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if !bytes.Equal(a[i].Bytes, b[i].Bytes) {
			return i, fmt.Sprintf("frame %d: stream data mismatch", a[i].Frame)
		}
	}
	if len(a) != len(b) {
		return minLen, fmt.Sprintf("stream count mismatch: %d vs %d", len(a), len(b))
	}
	return -1, ""
}
