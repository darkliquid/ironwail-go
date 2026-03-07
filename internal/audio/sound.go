// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import "fmt"

type System struct {
	channels    [MaxChannels]Channel
	totalChans  int
	initialized bool
	started     bool
	blocked     int

	dma        *DMAInfo
	cache      *SFXCache
	mixer      *Mixer
	rawSamples RawSamplesBuffer
	backend    Backend
	music      *musicState

	listener    ListenerState
	viewEntity  int
	soundTime   int
	paintedTime int
	mixAhead    float64
}

func NewSystem() *System {
	return &System{
		mixAhead: 0.1,
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
	s.initialized = true

	return nil
}

func (s *System) Shutdown() {
	if !s.initialized {
		return
	}

	s.StopMusic()

	if s.backend != nil {
		s.backend.Shutdown()
	}

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

func (s *System) StartSound(entNum, entChannel int, sfx *SFX, origin [3]float32, vol, attenuation float32) {
	if !s.started || sfx == nil || sfx.Cache == nil {
		return
	}

	targetChan := s.pickChannel(entNum, entChannel)
	if targetChan == nil {
		return
	}

	targetChan.SFX = sfx
	targetChan.Origin = origin
	targetChan.DistMult = attenuation / SoundNominalClipDist
	targetChan.MasterVol = int(vol * 255)
	targetChan.EntNum = entNum
	targetChan.EntChannel = entChannel
	targetChan.Pos = 0

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

func (s *System) StartStaticSound(sfx *SFX, origin [3]float32, vol, attenuation float32) {
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
		SFX:       sfx,
		Origin:    origin,
		DistMult:  attenuation / SoundNominalClipDist,
		MasterVol: int(vol * 255),
		End:       s.paintedTime + sfx.Cache.Length,
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

func (s *System) SetListener(origin, forward, right, up [3]float32) {
	if !s.started {
		return
	}

	s.listener.Origin = origin
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

func (s *System) Update(origin, forward, right, up [3]float32) {
	if !s.started || s.blocked > 0 {
		return
	}

	if s.listener.Origin != origin || s.listener.Forward != forward || s.listener.Right != right || s.listener.Up != up {
		s.SetListener(origin, forward, right, up)
	}

	if s.backend != nil {
		s.backend.Lock()
	}

	s.updateSoundTime()

	endTime := s.soundTime + int(s.mixAhead*float64(s.dma.Speed))
	maxTime := s.soundTime + s.dma.Samples/s.dma.Channels
	if endTime > maxTime {
		endTime = maxTime
	}

	s.updateMusic(endTime)
	s.paintedTime = s.mixer.PaintChannels(s.channels[:s.totalChans], &s.rawSamples, s.dma, s.paintedTime, endTime)

	if s.backend != nil {
		s.backend.Unlock()
	}
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
	if s.backend == nil {
		return
	}
	s.soundTime = s.backend.GetPosition()
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
	s.mixer.SetUnderwaterIntensity(intensity)
}

func (s *System) IsInitialized() bool { return s.initialized }
func (s *System) IsStarted() bool     { return s.started }
