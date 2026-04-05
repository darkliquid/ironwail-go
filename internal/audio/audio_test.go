// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"slices"
	"testing"
)

// TestSpatialize tests 3D sound spatialization.
// It correctly calculating stereo volumes based on the listener's position and orientation.
// Where in C: SND_Spatialize in snd_dma.c
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

// TestMixing tests the software audio mixer.
// It verifying that multiple sound channels are correctly combined into the final output buffer.
// Where in C: SND_PaintChannels in snd_mix.c
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
			Pitch:     1.0,
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

// TestLooping tests looping sound playback.
// It ensuring ambient sounds and looping effects repeat seamlessly.
// Where in C: SND_PaintChannels in snd_mix.c
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
			Pitch:     1.0,
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
	if channels[0].Pos != 51 {
		t.Errorf("Expected channel position to be 51 after loop, got %d", channels[0].Pos)
	}
}

// TestTransferPaintBuffer tests the transfer of mixed samples to the DMA buffer.
// It converting internal high-precision samples to the target output format (e.g., 16-bit PCM).
// Where in C: SND_TransferPaintBuffer in snd_mix.c
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

// TestStartStaticSoundUsesStaticChannelsAndRequiresLoopingCache tests static sound allocation.
// It managing ambient map sounds (torches, machinery) that persist regardless of distance.
// Where in C: S_StartStaticSound in snd_dma.c
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

	sys.StartStaticSound(nonLooped, [3]float32{0, 0, 0}, [3]float32{}, 1, 1)
	if got := sys.totalChans; got != base {
		t.Fatalf("non-looped static sound allocated channel: totalChans = %d, want %d", got, base)
	}

	sys.StartStaticSound(looped, [3]float32{64, 0, 0}, [3]float32{}, 1, 1)
	if got := sys.totalChans; got != base+1 {
		t.Fatalf("looped static sound did not allocate static channel: totalChans = %d, want %d", got, base+1)
	}
	if got := sys.channels[base].SFX; got != looped {
		t.Fatalf("static channel SFX = %v, want %v", got, looped)
	}

	sys.StartStaticSound(looped, [3]float32{64, 0, 0}, [3]float32{}, 1, 9999)
	if got := sys.totalChans; got != base+2 {
		t.Fatalf("inaudible static sound should still persist in static range: totalChans = %d, want %d", got, base+2)
	}
	if got := sys.channels[base+1].SFX; got != looped {
		t.Fatalf("inaudible static channel SFX = %v, want %v", got, looped)
	}
}

// TestClearStaticSoundsLeavesDynamicChannelsIntact tests clearing of static sounds.
// It resetting map-based sounds when changing levels without affecting global or UI sounds.
// Where in C: S_ClearStaticSounds in snd_dma.c
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

// TestSetViewEntityRespatializesExistingChannels tests view-entity relative spatialization.
// It ensuring sounds emitted by the player are always at full volume and centered.
// Where in C: S_Update in snd_dma.c
func TestSetViewEntityRespatializesExistingChannels(t *testing.T) {
	sys := NewSystem()
	sys.started = true
	sys.dma = &DMAInfo{Channels: 2}
	sys.totalChans = NumAmbients + MaxDynamicChannels
	sys.listener.Right = [3]float32{1, 0, 0}

	dyn := &sys.channels[NumAmbients]
	*dyn = Channel{
		SFX:       &SFX{Cache: &SoundCache{Length: 8, Width: 1, Data: make([]byte, 8)}},
		EntNum:    5,
		Origin:    [3]float32{128, 0, 0},
		DistMult:  0,
		MasterVol: 200,
	}
	sys.spatialize(dyn)
	if dyn.RightVol <= dyn.LeftVol {
		t.Fatalf("expected panned channel before setting view entity, got R:%d L:%d", dyn.RightVol, dyn.LeftVol)
	}

	sys.SetViewEntity(5)
	if dyn.LeftVol != dyn.MasterVol || dyn.RightVol != dyn.MasterVol {
		t.Fatalf("view-entity channel not forced full volume, got R:%d L:%d want %d", dyn.RightVol, dyn.LeftVol, dyn.MasterVol)
	}

	sys.SetViewEntity(0)
	if dyn.RightVol <= dyn.LeftVol {
		t.Fatalf("expected panning restored after clearing view entity, got R:%d L:%d", dyn.RightVol, dyn.LeftVol)
	}
}

