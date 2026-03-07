// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"slices"
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

func TestStartStaticSoundUsesStaticChannelsAndRequiresLoopingCache(t *testing.T) {
	sys := NewSystem()
	sys.started = true
	sys.totalChans = NumAmbients + MaxDynamicChannels
	sys.dma = &DMAInfo{Channels: 2}
	sys.listener.Right = [3]float32{1, 0, 0}

	base := NumAmbients + MaxDynamicChannels
	looped := &SFX{
		Cache: &SoundCache{
			Length:    16,
			LoopStart: 0,
			Width:     1,
			Data:      make([]byte, 16),
		},
	}
	nonLooped := &SFX{
		Cache: &SoundCache{
			Length:    16,
			LoopStart: -1,
			Width:     1,
			Data:      make([]byte, 16),
		},
	}

	sys.StartStaticSound(nonLooped, [3]float32{0, 0, 0}, 1, 1)
	if got := sys.totalChans; got != base {
		t.Fatalf("non-looped static sound allocated channel: totalChans = %d, want %d", got, base)
	}

	sys.StartStaticSound(looped, [3]float32{64, 0, 0}, 1, 1)
	if got := sys.totalChans; got != base+1 {
		t.Fatalf("looped static sound did not allocate static channel: totalChans = %d, want %d", got, base+1)
	}
	if got := sys.channels[base].SFX; got != looped {
		t.Fatalf("static channel SFX = %v, want %v", got, looped)
	}

	sys.StartStaticSound(looped, [3]float32{64, 0, 0}, 1, 9999)
	if got := sys.totalChans; got != base+2 {
		t.Fatalf("inaudible static sound should still persist in static range: totalChans = %d, want %d", got, base+2)
	}
	if got := sys.channels[base+1].SFX; got != looped {
		t.Fatalf("inaudible static channel SFX = %v, want %v", got, looped)
	}
}

func TestClearStaticSoundsLeavesDynamicChannelsIntact(t *testing.T) {
	sys := NewSystem()
	base := NumAmbients + MaxDynamicChannels
	sys.totalChans = base + 2

	dynSFX := &SFX{
		Cache: &SoundCache{
			Length: 4,
			Width:  1,
			Data:   make([]byte, 4),
		},
	}
	staticSFX := &SFX{
		Cache: &SoundCache{
			Length:    4,
			LoopStart: 0,
			Width:     1,
			Data:      make([]byte, 4),
		},
	}

	sys.channels[NumAmbients].SFX = dynSFX
	sys.channels[base].SFX = staticSFX
	sys.channels[base+1].SFX = staticSFX

	sys.ClearStaticSounds()

	if got := sys.totalChans; got != base {
		t.Fatalf("totalChans = %d, want %d after clearing static channels", got, base)
	}
	if got := sys.channels[NumAmbients].SFX; got != dynSFX {
		t.Fatalf("dynamic channel was modified: got %v, want %v", got, dynSFX)
	}
	if sys.channels[base].SFX != nil || sys.channels[base+1].SFX != nil {
		t.Fatalf("static channels not cleared")
	}
}

type lockOrderBackend struct {
	t      *testing.T
	locked bool
	events []string
}

func (b *lockOrderBackend) Init(sampleRate, sampleBits, channels, bufferSize int) (*DMAInfo, error) {
	return nil, nil
}

func (b *lockOrderBackend) Shutdown() {}

func (b *lockOrderBackend) Lock() {
	b.events = append(b.events, "lock")
	b.locked = true
}

func (b *lockOrderBackend) Unlock() {
	b.events = append(b.events, "unlock")
	b.locked = false
}

func (b *lockOrderBackend) GetPosition() int {
	b.events = append(b.events, "getpos")
	if b.locked {
		b.t.Fatalf("GetPosition called while backend lock is held")
	}
	return 128
}

func (b *lockOrderBackend) Block()   {}
func (b *lockOrderBackend) Unblock() {}

func TestUpdateDoesNotCallGetPositionWhileLocked(t *testing.T) {
	backend := &lockOrderBackend{t: t}
	sys := NewSystem()
	sys.started = true
	sys.backend = backend
	sys.dma = &DMAInfo{
		Channels:   2,
		Samples:    4096,
		SampleBits: 16,
		Speed:      44100,
		Buffer:     make([]byte, 4096*2*2),
	}
	sys.mixer = NewMixer()

	sys.Update([3]float32{}, [3]float32{}, [3]float32{}, [3]float32{})

	expectedEvents := []string{"getpos", "lock", "unlock"}
	if !slices.Equal(backend.events, expectedEvents) {
		t.Fatalf("backend call order = %v, want %v", backend.events, expectedEvents)
	}
	if got := sys.soundTime; got != 128 {
		t.Fatalf("soundTime = %d, want 128", got)
	}
}
