// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestAsyncMixer_DelegatesToInner tests the asynchronous mixer wrapper.
// It offloading audio mixing to a separate thread/goroutine to improve engine performance.
// Where in C: N/A (Go-specific optimization)
func TestAsyncMixer_DelegatesToInner(t *testing.T) {
	inner := &NullMixer{}
	inner.SetSndSpeed(44100)
	async := NewAsyncMixer(inner)
	defer async.Stop()

	result := async.PaintChannels(nil, nil, nil, 0, 100)
	if result != 100 {
		t.Errorf("expected 100, got %d", result)
	}
	if async.SndSpeed() != 44100 {
		t.Errorf("expected 44100, got %d", async.SndSpeed())
	}
}

// TestAsyncMixer_StopIdempotent tests safe shutdown of the async mixer.
// It preventing crashes or hangs when stopping the audio system.
// Where in C: N/A
func TestAsyncMixer_StopIdempotent(t *testing.T) {
	async := NewAsyncMixer(&NullMixer{})
	async.Stop()
	async.Stop() // should not panic
}

// TestAsyncMixer_MultipleRequests tests sequential mixing requests in the async mixer.
// It ensuring the async worker correctly processes a stream of mixing tasks.
// Where in C: N/A
func TestAsyncMixer_MultipleRequests(t *testing.T) {
	async := NewAsyncMixer(&NullMixer{})
	defer async.Stop()

	for i := 0; i < 10; i++ {
		result := async.PaintChannels(nil, nil, nil, 0, i*100)
		if result != i*100 {
			t.Errorf("request %d: expected %d, got %d", i, i*100, result)
		}
	}
}

// TestAsyncMixer_RunsInDifferentGoroutine tests that the async mixer actually runs in a separate goroutine.
// It verifying the performance isolation of the audio system.
// Where in C: N/A
func TestAsyncMixer_RunsInDifferentGoroutine(t *testing.T) {
	mainGID := currentGID()
	inner := &trackingMixer{started: make(chan struct{}, 1), release: make(chan struct{})}
	async := NewAsyncMixer(inner)
	defer async.Stop()

	done := make(chan int, 1)
	go func() {
		done <- async.PaintChannels(nil, nil, nil, 10, 25)
	}()

	select {
	case <-inner.started:
	case <-time.After(1 * time.Second):
		t.Fatal("inner mixer did not start")
	}

	if inner.gid == 0 {
		t.Fatal("inner mixer goroutine id was not captured")
	}
	if inner.gid == mainGID {
		t.Fatalf("expected mixer to run in different goroutine; both were %d", mainGID)
	}

	close(inner.release)

	select {
	case result := <-done:
		if result != 25 {
			t.Fatalf("expected result 25, got %d", result)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("async paint did not complete")
	}
}

type trackingMixer struct {
	started chan struct{}
	release chan struct{}
	gid     int64
}

func (m *trackingMixer) PaintChannels(_ []Channel, _ *RawSamplesBuffer, _ *DMAInfo, _, endTime int) int {
	m.gid = currentGID()
	m.started <- struct{}{}
	<-m.release
	return endTime
}

func (m *trackingMixer) SetSndSpeed(_ int) {}

func (m *trackingMixer) SndSpeed() int { return 0 }

func currentGID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	line := strings.TrimPrefix(string(buf[:n]), "goroutine ")
	idField := strings.Fields(line)
	if len(idField) == 0 {
		return 0
	}
	id, _ := strconv.ParseInt(idField[0], 10, 64)
	return id
}
