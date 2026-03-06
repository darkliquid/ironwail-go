// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

import (
	"time"

	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

// Centerprint displays centered text messages on the screen.
// These are typically used for level completion, key pickups, and important game events.
type Centerprint struct {
	drawManager *draw.Manager
	message     string
	expiryTime  time.Time
}

// NewCenterprint creates a new centerprint manager.
func NewCenterprint(dm *draw.Manager) *Centerprint {
	return &Centerprint{
		drawManager: dm,
	}
}

// SetMessage displays a centered message for the specified duration.
func (cp *Centerprint) SetMessage(message string, duration time.Duration) {
	cp.message = message
	cp.expiryTime = time.Now().Add(duration)
}

// Clear removes any active centerprint message.
func (cp *Centerprint) Clear() {
	cp.message = ""
	cp.expiryTime = time.Time{}
}

// IsActive returns true if there's an active centerprint message.
func (cp *Centerprint) IsActive() bool {
	return cp.message != "" && time.Now().Before(cp.expiryTime)
}

// Draw renders the centerprint if active.
func (cp *Centerprint) Draw(rc renderer.RenderContext, screenWidth, screenHeight int) {
	if rc == nil || !cp.IsActive() {
		return
	}

	// Check if message has expired
	if time.Now().After(cp.expiryTime) {
		cp.Clear()
		return
	}

	// Calculate position (center of screen)
	// For MVP, draw a simple background box
	msgLen := len(cp.message)
	if msgLen == 0 {
		return
	}

	// Approximate text width (8 pixels per character at 320 scale)
	scale := float32(screenWidth) / 320.0
	if scale < 1 {
		scale = 1
	}

	boxWidth := int(float32(msgLen+2) * 8 * scale)
	boxHeight := int(24 * scale)
	boxX := (screenWidth - boxWidth) / 2
	boxY := screenHeight/2 - boxHeight/2

	// Draw semi-transparent background (palette index 0 = black)
	rc.DrawFill(boxX, boxY, boxWidth, boxHeight, 0)

	// Draw border (palette index 15 = bright white)
	borderWidth := 2
	rc.DrawFill(boxX, boxY, boxWidth, borderWidth, 15)
	rc.DrawFill(boxX, boxY+boxHeight-borderWidth, boxWidth, borderWidth, 15)
	rc.DrawFill(boxX, boxY, borderWidth, boxHeight, 15)
	rc.DrawFill(boxX+boxWidth-borderWidth, boxY, borderWidth, boxHeight, 15)

	// Render each character of the message centered in the box
	charW := int(8 * scale)
	textWidth := msgLen * charW
	textX := (screenWidth - textWidth) / 2
	textY := boxY + int(8*scale)
	for i, ch := range cp.message {
		rc.DrawCharacter(textX+i*charW, textY, int(ch))
	}
}
