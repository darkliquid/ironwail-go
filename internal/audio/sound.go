// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"fmt"
	"strings"
)

const (
	ambientVolumeScale = 0.3
	ambientFadeRate    = 100.0
)

type System struct {
	channels    [MaxChannels]Channel
	totalChans  int
	initialized bool
	started     bool
	blocked     int

	dma        *DMAInfo
	cache      *SFXCache
	mixer      MixerPipeline
	rawSamples RawSamplesBuffer
	backend    Backend
	music      *musicState
	musicLoop  bool

	listener    ListenerState
	viewEntity  int
	soundTime   int
	paintedTime int
	mixAhead    float64

	// For tracking DMA buffer wraps to keep soundTime monotonic
	// (mirrors C Quake's GetSoundtime logic)
	oldSamplePos int
	bufferCount  int

	ambientSFX    [NumAmbients]*SFX
	ambientLevels [NumAmbients]float32
}

func NewSystem() *System {
	return &System{
		mixAhead:  0.1,
		musicLoop: true,
	}
}

func (s *System) Init(backend Backend, sampleRate int, load8Bit bool) error {
	if s.initialized {
		return fmt.Errorf("sound already initialized")
	}

	s.backend = backend

	var err error
	s.dma, err = backend.Init(sampleRate, 16, 2, 4096)
	if err != nil {
		return fmt.Errorf("failed to init audio backend: %w", err)
	}

	s.cache = NewSFXCache(s.dma.Speed, load8Bit)
	s.mixer = NewMixer()
	s.mixer.SetSndSpeed(s.dma.Speed)
	s.initialized = true

	return nil
}

func (s *System) Shutdown() {
	if !s.initialized {
		return
	}

	s.SetVolume(0)
	s.Block()
	s.StopMusic()
	s.StopAllSounds(true)

	if s.backend != nil {
		s.backend.Shutdown()
	}

	s.blocked = 0
	s.started = false
	s.initialized = false
}

func (s *System) Startup() error {
	if !s.initialized {
		return nil
	}

	s.started = true
	s.StopAllSounds(true)
	return nil
}

func (s *System) PrecacheSound(name string, loader func() ([]byte, error)) *SFX {
	if !s.initialized || s.cache == nil {
		return nil
	}

	sfx := s.cache.FindName(name)
	if sfx == nil {
		return nil
	}

	if sfx.Cached {
		return sfx
	}

	data, err := loader()
	if err != nil {
		return nil
	}

	s.cache.Load(sfx, data)
	return sfx
}

func (s *System) StartSound(entNum, entChannel int, sfx *SFX, origin, velocity [3]float32, vol, attenuation float32) {
	if !s.started || sfx == nil || sfx.Cache == nil {
		return
	}

	targetChan := s.pickChannel(entNum, entChannel)
	if targetChan == nil {
		return
	}

	targetChan.SFX = sfx
	targetChan.Origin = origin
	targetChan.Velocity = velocity
	targetChan.DistMult = attenuation / SoundNominalClipDist
	targetChan.MasterVol = int(vol * 255)
	targetChan.EntNum = entNum
	targetChan.EntChannel = entChannel
	targetChan.Pos = 0
	targetChan.PosFraction = 0
	targetChan.Pitch = 1.0

	s.spatialize(targetChan)

	if targetChan.LeftVol == 0 && targetChan.RightVol == 0 {
		targetChan.SFX = nil
		return
	}

	targetChan.End = s.paintedTime + sfx.Cache.Length
}

func (s *System) StopSound(entNum, entChannel int) {
	for i := NumAmbients; i < NumAmbients+MaxDynamicChannels; i++ {
		if s.channels[i].EntNum == entNum && s.channels[i].EntChannel == entChannel {
			s.channels[i].End = 0
			s.channels[i].SFX = nil
			return
		}
	}
}

