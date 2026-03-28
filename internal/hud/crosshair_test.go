// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

func TestCrosshairUpdateCvarCharacterSelection(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		want  int
	}{
		{name: "disabled at zero", value: 0, want: 0},
		{name: "plus at one", value: 1, want: int('+')},
		{name: "dot above one", value: 2, want: 15},
		{name: "negative custom glyph", value: -5, want: 5},
		{name: "negative custom glyph wraps to 8-bit", value: -256, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var c Crosshair
			c.UpdateCvar(tt.value)
			if c.crosshairChar != tt.want {
				t.Fatalf("UpdateCvar(%v) char = %d, want %d", tt.value, c.crosshairChar, tt.want)
			}
		})
	}
}

func TestCrosshairDrawCenteredWhenEnabled(t *testing.T) {
	rc := &mockRenderContext{}
	c := Crosshair{crosshairChar: int('+')}

	c.Draw(rc, State{}, 640, 480)

	if len(rc.characters) != 1 {
		t.Fatalf("Draw drew %d characters, want 1", len(rc.characters))
	}
	got := rc.characters[0]
	if got.x != -4 || got.y != -4 || got.num != int('+') {
		t.Fatalf("Draw position/char = (%d,%d,%d), want (-4,-4,%d)", got.x, got.y, got.num, int('+'))
	}
}

func TestCrosshairDrawSkipsIntermissionAndDisabled(t *testing.T) {
	rc := &mockRenderContext{}
	enabled := Crosshair{crosshairChar: int('+')}
	disabled := Crosshair{crosshairChar: 0}

	enabled.Draw(rc, State{Intermission: 1}, 640, 480)
	disabled.Draw(rc, State{}, 640, 480)

	if len(rc.characters) != 0 {
		t.Fatalf("expected no crosshair draw, got %d characters", len(rc.characters))
	}
}

func TestCrosshairDrawSkipsCutsceneAndLargeViewsize(t *testing.T) {
	cvar.Register("scr_viewsize", "100", cvar.FlagArchive, "")
	rc := &mockRenderContext{}
	enabled := Crosshair{crosshairChar: int('+')}

	enabled.Draw(rc, State{InCutscene: true}, 640, 480)
	cvar.Set("scr_viewsize", "130")
	t.Cleanup(func() {
		cvar.Set("scr_viewsize", "100")
	})
	enabled.Draw(rc, State{}, 640, 480)

	if len(rc.characters) != 0 {
		t.Fatalf("expected no crosshair draw, got %d characters", len(rc.characters))
	}
}
