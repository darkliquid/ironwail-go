// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

import (
	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

// StatusBar renders the Quake-style status bar at the bottom of the screen.
type StatusBar struct {
	drawManager *draw.Manager
	palette     []byte
}

// NewStatusBar creates a new status bar renderer.
func NewStatusBar(dm *draw.Manager) *StatusBar {
	var pal []byte
	if dm != nil {
		pal = dm.Palette()
	}
	return &StatusBar{
		drawManager: dm,
		palette:     pal,
	}
}

// Draw renders the status bar with health, armor, and ammo indicators.
func (sb *StatusBar) Draw(rc renderer.RenderContext, health, armor, ammo, screenWidth, screenHeight int) {
	if rc == nil {
		return
	}

	// Status bar height in pixels (original Quake: 24 pixels at 320x200 scale)
	sbHeight := 24
	sbY := screenHeight - sbHeight
	if sbY < 0 {
		sbY = 0
	}

	// Scale for modern resolutions (320 → screenWidth)
	scale := float32(screenWidth) / 320.0
	if scale < 1 {
		scale = 1
	}

	// Draw background bar (dark brown/gray - Quake palette index 4)
	bgColor := byte(4)
	rc.DrawFill(0, sbY, screenWidth, sbHeight, bgColor)

	// Health bar (red, left side)
	// Original: 0-100, displayed as a bar
	healthWidth := int(float32(health) / 100.0 * 100.0 * scale)
	if healthWidth < 0 {
		healthWidth = 0
	}
	if healthWidth > int(100*scale) {
		healthWidth = int(100 * scale)
	}
	barY := sbY + 8
	barHeight := 8
	rc.DrawFill(10, barY, healthWidth, barHeight, 48) // Palette index 48 = red

	// Armor bar (green, center)
	armorWidth := int(float32(armor) / 100.0 * 100.0 * scale)
	if armorWidth < 0 {
		armorWidth = 0
	}
	if armorWidth > int(100*scale) {
		armorWidth = int(100 * scale)
	}
	armorX := 10 + int(110*scale)
	rc.DrawFill(armorX, barY, armorWidth, barHeight, 56) // Palette index 56 = green

	// Ammo bar (yellow, right side)
	ammoWidth := int(float32(ammo) / 100.0 * 50.0 * scale)
	if ammoWidth < 0 {
		ammoWidth = 0
	}
	if ammoWidth > int(50*scale) {
		ammoWidth = int(50 * scale)
	}
	ammoX := screenWidth - int(60*scale)
	rc.DrawFill(ammoX, barY, ammoWidth, barHeight, 111) // Palette index 111 = yellow
}
