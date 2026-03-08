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
	state State

	// Screen dimensions
	screenWidth  int
	screenHeight int
}

// State is the subset of client state required to render the classic status bar.
type State struct {
	Health       int
	Armor        int
	Ammo         int
	WeaponModel  int
	ActiveWeapon int
	Shells       int
	Nails        int
	Rockets      int
	Cells        int
	Items        uint32
	GameType     int
	MaxClients   int
	ShowScores   bool
	Scoreboard   []ScoreEntry

	Intermission    int
	CompletedTime   float64
	Time            float64
	CenterPrint     string
	CenterPrintAt   float64
	CenterPrintHold float64
	LevelName       string
	Secrets         int
	TotalSecrets    int
	Monsters        int
	TotalMonsters   int
}

// ScoreEntry is a single player row in the multiplayer scoreboard.
type ScoreEntry struct {
	ClientIndex int
	Name        string
	Frags       int
	Colors      byte
	IsCurrent   bool
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

// SetState updates the HUD values from player/client state.
func (h *HUD) SetState(state State) {
	h.state = state
}

// State returns the latest HUD state snapshot.
func (h *HUD) State() State {
	return h.state
}

// Draw renders the complete HUD overlay.
func (h *HUD) Draw(rc renderer.RenderContext) {
	if rc == nil {
		return
	}

	// Draw status bar at bottom of screen
	if h.state.Intermission == 0 {
		h.status.Draw(rc, h.state, h.screenWidth, h.screenHeight)
	}
	h.centerprint.Draw(rc, h.state, h.screenWidth, h.screenHeight)
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
