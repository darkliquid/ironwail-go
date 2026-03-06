// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"testing"
)

func TestSpatialize(t *testing.T) {
	sys := &System{}
	sys.dma = &DMAInfo{Channels: 2}
	sys.listener.Origin = [3]float32{0, 0, 0}
	sys.listener.Right = [3]float32{1, 0, 0}
	sys.viewEntity = 1

	ch := &Channel{
		EntNum:    2,
		Origin:    [3]float32{100, 0, 0},
		DistMult:  0.001, // 1.0 / 1000.0
		MasterVol: 255,
	}

	// Sound to the right
	sys.spatialize(ch)
	if ch.RightVol <= ch.LeftVol {
		t.Errorf("Expected RightVol > LeftVol for sound on the right, got R:%d L:%d", ch.RightVol, ch.LeftVol)
	}

	// Sound to the left
	ch.Origin = [3]float32{-100, 0, 0}
	sys.spatialize(ch)
	if ch.LeftVol <= ch.RightVol {
		t.Errorf("Expected LeftVol > RightVol for sound on the left, got R:%d L:%d", ch.RightVol, ch.LeftVol)
	}

	// Sound at listener (view entity)
	ch.EntNum = 1
	sys.spatialize(ch)
	if ch.LeftVol != 255 || ch.RightVol != 255 {
		t.Errorf("Expected full volume for view entity, got R:%d L:%d", ch.RightVol, ch.LeftVol)
	}
}

func TestMixing(t *testing.T) {
	mixer := NewMixer()
	mixer.SetVolume(1.0)
	mixer.SetSndSpeed(44100)

	// Create a simple 16-bit mono sound
	data := make([]byte, 200)
	for i := 0; i < 100; i++ {
		data[i*2] = 0xFF
		data[i*2+1] = 0x7F // 32767
	}
	cache := &SoundCache{
		Length:    100,
		Width:     2,
		Data:      data,
		LoopStart: -1,
	}
	sfx := &SFX{Cache: cache}

	channels := []Channel{
		{
			SFX:       sfx,
			LeftVol:   255,
			RightVol:  255,
			End:       100,
			Pos:       0,
			MasterVol: 255,
		},
	}

	dma := &DMAInfo{
		Channels:   2,
		Samples:    2048,
		SampleBits: 16,
		Speed:      44100,
		Buffer:     make([]byte, 2048*2*2),
	}

	rawSamples := &RawSamplesBuffer{}

	mixer.PaintChannels(channels, rawSamples, dma, 0, 100)

	// Check if something was mixed into the DMA buffer
	found := false
	for _, b := range dma.Buffer {
		if b != 0 {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("Expected non-zero DMA buffer after mixing")
	}
}

func TestLooping(t *testing.T) {
	mixer := NewMixer()
	mixer.SetVolume(1.0)

	data := make([]byte, 100)
	cache := &SoundCache{
		Length:    100,
		Width:     1,
		Data:      data,
		LoopStart: 0,
	}
	sfx := &SFX{Cache: cache}

	channels := []Channel{
		{
			SFX:       sfx,
			LeftVol:   255,
			RightVol:  255,
			End:       100,
			Pos:       0,
			MasterVol: 255,
		},
	}

	dma := &DMAInfo{
		Channels:   2,
		Samples:    2048,
		SampleBits: 16,
		Speed:      44100,
		Buffer:     make([]byte, 2048*2*2),
	}

	rawSamples := &RawSamplesBuffer{}

	// Paint 150 samples, should loop
	mixer.PaintChannels(channels, rawSamples, dma, 0, 150)

	if channels[0].SFX == nil {
		t.Errorf("Expected channel to still be active (looping)")
	}
	if channels[0].Pos != 50 {
		t.Errorf("Expected channel position to be 50 after loop, got %d", channels[0].Pos)
	}
}

func TestTransferPaintBuffer(t *testing.T) {
	mixer := NewMixer()
	dma := &DMAInfo{
		Channels:   2,
		Samples:    2048,
		SampleBits: 16,
		Buffer:     make([]byte, 2048*2*2),
	}

	mixer.paintBuffer[0].Left = 32767 * 256
	mixer.paintBuffer[0].Right = -32768 * 256

	mixer.transferPaintBuffer(dma, 0, 1)

	if dma.Buffer[0] != 0xFF || dma.Buffer[1] != 0x7F {
		t.Errorf("Expected 0x7FFF for left channel, got %02X%02X", dma.Buffer[1], dma.Buffer[0])
	}
	if dma.Buffer[2] != 0x00 || dma.Buffer[3] != 0x80 {
		t.Errorf("Expected 0x8000 for right channel, got %02X%02X", dma.Buffer[3], dma.Buffer[2])
	}
}
