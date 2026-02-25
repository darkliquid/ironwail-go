// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"github.com/ironwail/ironwail-go/internal/host"
)

// AudioAdapter wraps audio.System to implement host.Audio interface
type AudioAdapter struct {
	sys *System
}

func NewAudioAdapter(sys *System) *AudioAdapter {
	return &AudioAdapter{sys: sys}
}

func (a *AudioAdapter) Init() error {
	// Audio system needs a backend, but for host interface
	// we'll just initialize without backend for now
	if a.sys != nil {
		return a.sys.Init(nil)
	}
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
