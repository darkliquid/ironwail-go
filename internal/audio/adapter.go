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

func (a *AudioAdapter) Shutdown() {
	if a.sys != nil {
		a.sys.Shutdown()
	}
}