func (s *System) StartStaticSound(sfx *SFX, origin, velocity [3]float32, vol, attenuation float32) {
	if !s.started || sfx == nil || sfx.Cache == nil || sfx.Cache.LoopStart < 0 {
		return
	}

	staticBase := NumAmbients + MaxDynamicChannels
	if s.totalChans < staticBase {
		s.totalChans = staticBase
	}

	chanIndex := -1
	for i := staticBase; i < s.totalChans; i++ {
		if s.channels[i].SFX == nil {
			chanIndex = i
			break
		}
	}
	if chanIndex == -1 {
		if s.totalChans >= MaxChannels {
			return
		}
		chanIndex = s.totalChans
		s.totalChans++
	}

	ch := &s.channels[chanIndex]
	*ch = Channel{
		SFX:         sfx,
		Origin:      origin,
		Velocity:    velocity,
		DistMult:    attenuation / SoundNominalClipDist,
		MasterVol:   int(vol * 255),
		End:         s.paintedTime + sfx.Cache.Length,
		Pitch:       1.0,
		PosFraction: 0,
	}
	s.spatialize(ch)
}

func (s *System) ClearStaticSounds() {
	staticBase := NumAmbients + MaxDynamicChannels
	if s.totalChans <= staticBase {
		return
	}
	for i := staticBase; i < s.totalChans; i++ {
		s.channels[i] = Channel{}
	}
	s.totalChans = staticBase
}

func (s *System) StopAllSounds(clear bool) {
	if s.backend != nil {
		s.backend.Lock()
		defer s.backend.Unlock()
	} else if clear && s.dma != nil {
		s.dma.mu.Lock()
		defer s.dma.mu.Unlock()
	}

	s.totalChans = NumAmbients + MaxDynamicChannels

	for i := 0; i < MaxChannels; i++ {
		s.channels[i] = Channel{}
	}

	if clear && s.dma != nil {
		for i := range s.dma.Buffer {
			s.dma.Buffer[i] = 0
		}
	}
}

func (s *System) SetListener(origin, velocity, forward, right, up [3]float32) {
	if !s.started {
		return
	}

	s.listener.Origin = origin
	s.listener.Velocity = velocity
	s.listener.Forward = forward
	s.listener.Right = right
	s.listener.Up = up

	for i := NumAmbients; i < s.totalChans; i++ {
		ch := &s.channels[i]
		if ch.SFX == nil {
			continue
		}
		s.spatialize(ch)
	}
}

func (s *System) SetViewEntity(viewEntity int) {
	if !s.started {
		return
	}
	if s.viewEntity == viewEntity {
		return
	}
	s.viewEntity = viewEntity

	for i := NumAmbients; i < s.totalChans; i++ {
		ch := &s.channels[i]
		if ch.SFX == nil {
			continue
		}
		s.spatialize(ch)
	}
}

func (s *System) ViewEntity() int {
	return s.viewEntity
}

func (s *System) Update(origin, velocity, forward, right, up [3]float32) {
	if !s.started || s.blocked > 0 {
		return
	}
	s.SetListener(origin, velocity, forward, right, up)
	s.combineStaticChannels()

	s.updateSoundTime()

	if s.backend != nil {
		s.backend.Lock()
		defer s.backend.Unlock()
	}

	endTime := s.soundTime + int(s.mixAhead*float64(s.dma.Speed))
	maxTime := s.soundTime + s.dma.Samples/s.dma.Channels
	if endTime > maxTime {
		endTime = maxTime
	}

	s.updateMusic(endTime)
	s.paintedTime = s.mixer.PaintChannels(s.channels[:s.totalChans], &s.rawSamples, s.dma, s.paintedTime, endTime)
}

func (s *System) combineStaticChannels() {
	staticBase := NumAmbients + MaxDynamicChannels
	if s.totalChans <= staticBase {
		return
	}

	var lastCombined *Channel
	for i := staticBase; i < s.totalChans; i++ {
		ch := &s.channels[i]
		if ch.SFX == nil || (ch.LeftVol == 0 && ch.RightVol == 0) {
			continue
		}

		if lastCombined != nil && lastCombined.SFX == ch.SFX {
			lastCombined.LeftVol += ch.LeftVol
			lastCombined.RightVol += ch.RightVol
			ch.LeftVol = 0
			ch.RightVol = 0
			continue
		}

		var combine *Channel
		for j := staticBase; j < i; j++ {
			prev := &s.channels[j]
			if prev.SFX == ch.SFX {
				combine = prev
				break
			}
		}
		if combine != nil && combine != ch {
			combine.LeftVol += ch.LeftVol
			combine.RightVol += ch.RightVol
			ch.LeftVol = 0
			ch.RightVol = 0
			lastCombined = combine
			continue
		}
		lastCombined = ch
	}
}