// TestUpdateCombinesIdenticalStaticSounds tests static sound optimization.
// It reducing mixer overhead by combining multiple instances of the same static sound at a location.
// Where in C: S_Update in snd_dma.c
func TestUpdateCombinesIdenticalStaticSounds(t *testing.T) {
	sys := NewSystem()
	sys.started = true
	sys.mixer = NewMixer()
	sys.dma = &DMAInfo{
		Channels:   2,
		Samples:    4096,
		SampleBits: 16,
		Speed:      44100,
		Buffer:     make([]byte, 4096*2*2),
	}

	staticBase := NumAmbients + MaxDynamicChannels
	sys.totalChans = staticBase + 3
	sys.listener.Right = [3]float32{1, 0, 0}

	loop := &SFX{Cache: &SoundCache{Length: 16, LoopStart: 0, Width: 1, Data: make([]byte, 16)}}
	other := &SFX{Cache: &SoundCache{Length: 16, LoopStart: 0, Width: 1, Data: make([]byte, 16)}}

	sys.channels[staticBase] = Channel{SFX: loop, Origin: [3]float32{32, 0, 0}, DistMult: 0, MasterVol: 80}
	sys.channels[staticBase+1] = Channel{SFX: loop, Origin: [3]float32{-32, 0, 0}, DistMult: 0, MasterVol: 50}
	sys.channels[staticBase+2] = Channel{SFX: other, Origin: [3]float32{0, 32, 0}, DistMult: 0, MasterVol: 70}

	expectedA := Channel{Origin: [3]float32{32, 0, 0}, DistMult: 0, MasterVol: 80}
	expectedB := Channel{Origin: [3]float32{-32, 0, 0}, DistMult: 0, MasterVol: 50}
	sys.spatialize(&expectedA)
	sys.spatialize(&expectedB)

	sys.Update([3]float32{}, [3]float32{}, [3]float32{}, [3]float32{1, 0, 0}, [3]float32{})

	combined := sys.channels[staticBase]
	dupe := sys.channels[staticBase+1]
	if combined.LeftVol != expectedA.LeftVol+expectedB.LeftVol || combined.RightVol != expectedA.RightVol+expectedB.RightVol {
		t.Fatalf("combined static channel volumes = R:%d L:%d, want R:%d L:%d",
			combined.RightVol, combined.LeftVol, expectedA.RightVol+expectedB.RightVol, expectedA.LeftVol+expectedB.LeftVol)
	}
	if dupe.LeftVol != 0 || dupe.RightVol != 0 {
		t.Fatalf("duplicate static channel should be muted after combine, got R:%d L:%d", dupe.RightVol, dupe.LeftVol)
	}
	if sys.channels[staticBase+2].LeftVol == 0 && sys.channels[staticBase+2].RightVol == 0 {
		t.Fatalf("different static sound unexpectedly muted")
	}
}

