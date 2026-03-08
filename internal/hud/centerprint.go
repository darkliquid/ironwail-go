// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

import (
	"fmt"
	"strings"
	"time"

	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

const finaleRevealCharsPerSecond = 8.0

// Centerprint displays centered text messages on the screen.
// These are typically used for level completion, key pickups, and important game events.
type Centerprint struct {
	drawManager *draw.Manager
	completePic *image.QPic
	interPic    *image.QPic
	finalePic   *image.QPic

	manualMessage string
	manualExpiry  time.Time
}

// NewCenterprint creates a new centerprint manager.
func NewCenterprint(dm *draw.Manager) *Centerprint {
	cp := &Centerprint{drawManager: dm}
	if dm != nil {
		cp.completePic = dm.GetPic("gfx/complete.lmp")
		cp.interPic = dm.GetPic("gfx/inter.lmp")
		cp.finalePic = dm.GetPic("gfx/finale.lmp")
	}
	return cp
}

// SetMessage displays a centered message for the specified duration.
func (cp *Centerprint) SetMessage(message string, duration time.Duration) {
	cp.manualMessage = message
	cp.manualExpiry = time.Now().Add(duration)
}

// Clear removes any active centerprint message.
func (cp *Centerprint) Clear() {
	cp.manualMessage = ""
	cp.manualExpiry = time.Time{}
}

// IsActive returns true if there's an active centerprint message.
func (cp *Centerprint) IsActive() bool {
	return cp.manualMessage != "" && time.Now().Before(cp.manualExpiry)
}

// Draw renders centerprint/intermission/finale overlays.
func (cp *Centerprint) Draw(rc renderer.RenderContext, state State, screenWidth, screenHeight int) {
	if rc == nil {
		return
	}

	switch state.Intermission {
	case 1:
		cp.drawIntermissionOverlay(rc, state, screenWidth)
		return
	case 2, 3:
		cp.drawFinaleOverlay(rc, state, screenWidth, screenHeight)
		return
	}

	message := cp.activeCenterText(state)
	if message == "" {
		return
	}
	cp.drawTextBlock(rc, message, screenWidth, screenHeight/3, true)
}

func (cp *Centerprint) drawIntermissionOverlay(rc renderer.RenderContext, state State, screenWidth int) {
	baseX := (screenWidth - 320) / 2
	if cp.completePic != nil {
		rc.DrawPic(baseX+(320-int(cp.completePic.Width))/2, 24, cp.completePic)
	}
	if cp.interPic != nil {
		rc.DrawPic(baseX+(320-int(cp.interPic.Width))/2, 56, cp.interPic)
	}

	if state.LevelName != "" {
		cp.drawTextBlock(rc, state.LevelName, screenWidth, 80, false)
	}

	const rowX = 72
	const rowValueX = 184
	rowY := 128
	DrawString(rc, baseX+rowX, rowY, "time")
	DrawString(rc, baseX+rowValueX, rowY, formatIntermissionTime(state.CompletedTime))
	rowY += 16
	DrawString(rc, baseX+rowX, rowY, "secrets")
	DrawString(rc, baseX+rowValueX, rowY, fmt.Sprintf("%d/%d", state.Secrets, state.TotalSecrets))
	rowY += 16
	DrawString(rc, baseX+rowX, rowY, "monsters")
	DrawString(rc, baseX+rowValueX, rowY, fmt.Sprintf("%d/%d", state.Monsters, state.TotalMonsters))
}

func (cp *Centerprint) drawFinaleOverlay(rc renderer.RenderContext, state State, screenWidth, screenHeight int) {
	if cp.finalePic != nil {
		rc.DrawPic((screenWidth-int(cp.finalePic.Width))/2, 16, cp.finalePic)
	}
	text := cp.revealedFinaleText(state, cp.activeCenterText(state))
	if text == "" {
		return
	}
	cp.drawTextBlock(rc, text, screenWidth, screenHeight/3, false)
}

func (cp *Centerprint) revealedFinaleText(state State, text string) string {
	if text == "" || (state.Intermission != 2 && state.Intermission != 3) {
		return text
	}

	visibleChars := int((state.Time - state.CenterPrintAt) * finaleRevealCharsPerSecond)
	if visibleChars <= 0 {
		return ""
	}

	return limitCenterTextVisibleChars(text, visibleChars)
}

func limitCenterTextVisibleChars(text string, visibleChars int) string {
	if visibleChars <= 0 {
		return ""
	}

	seen := 0
	for i, r := range text {
		if r == '\n' || r == '\r' {
			continue
		}
		seen++
		if seen > visibleChars {
			return text[:i]
		}
	}
	return text
}

func (cp *Centerprint) drawTextBlock(rc renderer.RenderContext, message string, screenWidth, y int, boxed bool) {
	lines := strings.Split(strings.ReplaceAll(message, "\r\n", "\n"), "\n")
	maxChars := 0
	for _, line := range lines {
		if len(line) > maxChars {
			maxChars = len(line)
		}
	}
	if maxChars == 0 {
		return
	}
	if boxed {
		boxWidth := (maxChars + 2) * 8
		boxHeight := len(lines)*8 + 8
		boxX := (screenWidth - boxWidth) / 2
		boxY := y - 4
		rc.DrawFill(boxX, boxY, boxWidth, boxHeight, 0)
		rc.DrawFill(boxX, boxY, boxWidth, 1, 15)
		rc.DrawFill(boxX, boxY+boxHeight-1, boxWidth, 1, 15)
		rc.DrawFill(boxX, boxY, 1, boxHeight, 15)
		rc.DrawFill(boxX+boxWidth-1, boxY, 1, boxHeight, 15)
	}
	for i, line := range lines {
		x := (screenWidth - len(line)*8) / 2
		DrawString(rc, x, y+i*8, line)
	}
}

func (cp *Centerprint) activeCenterText(state State) string {
	if state.CenterPrint != "" {
		if state.Intermission == 2 || state.Intermission == 3 {
			return state.CenterPrint
		}
		hold := state.CenterPrintHold
		if hold <= 0 {
			hold = 2
		}
		if state.Time-state.CenterPrintAt <= hold {
			return state.CenterPrint
		}
	}
	if cp.IsActive() {
		return cp.manualMessage
	}
	return ""
}

func formatIntermissionTime(seconds float64) string {
	total := int(seconds)
	if total < 0 {
		total = 0
	}
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}