func (s *System) SetVolume(vol float64) {
	if !s.initialized || s.mixer == nil {
		return
	}
	if vol < 0 {
		vol = 0
	}
	if vol > 1 {
		vol = 1
	}
	if mixer, ok := s.mixer.(interface{ SetVolume(float64) }); ok {
		mixer.SetVolume(vol)
	}
}

func (s *System) Volume() float64 {
	if s.mixer == nil {
		return 0
	}
	if mixer, ok := s.mixer.(interface{ Volume() float64 }); ok {
		return mixer.Volume()
	}
	return 0
}

func (s *System) pickChannel(entNum, entChannel int) *Channel {
	firstToDie := -1
	lifeLeft := 0x7fffffff

	for i := NumAmbients; i < NumAmbients+MaxDynamicChannels; i++ {
		if entChannel != 0 && s.channels[i].EntNum == entNum &&
			(s.channels[i].EntChannel == entChannel || entChannel == -1) {
			firstToDie = i
			break
		}

		if s.channels[i].End-s.paintedTime < lifeLeft {
			lifeLeft = s.channels[i].End - s.paintedTime
			firstToDie = i
		}
	}

	if firstToDie == -1 {
		return nil
	}

	return &s.channels[firstToDie]
}

func (s *System) updateSoundTime() {
	if s.backend == nil || s.dma == nil {
		return
	}

	// Get the raw DMA position (wraps at dma.Samples)
	samplePos := s.backend.GetPosition()
	fullSamples := s.dma.Samples

	// Detect buffer wrap-around: if position went backwards, the buffer wrapped
	if samplePos < s.oldSamplePos {
		s.bufferCount++
	}
	s.oldSamplePos = samplePos

	// Compute monotonically increasing sound time
	s.soundTime = s.bufferCount*fullSamples + samplePos
}

func (s *System) AddRawSamples(samples int, rate, width, channels int, data []byte, volume float32) {
	if s.rawSamples.End < s.paintedTime {
		s.rawSamples.End = s.paintedTime
	}

	scale := float64(rate) / float64(s.dma.Speed)
	intVolume := int(volume * 256)

	for i := 0; ; i++ {
		src := int(float64(i) * scale)
		if src >= samples {
			break
		}
		dst := s.rawSamples.End & (MaxRawSamples - 1)
		s.rawSamples.End++

		if channels == 2 && width == 2 {
			s.rawSamples.Samples[dst].Left = int32(int16(uint16(data[src*4])|uint16(data[src*4+1])<<8)) * int32(intVolume)
			s.rawSamples.Samples[dst].Right = int32(int16(uint16(data[src*4+2])|uint16(data[src*4+3])<<8)) * int32(intVolume)
		} else if channels == 2 && width == 1 {
			s.rawSamples.Samples[dst].Left = (int32(data[src*2]) - 128) * int32(intVolume) << 8
			s.rawSamples.Samples[dst].Right = (int32(data[src*2+1]) - 128) * int32(intVolume) << 8
		} else if channels == 1 && width == 2 {
			sample := int32(int16(uint16(data[src*2])|uint16(data[src*2+1])<<8)) * int32(intVolume)
			s.rawSamples.Samples[dst].Left = sample
			s.rawSamples.Samples[dst].Right = sample
		} else if channels == 1 && width == 1 {
			sample := (int32(data[src]) - 128) * int32(intVolume) << 8
			s.rawSamples.Samples[dst].Left = sample
			s.rawSamples.Samples[dst].Right = sample
		}
	}
}

func (s *System) Block() {
	if s.started && s.blocked == 0 {
		s.blocked = 1
		if s.backend != nil {
			s.backend.Block()
		}
	}
}

func (s *System) Unblock() {
	if !s.started || s.blocked == 0 {
		return
	}
	s.blocked = 0
	if s.backend != nil {
		s.backend.Unblock()
	}
}

