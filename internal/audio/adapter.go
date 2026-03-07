// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

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

	backend := Backend(NewNullBackend())
	if preferred := NewSDL3AudioBackend(); preferred != nil {
		backend = preferred
	} else if preferred := NewOtoBackend(); preferred != nil {
		backend = preferred
	}

	if err := a.sys.Init(backend, 44100, false); err != nil {
		fallback := Backend(NewNullBackend())
		if err2 := a.sys.Init(fallback, 44100, false); err2 != nil {
			return err
		}
	}

	return a.sys.Startup()
}

func (a *AudioAdapter) Update(origin, forward, right, up [3]float32) {
	if a.sys != nil {
		a.sys.Update(origin, forward, right, up)
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

func (a *AudioAdapter) StartSound(entNum, entChannel int, sfx *SFX, origin [3]float32, vol, attenuation float32) {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.StartSound(entNum, entChannel, sfx, origin, vol, attenuation)
}

func (a *AudioAdapter) StartStaticSound(sfx *SFX, origin [3]float32, vol, attenuation float32) {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.StartStaticSound(sfx, origin, vol, attenuation)
}

func (a *AudioAdapter) ClearStaticSounds() {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.ClearStaticSounds()
}

func (a *AudioAdapter) SetListener(origin, forward, right, up [3]float32) {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.SetListener(origin, forward, right, up)
}

func (a *AudioAdapter) StopSound(entNum, entChannel int) {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.StopSound(entNum, entChannel)
}

func (a *AudioAdapter) PlayCDTrack(track, loopTrack int, loader func(string) ([]byte, error)) error {
	if a == nil || a.sys == nil {
		return nil
	}
	return a.sys.PlayCDTrack(track, loopTrack, loader)
}

func (a *AudioAdapter) StopMusic() {
	if a == nil || a.sys == nil {
		return
	}
	a.sys.StopMusic()
}
