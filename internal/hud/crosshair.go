// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

import "github.com/ironwail/ironwail-go/internal/renderer"

// Crosshair renders the center-screen crosshair character.
type Crosshair struct {
	crosshairChar int
}

// UpdateCvar updates the active crosshair character from the crosshair cvar value.
//
// C Ironwail behavior:
//   - 0: disabled
//   - <0: custom character index (-value & 255)
//   - 1: '+'
//   - >1: dot (character 15)
func (c *Crosshair) UpdateCvar(value float64) {
	if value == 0 {
		c.crosshairChar = 0
		return
	}
	if value < 0 {
		c.crosshairChar = int(-value) & 255
		return
	}
	if value > 1 {
		c.crosshairChar = 15
		return
	}
	c.crosshairChar = int('+')
}

// Draw renders the crosshair centered within the active crosshair canvas.
func (c *Crosshair) Draw(rc renderer.RenderContext, state State, screenWidth, screenHeight int) {
	if rc == nil || state.Intermission != 0 || state.InCutscene || currentViewSize() >= 130 || c.crosshairChar == 0 {
		return
	}
	rc.DrawCharacter(-4, -4, c.crosshairChar)
}