func (s *System) SetUnderwaterIntensity(intensity float32) {
	if mixer, ok := s.mixer.(interface{ SetUnderwaterIntensity(float32) }); ok {
		mixer.SetUnderwaterIntensity(intensity)
	}
}
func (s *System) SetAmbientSound(channel int, sfx *SFX) {
	if channel < 0 || channel >= NumAmbients {
		return
	}
	s.ambientSFX[channel] = sfx
}

func (s *System) SoundInfo() string {
	if !s.initialized {
		return "sound system not initialized\n"
	}

	active := 0
	for i := 0; i < s.totalChans; i++ {
		if s.channels[i].SFX != nil {
			active++
		}
	}

	cached := 0
	memory := 0
	if s.cache != nil {
		cached = s.cache.numSounds
		for i := 0; i < s.cache.numSounds; i++ {
			if sfx := s.cache.sounds[i]; sfx != nil && sfx.Cache != nil {
				memory += len(sfx.Cache.Data)
			}
		}
	}

	return fmt.Sprintf("%d active channels\n%d precached sounds\n%.1f MB sound memory\n",
		active, cached, float32(memory)/(1024*1024))
}

func (s *System) SoundList() string {
	if !s.initialized || s.cache == nil {
		return "0 sounds, 0 bytes\n"
	}

	var builder strings.Builder
	total := 0
	for i := 0; i < s.cache.numSounds; i++ {
		sfx := s.cache.sounds[i]
		if sfx == nil || sfx.Cache == nil {
			continue
		}
		size := sfx.Cache.Length * sfx.Cache.Width * (sfx.Cache.Stereo + 1)
		total += size
		if sfx.Cache.LoopStart >= 0 {
			builder.WriteString("L")
		} else {
			builder.WriteString(" ")
		}
		builder.WriteString(fmt.Sprintf("(%2db) %6d : %s\n", sfx.Cache.Width*8, size, sfx.Name))
	}
	builder.WriteString(fmt.Sprintf("%d sounds, %d bytes\n", s.cache.numSounds, total))
	return builder.String()
}

func (s *System) UpdateAmbientSounds(frameTime float32, hasLeaf bool, ambientLevels [NumAmbients]uint8, underwaterIntensity float32) {
	if s.mixer == nil {
		return
	}

	if mixer, ok := s.mixer.(interface{ SetUnderwaterIntensity(float32) }); ok {
		mixer.SetUnderwaterIntensity(underwaterIntensity)
	}

	if !hasLeaf {
		for i := 0; i < NumAmbients; i++ {
			s.channels[i] = Channel{}
			s.ambientLevels[i] = 0
		}
		return
	}

	fadeStep := frameTime * ambientFadeRate
	if fadeStep < 0 {
		fadeStep = 0
	}

	for i := 0; i < NumAmbients; i++ {
		ch := &s.channels[i]
		sfx := s.ambientSFX[i]
		if ch.SFX != sfx {
			ch.SFX = sfx
			ch.Pos = 0
			ch.End = s.paintedTime
		}

		target := float32(ambientLevels[i]) * ambientVolumeScale
		if target < 8 {
			target = 0
		} else if target > 255 {
			target = 255
		}

		level := s.ambientLevels[i]
		if level < target {
			level += fadeStep
			if level > target {
				level = target
			}
		} else if level > target {
			level -= fadeStep
			if level < target {
				level = target
			}
		}
		s.ambientLevels[i] = level

		vol := int(level)
		ch.LeftVol = vol
		ch.RightVol = vol
		ch.MasterVol = vol
	}
}

func (s *System) UnderwaterIntensity() float32 {
	if s == nil || s.mixer == nil {
		return 0
	}
	if mixer, ok := s.mixer.(interface{ UnderwaterIntensity() float32 }); ok {
		return mixer.UnderwaterIntensity()
	}
	return 0
}

func (s *System) AmbientVolume(channel int) int {
	if s == nil || channel < 0 || channel >= NumAmbients {
		return 0
	}
	return s.channels[channel].MasterVol
}

func (s *System) AmbientSound(channel int) *SFX {
	if s == nil || channel < 0 || channel >= NumAmbients {
		return nil
	}
	return s.channels[channel].SFX
}

func (s *System) IsInitialized() bool { return s.initialized }
func (s *System) IsStarted() bool     { return s.started }
