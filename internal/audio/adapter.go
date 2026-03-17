// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"log/slog"
)

// AudioAdapter wraps audio.System to implement host.Audio interface
type AudioAdapter struct {
	sys *System
}

func NewAudioAdapter(sys *System) *AudioAdapter {
	return &AudioAdapter{sys: sys}
}

func (a *AudioAdapter) Init() error {
	if a.sys == nil {
		return nil
	}

	sdl3 := NewSDL3AudioBackend()
	oto := NewOtoBackend()
	slog.Info("audio backend availability", "sdl3", sdl3 != nil, "oto", oto != nil)

	backend := Backend(NewNullBackend())
	if sdl3 != nil {
		slog.Info("selecting SDL3 audio backend")
		backend = sdl3
	} else if oto != nil {
		slog.Info("selecting Oto audio backend")
		backend = oto
	} else {
		slog.Warn("no hardware audio backends available, using null backend")
	}

	if err := a.sys.Init(backend, 44100, false); err != nil {
		slog.Warn("failed to init audio at 44.1kHz, retrying at 48kHz", "error", err)
		// Fallback to 48kHz which is common on modern Linux/Pipewire
		if err2 := a.sys.Init(backend, 48000, false); err2 != nil {
			slog.Error("failed to init audio at 48kHz, using null backend", "error", err2)
			fallback := Backend(NewNullBackend())
			if err3 := a.sys.Init(fallback, 44100, false); err3 != nil {
				return err
			}
		} else {
			slog.Info("audio initialized at 48kHz")
		}
	} else {
		slog.Info("audio initialized at 44.1kHz")
	}

	return a.sys.Startup()
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
	if a == nil || a.sys != nil {
		a.sys.SetListener(origin, velocity, forward, right, up)
	}
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
