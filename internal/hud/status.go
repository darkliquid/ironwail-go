// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

import (
	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

// StatusBar renders the Quake-style status bar at the bottom of the screen.
type StatusBar struct {
	drawManager *draw.Manager
	palette     []byte
	ibarPic     *image.QPic // Status bar background image
}

// NewStatusBar creates a new status bar renderer.
func NewStatusBar(dm *draw.Manager) *StatusBar {
	var pal []byte
	var ibar *image.QPic
	if dm != nil {
		pal = dm.Palette()
		// Load status bar background (ibar.lmp)
		ibar = dm.GetPic("ibar")
	}
	return &StatusBar{
		drawManager: dm,
		palette:     pal,
		ibarPic:     ibar,
	}
}

// Draw renders the status bar with health, armor, and ammo indicators.
// This includes the background image, numeric displays, and colored bars.
func (sb *StatusBar) Draw(rc renderer.RenderContext, health, armor, ammo, screenWidth, screenHeight int) {
	if rc == nil {
		return
	}

	// Status bar height in pixels (original Quake uses 24-pixel tall status bar at 320x200)
	// In modern resolutions, we scale proportionally
	const sbarHeight = 24
	const sbarWidth = 320

	// Calculate position - status bar is always at bottom of screen
	sbY := screenHeight - sbarHeight
	if sbY < 0 {
		sbY = 0
	}

	// Draw status bar background image if available
	if sb.ibarPic != nil {
		// The ibar image is 320 pixels wide in Quake
		// We draw it centered horizontally
		ibarX := (screenWidth - int(sb.ibarPic.Width)) / 2
		rc.DrawPic(ibarX, sbY, sb.ibarPic)
	} else {
		// Fallback: draw a simple colored background bar
		bgColor := byte(4) // Dark gray from Quake palette
		rc.DrawFill(0, sbY, screenWidth, sbarHeight, bgColor)
	}

	// Calculate scaling for HUD elements
	// Original Quake HUD is designed for 320x200, so we scale to match screen width
	scale := float32(screenWidth) / 320.0
	if scale < 1 {
		scale = 1
	}

	// Center HUD elements horizontally (HUD is 320 units wide in virtual space)
	hudCenterX := screenWidth / 2

	// Health display (left side of status bar)
	// Position: approximately 136 pixels from left edge in 320-wide space
	healthX := hudCenterX - int(184*scale)
	healthY := sbY + int(16*scale)
	DrawNumber(rc, healthX, healthY, health, 3)

	// Draw health bar indicator (optional visual bar)
	if health > 0 {
		barY := sbY + int(20*scale)
		healthWidth := int(float32(health) / 100.0 * 50.0 * scale)
		if healthWidth < 0 {
			healthWidth = 0
		}
		if healthWidth > int(50*scale) {
			healthWidth = int(50 * scale)
		}
		healthBarX := hudCenterX - int(200*scale)
		rc.DrawFill(healthBarX, barY, healthWidth, 2, 48) // Red bar
	}

	// Armor display (left-center of status bar)
	// Position: approximately 208 pixels from left edge in 320-wide space
	armorX := hudCenterX - int(112*scale)
	armorY := sbY + int(16*scale)
	DrawNumber(rc, armorX, armorY, armor, 3)

	// Draw armor bar indicator (optional visual bar)
	if armor > 0 {
		barY := sbY + int(20*scale)
		armorWidth := int(float32(armor) / 100.0 * 50.0 * scale)
		if armorWidth < 0 {
			armorWidth = 0
		}
		if armorWidth > int(50*scale) {
			armorWidth = int(50 * scale)
		}
		armorBarX := hudCenterX - int(128*scale)
		rc.DrawFill(armorBarX, barY, armorWidth, 2, 56) // Green bar
	}

	// Ammo display (right side of status bar)
	// Position: approximately 272 pixels from left edge in 320-wide space
	ammoX := hudCenterX + int(112*scale)
	ammoY := sbY + int(16*scale)
	DrawNumber(rc, ammoX, ammoY, ammo, 3)

	// Draw ammo bar indicator (optional visual bar)
	if ammo > 0 {
		barY := sbY + int(20*scale)
		ammoWidth := int(float32(ammo) / 200.0 * 40.0 * scale)
		if ammoWidth < 0 {
			ammoWidth = 0
		}
		if ammoWidth > int(40*scale) {
			ammoWidth = int(40 * scale)
		}
		ammoBarX := hudCenterX + int(72*scale)
		rc.DrawFill(ammoBarX, barY, ammoWidth, 2, 111) // Yellow bar
	}
}
