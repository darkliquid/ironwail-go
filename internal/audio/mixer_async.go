// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import "sync"

// AsyncMixer wraps a MixerPipeline and runs mixing in a dedicated goroutine.
// The game thread sends mix requests via a channel; the mixer goroutine
// processes them and signals completion. This keeps audio mixing off the
// main thread, reducing frame-time jitter from CPU-intensive mixing.
//
// Usage:
// base := NewMixer()
// async := NewAsyncMixer(base)
// defer async.Stop()
// // async implements MixerPipeline — use it wherever Mixer was used
//
// Note: PaintChannels is still synchronous from the caller perspective; it
// waits for completion before returning the updated painted time.
type AsyncMixer struct {
	inner    MixerPipeline
	requests chan mixRequest
	done     chan struct{}
	once     sync.Once
}

type mixRequest struct {
	channels    []Channel
	rawSamples  *RawSamplesBuffer
	dma         *DMAInfo
	paintedTime int
	endTime     int
	result      chan<- int
}

func NewAsyncMixer(inner MixerPipeline) *AsyncMixer {
	a := &AsyncMixer{
		inner:    inner,
		requests: make(chan mixRequest, 1),
		done:     make(chan struct{}),
	}
	go a.run()
	return a
}

func (a *AsyncMixer) run() {
	defer close(a.done)
	for req := range a.requests {
		result := a.inner.PaintChannels(req.channels, req.rawSamples, req.dma, req.paintedTime, req.endTime)
		req.result <- result
	}
}

func (a *AsyncMixer) PaintChannels(channels []Channel, rawSamples *RawSamplesBuffer, dma *DMAInfo, paintedTime, endTime int) int {
	result := make(chan int, 1)
	a.requests <- mixRequest{
		channels:    channels,
		rawSamples:  rawSamples,
		dma:         dma,
		paintedTime: paintedTime,
		endTime:     endTime,
		result:      result,
	}
	return <-result
}

func (a *AsyncMixer) SetSndSpeed(speed int) { a.inner.SetSndSpeed(speed) }

func (a *AsyncMixer) SndSpeed() int { return a.inner.SndSpeed() }

// Stop shuts down the mixer goroutine. Safe to call multiple times.
func (a *AsyncMixer) Stop() {
	a.once.Do(func() {
		close(a.requests)
		<-a.done
	})
}

var _ MixerPipeline = (*AsyncMixer)(nil)
