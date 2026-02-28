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
	// Audio system initialization deferred until a real backend is available (M7)
	return nil
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
