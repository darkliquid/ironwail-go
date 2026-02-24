// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

type NullBackend struct {
	sampleRate int
	sampleBits int
	channels   int
	bufferSize int
	dma        *DMAInfo
	pos        int
}

func NewNullBackend() *NullBackend {
	return &NullBackend{}
}

func (b *NullBackend) Init(sampleRate, sampleBits, channels, bufferSize int) (*DMAInfo, error) {
	b.sampleRate = sampleRate
	b.sampleBits = sampleBits
	b.channels = channels
	b.bufferSize = bufferSize

	dma := &DMAInfo{
		Channels:        channels,
		Samples:         bufferSize,
		SubmissionChunk: 1,
		SamplePos:       0,
		SampleBits:      sampleBits,
		Speed:           sampleRate,
		Buffer:          make([]byte, bufferSize*channels*(sampleBits/8)),
	}

	b.dma = dma
	return dma, nil
}

func (b *NullBackend) Shutdown() {}

func (b *NullBackend) Lock() {}

func (b *NullBackend) Unlock() {}

func (b *NullBackend) GetPosition() int {
	b.pos += 256
	if b.pos >= b.bufferSize {
		b.pos = 0
	}
	return b.pos
}

func (b *NullBackend) Block() {}

func (b *NullBackend) Unblock() {}