// TestUpdateAmbientSoundsFadesAndAppliesUnderwater tests ambient sound management and underwater effects.
// It correctly fading ambient levels and applying low-pass filters when the player is submerged.
// Where in C: S_UpdateAmbientSounds in snd_dma.c
func TestUpdateAmbientSoundsFadesAndAppliesUnderwater(t *testing.T) {
	sys := NewSystem()
	sys.mixer = NewMixer()
	sys.paintedTime = 100
	water := &SFX{Cache: &SoundCache{Length: 8, LoopStart: 0, Width: 1, Data: make([]byte, 8)}}
	wind := &SFX{Cache: &SoundCache{Length: 8, LoopStart: 0, Width: 1, Data: make([]byte, 8)}}
	sys.SetAmbientSound(0, water)
	sys.SetAmbientSound(1, wind)

	sys.UpdateAmbientSounds(0.1, true, [NumAmbients]uint8{100, 50}, 1)
	if got := sys.channels[0].MasterVol; got != 10 {
		t.Fatalf("ambient water master volume = %d, want 10 after first fade step", got)
	}
	if got := sys.channels[1].MasterVol; got != 10 {
		t.Fatalf("ambient wind master volume = %d, want 10 after first fade step", got)
	}
	if got := sys.channels[0].SFX; got != water {
		t.Fatalf("ambient water channel sfx = %v, want %v", got, water)
	}
	if got := sys.channels[1].SFX; got != wind {
		t.Fatalf("ambient wind channel sfx = %v, want %v", got, wind)
	}
	if got := sys.UnderwaterIntensity(); got <= 0 {
		t.Fatalf("underwater intensity = %v, want > 0", got)
	}
}

