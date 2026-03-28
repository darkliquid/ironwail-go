// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"fmt"
	"log/slog"

	"github.com/ironwail/ironwail-go/internal/console"
)

var (
	newSDL3AudioBackend  = NewSDL3AudioBackend
	newOtoBackend        = NewOtoBackend
	newMiniaudioBackend  = NewMiniaudioBackend
	audioInitSampleRates = []int{44100, 48000}
)

// AudioAdapter wraps audio.System to implement host.Audio interface
type AudioAdapter struct {
	sys              *System
	consoleSoundHash int
}

func NewAudioAdapter(sys *System) *AudioAdapter {
	return &AudioAdapter{sys: sys, consoleSoundHash: 345}
}

func (a *AudioAdapter) Init() error {
	if a.sys == nil {
		return nil
	}
	console.Printf("Sound Initialization\n")

	sdl3 := newSDL3AudioBackend()
	oto := newOtoBackend()
	miniaudio := newMiniaudioBackend()
	slog.Debug("audio backend availability", "sdl3", sdl3 != nil, "oto", oto != nil, "miniaudio", miniaudio != nil)

	candidates := []struct {
		name    string
		backend Backend
	}{
		{name: "SDL3", backend: sdl3},
		{name: "Oto", backend: oto},
		{name: "miniaudio", backend: miniaudio},
		{name: "null", backend: NewNullBackend()},
	}

	var initErr error
	for _, candidate := range candidates {
		if candidate.backend == nil {
			continue
		}
		for index, rate := range audioInitSampleRates {
			err := a.sys.Init(candidate.backend, rate, false)
			if err == nil {
				slog.Info("audio initialized", "backend", candidate.name, "sample_rate", rate)
				initErr = nil
				goto startup
			}
			initErr = err
			if index+1 < len(audioInitSampleRates) {
				slog.Warn("failed to init audio backend, retrying alternate sample rate", "backend", candidate.name, "sample_rate", rate, "error", err)
				continue
			}
			slog.Warn("failed to init audio backend", "backend", candidate.name, "sample_rate", rate, "error", err)
		}
	}
	if initErr != nil {
		return initErr
	}

startup:
	if err := a.sys.Startup(); err != nil {
		return err
	}
	if a.sys.dma != nil {
		console.Printf("Audio: %d bit, stereo, %d Hz\n\n", a.sys.dma.SampleBits, a.sys.dma.Speed)
	}
	return nil
}

func (a *AudioAdapter) Update(origin, velocity, forward, right, up [3]float32) {
	if a.sys != nil {
		a.sys.Update(origin, velocity, forward, right, up)
	}
}

func (a *AudioAdapter) StopAllSounds(clear bool) {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.StopAllSounds(clear)
}

func (a *AudioAdapter) SoundInfo() string {
	if a == nil || a.sys == nil {
		return "sound system not available\n"
	}
	return a.sys.SoundInfo()
}

func (a *AudioAdapter) SoundList() string {
	if a == nil || a.sys == nil {
		return "0 sounds, 0 bytes\n"
	}
	return a.sys.SoundList()
}

func (a *AudioAdapter) Shutdown() {
	if a.sys != nil {
		a.sys.Shutdown()
	}
}

func (a *AudioAdapter) PrecacheSound(name string, loader func() ([]byte, error)) *SFX {
	if a == nil || a.sys == nil {
		return nil
	}
	return a.sys.PrecacheSound(name, loader)
}

func (a *AudioAdapter) StartSound(entNum, entChannel int, sfx *SFX, origin, velocity [3]float32, vol, attenuation float32) {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.StartSound(entNum, entChannel, sfx, origin, velocity, vol, attenuation)
}

func (a *AudioAdapter) StartStaticSound(sfx *SFX, origin, velocity [3]float32, vol, attenuation float32) {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.StartStaticSound(sfx, origin, velocity, vol, attenuation)
}

func (a *AudioAdapter) ClearStaticSounds() {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.ClearStaticSounds()
}

func (a *AudioAdapter) SetListener(origin, velocity, forward, right, up [3]float32) {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.SetListener(origin, velocity, forward, right, up)
}

func (a *AudioAdapter) SetViewEntity(viewEntity int) {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.SetViewEntity(viewEntity)
}

func (a *AudioAdapter) StopSound(entNum, entChannel int) {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.StopSound(entNum, entChannel)
}

func (a *AudioAdapter) PlayCDTrack(track, loopTrack int, loader func(string) ([]byte, error), resolvers ...musicResolveFunc) error {
	if a == nil || a.sys == nil {
		return nil
	}
	return a.sys.PlayCDTrack(track, loopTrack, loader, resolvers...)
}

func (a *AudioAdapter) StopMusic() {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.StopMusic()
}

func (a *AudioAdapter) PauseMusic() {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.PauseMusic()
}

func (a *AudioAdapter) ResumeMusic() {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.ResumeMusic()
}

func (a *AudioAdapter) SetMusicLoop(loop bool) {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.SetMusicLoop(loop)
}

func (a *AudioAdapter) ToggleMusicLoop() bool {
	if a == nil || a.sys == nil {
		return false
	}
	return a.sys.ToggleMusicLoop()
}

func (a *AudioAdapter) MusicLooping() bool {
	if a == nil || a.sys == nil {
		return false
	}
	return a.sys.MusicLooping()
}

func (a *AudioAdapter) CurrentMusic() string {
	if a == nil || a.sys == nil {
		return ""
	}
	return a.sys.CurrentMusic()
}

func (a *AudioAdapter) JumpMusic(order int) bool {
	if a == nil || a.sys == nil {
		return false
	}
	return a.sys.JumpMusic(order)
}

func (a *AudioAdapter) PlayLocalSound(name string, loader func() ([]byte, error), vol float32) error {
	if a == nil || a.sys == nil {
		return fmt.Errorf("sound system not available")
	}
	sfx := a.sys.PrecacheSound(name, loader)
	if sfx == nil || sfx.Cache == nil {
		return fmt.Errorf("failed to load sound %q", name)
	}
	a.sys.StartSound(a.consoleSoundHash, 0, sfx, a.sys.listener.Origin, a.sys.listener.Velocity, vol, 1.0)
	a.consoleSoundHash++
	return nil
}

func (a *AudioAdapter) PlayMusic(filename string, loader func(string) ([]byte, error), resolver func([]string) (string, []byte, error)) error {
	if a == nil || a.sys == nil {
		return fmt.Errorf("music system not available")
	}
	return a.sys.PlayMusic(filename, loader, resolver)
}

func (a *AudioAdapter) SetVolume(vol float64) {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.SetVolume(vol)
}

func (a *AudioAdapter) SetAmbientSound(channel int, sfx *SFX) {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.SetAmbientSound(channel, sfx)
}

func (a *AudioAdapter) UpdateAmbientSounds(frameTime float32, hasLeaf bool, ambientLevels [NumAmbients]uint8, underwaterIntensity float32) {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.UpdateAmbientSounds(frameTime, hasLeaf, ambientLevels, underwaterIntensity)
}
