// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

// Package hud implements the Quake heads-up display rendering.
// It renders the status bar, centerprint messages, and other 2D overlays.
package hud

import (
	"time"

	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

// HUD manages the heads-up display rendering.
type HUD struct {
	drawManager *draw.Manager
	status      *StatusBar
	centerprint *Centerprint

	// Player state
	health int
	armor  int
	ammo   int
	weapon int

	// Screen dimensions
	screenWidth  int
	screenHeight int
}

// NewHUD creates a new HUD instance.
func NewHUD(dm *draw.Manager) *HUD {
	return &HUD{
		drawManager: dm,
		status:      NewStatusBar(dm),
		centerprint: NewCenterprint(dm),
	}
}

// SetScreenSize updates the screen dimensions for layout.
func (h *HUD) SetScreenSize(width, height int) {
	h.screenWidth = width
	h.screenHeight = height
}

// SetState updates the HUD values from player state.
func (h *HUD) SetState(health, armor, ammo, weapon int) {
	h.health = health
	h.armor = armor
	h.ammo = ammo
	h.weapon = weapon
}

// Draw renders the complete HUD overlay.
func (h *HUD) Draw(rc renderer.RenderContext) {
	if rc == nil {
		return
	}

	// Draw status bar at bottom of screen
	h.status.Draw(rc, h.health, h.armor, h.ammo, h.screenWidth, h.screenHeight)

	// Draw centerprint message if active
	h.centerprint.Draw(rc, h.screenWidth, h.screenHeight)
}

// SetCenterprint displays a centered message for the specified duration.
func (h *HUD) SetCenterprint(message string, duration time.Duration) {
	h.centerprint.SetMessage(message, duration)
}

// ClearCenterprint removes any active centerprint message.
func (h *HUD) ClearCenterprint() {
	h.centerprint.Clear()
}

// IsActive returns true if the HUD has any active elements.
func (h *HUD) IsActive() bool {
	return h.centerprint.IsActive()
}