// TestUpdateAmbientSoundsClearsWithoutLeaf tests ambient sound clearing in \"empty\" space.
// It ensuring ambient sounds stop when the player is outside the map or in a leaf with no ambient data.
// Where in C: S_UpdateAmbientSounds in snd_dma.c
func TestUpdateAmbientSoundsClearsWithoutLeaf(t *testing.T) {
	sys := NewSystem()
	sys.mixer = NewMixer()
	sys.channels[0] = Channel{SFX: &SFX{}, LeftVol: 12, RightVol: 12, MasterVol: 12}
	sys.channels[1] = Channel{SFX: &SFX{}, LeftVol: 8, RightVol: 8, MasterVol: 8}
	sys.ambientLevels = [NumAmbients]float32{12, 8}

	sys.UpdateAmbientSounds(0.016, false, [NumAmbients]uint8{}, 0)
	if sys.channels[0].SFX != nil || sys.channels[1].SFX != nil {
		t.Fatalf("ambient channels should be cleared without leaf")
	}
	if sys.ambientLevels != [NumAmbients]float32{} {
		t.Fatalf("ambient levels = %v, want zeroed", sys.ambientLevels)
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

// positionBackend is a mock backend that returns a programmable position sequence.
type positionBackend struct {
	positions []int
	index     int
}

func (b *positionBackend) Init(sampleRate, sampleBits, channels, bufferSize int) (*DMAInfo, error) {
	return nil, nil
}
func (b *positionBackend) Shutdown() {}
func (b *positionBackend) Lock()     {}
func (b *positionBackend) Unlock()   {}
func (b *positionBackend) Block()    {}
func (b *positionBackend) Unblock()  {}
func (b *positionBackend) GetPosition() int {
	if b.index >= len(b.positions) {
		return b.positions[len(b.positions)-1]
	}
	pos := b.positions[b.index]
	b.index++
	return pos
}

type shutdownBackend struct {
	events []string
}

func (b *shutdownBackend) Init(sampleRate, sampleBits, channels, bufferSize int) (*DMAInfo, error) {
	return nil, nil
}

func (b *shutdownBackend) Shutdown() {
	b.events = append(b.events, "shutdown")
}

func (b *shutdownBackend) Lock() {
	b.events = append(b.events, "lock")
}

func (b *shutdownBackend) Unlock() {
	b.events = append(b.events, "unlock")
}

func (b *shutdownBackend) GetPosition() int { return 0 }

func (b *shutdownBackend) Block() {
	b.events = append(b.events, "block")
}

func (b *shutdownBackend) Unblock() {
	b.events = append(b.events, "unblock")
}

func TestShutdownMutesAndClearsBeforeBackendTeardown(t *testing.T) {
	backend := &shutdownBackend{}
	sys := NewSystem()
	sys.initialized = true
	sys.started = true
	sys.backend = backend
	sys.dma = &DMAInfo{Buffer: []byte{1, 2, 3, 4}}
	sys.mixer = NewMixer()
	sys.channels[0] = Channel{SFX: &SFX{}}
	sys.channels[NumAmbients] = Channel{SFX: &SFX{}}
	sys.totalChans = NumAmbients + MaxDynamicChannels + 1
	sys.music = &musicState{}
	sys.rawSamples.End = 99
	sys.paintedTime = 33

	sys.Shutdown()

	if got := sys.Volume(); got != 0 {
		t.Fatalf("Volume() after Shutdown = %v, want 0", got)
	}
	if sys.music != nil {
		t.Fatal("Shutdown did not clear active music state")
	}
	if sys.rawSamples.End != sys.paintedTime {
		t.Fatalf("rawSamples.End = %d, want %d", sys.rawSamples.End, sys.paintedTime)
	}
	if sys.channels[0].SFX != nil || sys.channels[NumAmbients].SFX != nil {
		t.Fatal("Shutdown did not clear active sound channels")
	}
	if !slices.Equal(sys.dma.Buffer, []byte{0, 0, 0, 0}) {
		t.Fatalf("DMA buffer after Shutdown = %v, want zeroed buffer", sys.dma.Buffer)
	}
	if !slices.Equal(backend.events, []string{"block", "lock", "unlock", "shutdown"}) {
		t.Fatalf("backend events = %v, want [block lock unlock shutdown]", backend.events)
	}
	if sys.started || sys.initialized || sys.blocked != 0 {
		t.Fatalf("shutdown state = started:%v initialized:%v blocked:%d, want false false 0", sys.started, sys.initialized, sys.blocked)
	}
}

// TestUpdateSoundTimeMonotonicAcrossWraps tests sound timing logic across DMA buffer wraps.
// It preventing audio glitches and sync issues by correctly tracking global sound time.
// Where in C: S_Update in snd_dma.c
func TestUpdateSoundTimeMonotonicAcrossWraps(t *testing.T) {
	// Simulate a DMA buffer of 4096 samples that wraps around.
	// Position sequence: 1024, 2048, 3072, 0 (wrap), 1024, 2048
	backend := &positionBackend{
		positions: []int{1024, 2048, 3072, 0, 1024, 2048},
	}
	sys := NewSystem()
	sys.backend = backend
	sys.dma = &DMAInfo{Samples: 4096}

	var times []int
	for range backend.positions {
		sys.updateSoundTime()
		times = append(times, sys.soundTime)
	}

	// Verify monotonically increasing
	for i := 1; i < len(times); i++ {
		if times[i] <= times[i-1] {
			t.Fatalf("soundTime not monotonic: times[%d]=%d <= times[%d]=%d (full: %v)",
				i, times[i], i-1, times[i-1], times)
		}
	}

	// Verify expected values:
	// Before wrap: 0*4096+1024=1024, 0*4096+2048=2048, 0*4096+3072=3072
	// After wrap:  1*4096+0=4096,    1*4096+1024=5120, 1*4096+2048=6144
	expected := []int{1024, 2048, 3072, 4096, 5120, 6144}
	for i, want := range expected {
		if times[i] != want {
			t.Fatalf("soundTime[%d] = %d, want %d (full: %v)", i, times[i], want, times)
		}
	}
}

// TestUpdateDoesNotCallGetPositionWhileLocked tests thread-safety of the audio system.
// It preventing deadlocks by ensuring backend position queries don't happen while the mixer lock is held.
// Where in C: S_Update in snd_dma.c
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

	sys.Update([3]float32{}, [3]float32{}, [3]float32{}, [3]float32{}, [3]float32{})

	expectedEvents := []string{"getpos", "lock", "unlock"}
	if !slices.Equal(backend.events, expectedEvents) {
		t.Fatalf("backend call order = %v, want %v", backend.events, expectedEvents)
	}
	if got := sys.soundTime; got != 128 {
		t.Fatalf("soundTime = %d, want 128", got)
	}
}
