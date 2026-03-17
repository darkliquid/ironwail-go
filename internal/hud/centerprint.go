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

// finaleRevealCharsPerSecond controls the typewriter-style text reveal speed
// during the end-of-episode finale sequences (Intermission types 2 and 3).
// At 8 characters per second, the text gradually appears as if being typed,
// creating the dramatic storytelling effect between Quake episodes.
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

// drawIntermissionOverlay renders the level-completion screen (Intermission 1).
// It shows the "COMPLETE" and inter-level graphics, the level name, and the
// three completion statistics: time, secrets found, and monsters killed.
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

// drawFinaleOverlay renders the end-of-episode text crawl (Intermission 2/3).
// It displays the "FINALE" header graphic and progressively reveals the story
// text using a typewriter effect controlled by finaleRevealCharsPerSecond.
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

// revealedFinaleText returns the portion of the center text that should be
// visible during a finale sequence. The number of visible characters increases
// over time at finaleRevealCharsPerSecond, creating the typewriter effect.
// For non-finale intermissions, the full text is returned immediately.
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

// limitCenterTextVisibleChars truncates text to at most visibleChars printable
// characters. Newline and carriage-return characters are not counted towards
// the limit, ensuring line breaks don't consume character slots.
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

// drawTextBlock renders a multi-line text message centered on the screen at
// the given Y position. If boxed is true, a dark background rectangle with
// a thin white border is drawn behind the text for readability (used for
// in-game centerprint messages but not for intermission/finale text).
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

// activeCenterText returns the text that should currently be displayed as the
// centerprint message. It checks the server-sent CenterPrint first (with a
// hold-time expiry), then falls back to any manually set message. During
// finale sequences (Intermission 2/3) the CenterPrint text is always shown
// regardless of hold time.
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

// formatIntermissionTime converts a floating-point seconds value to a "M:SS"
// string suitable for the intermission time display (e.g. 125.7 → "2:05").
func formatIntermissionTime(seconds float64) string {
	total := int(seconds)
	if total < 0 {
		total = 0
	}
	return fmt.Sprintf("%d:%02d", total/60, total%60)
}
