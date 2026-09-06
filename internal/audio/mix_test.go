// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package audio

import (
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func TestSetUnderwaterIntensityCvarAndRamping(t *testing.T) {
	m := &Mixer{}

	// When snd_waterfx is 0, intensity should stay 0 even if target is 1.0.
	m.SetUnderwaterIntensity(1.0, 0.05, 0.0)
	if m.UnderwaterIntensity() != 0 {
		t.Fatalf("expected intensity 0 with waterfx=0, got %f", m.UnderwaterIntensity())
	}

	// When snd_waterfx is 1.0 and frameTime is 0.05, intensity should increase by 0.05 * 4 = 0.2
	m.SetUnderwaterIntensity(1.0, 0.05, 1.0)
	expected := float32(0.2)
	if math.Abs(float64(m.UnderwaterIntensity()-expected)) > 1e-4 {
		t.Fatalf("expected intensity %f after ramp, got %f", expected, m.UnderwaterIntensity())
	}

	// Ramp up to target
	for i := 0; i < 10; i++ {
		m.SetUnderwaterIntensity(1.0, 0.05, 1.0)
	}
	if m.UnderwaterIntensity() != 1.0 {
		t.Fatalf("expected clamped target intensity 1.0, got %f", m.UnderwaterIntensity())
	}

	// Ramp down to 0
	for i := 0; i < 10; i++ {
		m.SetUnderwaterIntensity(0.0, 0.05, 1.0)
	}
	if m.UnderwaterIntensity() != 0.0 {
		t.Fatalf("expected ramped down intensity 0.0, got %f", m.UnderwaterIntensity())
	}
}

func TestApplyUnderwaterFilterPrimingAndAttenuation(t *testing.T) {
	m := &Mixer{}

	// Fill with constant dry signal
	for i := 0; i < 100; i++ {
		m.paintBuffer[i].Left = 10000
		m.paintBuffer[i].Right = 10000
	}

	// When intensity is 0, accumulator should be primed to the last sample
	m.underwater.Intensity = 0
	m.applyUnderwaterFilter(100)
	if m.underwater.Accum[0] != 10000 || m.underwater.Accum[1] != 10000 {
		t.Fatalf("expected primed accumulator [10000, 10000], got [%f, %f]",
			m.underwater.Accum[0], m.underwater.Accum[1])
	}

	// Now set intensity to 1.0 (submerged)
	m.underwater.Intensity = 1.0
	m.underwater.Alpha = float32(math.Exp(-float64(m.underwater.Intensity) * math.Log(12)))

	// Alternating high-frequency signal (Nyquist: +10000, -10000)
	for i := 0; i < 100; i++ {
		if i%2 == 0 {
			m.paintBuffer[i].Left = 10000
			m.paintBuffer[i].Right = 10000
		} else {
			m.paintBuffer[i].Left = -10000
			m.paintBuffer[i].Right = -10000
		}
	}

	m.applyUnderwaterFilter(100)

	// High frequency signal should be heavily attenuated by the low-pass filter
	lastSample := m.paintBuffer[99].Left
	if math.Abs(float64(lastSample)) > 2000 {
		t.Fatalf("expected high frequency to be attenuated below 2000, got %d", lastSample)
	}
}

func TestSystemWaterFxCvar(t *testing.T) {
	sys := NewSystem()
	if got := sys.getWaterFx(); got != 1.0 {
		t.Fatalf("expected default waterfx 1.0, got %f", got)
	}

	sys.initialized = true
	cv := cvar.NewCVarSystem()
	RegisterCVars(cv)
	sys.UpdateFromCVars(cv)
	if got := sys.getWaterFx(); got != 1.0 {
		t.Fatalf("expected registered default waterfx 1.0, got %f", got)
	}

	cv.Set("snd_waterfx", "0.5")
	if got := sys.getWaterFx(); got != 0.5 {
		t.Fatalf("expected waterfx 0.5, got %f", got)
	}

	mixer := NewMixer()
	sys.mixer = mixer
	sys.UpdateAmbientSounds(0.05, true, [NumAmbients]uint8{}, 1.0)
	// target is 1.0 * 0.5 = 0.5. Ramping step is 0.05 * 4 = 0.2.
	if math.Abs(float64(mixer.UnderwaterIntensity()-0.2)) > 1e-4 {
		t.Fatalf("expected intensity 0.2, got %f", mixer.UnderwaterIntensity())
	}
}

func TestAudioAdapterUnderwaterIntensity(t *testing.T) {
	sys := NewSystem()
	sys.mixer = NewMixer()
	adapter := NewAudioAdapter(sys)

	adapter.SetUnderwaterIntensity(1.0)
	if got := adapter.UnderwaterIntensity(); got <= 0 {
		t.Fatalf("expected underwater intensity > 0, got %f", got)
	}
}
