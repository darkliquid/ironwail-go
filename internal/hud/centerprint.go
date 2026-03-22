// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

package hud

import (
	"fmt"
	"strings"
	"time"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/renderer"
)

const (
	centerPrintBackgroundCVar = "scr_centerprintbg"
	menuBGAlphaCVar           = "scr_menubgalpha"
	printSpeedCVar            = "scr_printspeed"
	centerPrintDefaultHold    = 2.0
	notifyFadeCVar            = "con_notifyfade"
	notifyFadeTimeCVar        = "con_notifyfadetime"
)

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
		prevCanvas := rc.Canvas().Type
		rc.SetCanvas(renderer.CanvasMenu)
		defer rc.SetCanvas(prevCanvas)
		cp.drawIntermissionOverlay(rc, state)
		return
	case 2, 3:
		prevCanvas := rc.Canvas().Type
		rc.SetCanvas(renderer.CanvasMenu)
		defer rc.SetCanvas(prevCanvas)
		cp.drawFinaleOverlay(rc, state)
		return
	}

	message, alpha := cp.activeCenterText(state)
	if message == "" {
		return
	}
	cp.drawTextBlock(rc, message, screenWidth, regularCenterprintY(screenHeight, message), centerprintBackgroundMode(), alpha)
}

// drawIntermissionOverlay renders the level-completion screen (Intermission 1).
// It shows the "COMPLETE" and inter-level graphics, the level name, and the
// three completion statistics: time, secrets found, and monsters killed.
func (cp *Centerprint) drawIntermissionOverlay(rc renderer.RenderContext, state State) {
	if cp.completePic != nil {
		rc.DrawPic((320-int(cp.completePic.Width))/2, 8, cp.completePic)
	}
	if cp.interPic != nil {
		rc.DrawPic((320-int(cp.interPic.Width))/2, 56, cp.interPic)
	}

	if state.LevelName != "" {
		cp.drawTextBlock(rc, state.LevelName, 320, 36, 0, 1)
	}

	const rowX = 72
	const rowValueX = 184
	rowY := 64
	DrawString(rc, rowX, rowY, "time")
	DrawString(rc, rowValueX, rowY, formatIntermissionTime(state.CompletedTime))
	rowY += 40
	DrawString(rc, rowX, rowY, "secrets")
	DrawString(rc, rowValueX, rowY, fmt.Sprintf("%d/%d", state.Secrets, state.TotalSecrets))
	rowY += 40
	DrawString(rc, rowX, rowY, "monsters")
	DrawString(rc, rowValueX, rowY, fmt.Sprintf("%d/%d", state.Monsters, state.TotalMonsters))
}

// drawFinaleOverlay renders the end-of-episode text crawl (Intermission 2/3).
// It displays the "FINALE" header graphic and progressively reveals the story
// text using a typewriter effect controlled by scr_printspeed.
func (cp *Centerprint) drawFinaleOverlay(rc renderer.RenderContext, state State) {
	if cp.finalePic != nil {
		rc.DrawPic((320-int(cp.finalePic.Width))/2, 16, cp.finalePic)
	}
	message, _ := cp.activeCenterText(state)
	text := cp.revealedFinaleText(state, message)
	if text == "" {
		return
	}
	cp.drawTextBlock(rc, text, 320, centerprintY(200, text), 0, 1)
}

// revealedFinaleText returns the portion of the center text that should be
// visible during a finale sequence. The number of visible characters increases
// over time at scr_printspeed, creating the typewriter effect.
// For non-finale intermissions, the full text is returned immediately.
func (cp *Centerprint) revealedFinaleText(state State, text string) string {
	if text == "" || (state.Intermission != 2 && state.Intermission != 3) {
		return text
	}

	visibleChars := int((state.Time - state.CenterPrintAt) * finaleRevealCharsPerSecond())
	if visibleChars <= 0 {
		return ""
	}

	return limitCenterTextVisibleChars(text, visibleChars)
}

func finaleRevealCharsPerSecond() float64 {
	if cv := cvar.Get(printSpeedCVar); cv != nil {
		return cv.Float
	}
	return 8
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
// the given Y position. For regular in-game centerprints it can also draw one
// of the canonical background styles selected via scr_centerprintbg.
func (cp *Centerprint) drawTextBlock(rc renderer.RenderContext, message string, screenWidth, y int, backgroundMode int, alpha float64) {
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
	drawCenterprintBackground(rc, screenWidth, y, len(lines), maxChars, backgroundMode, alpha)
	for i, line := range lines {
		x := (screenWidth - len(line)*8) / 2
		drawCenterprintLine(rc, x, y+i*8, line, i, alpha)
	}
}

// activeCenterText returns the text that should currently be displayed as the
// centerprint message. It checks the server-sent CenterPrint first (with a
// hold-time expiry), then falls back to any manually set message. During
// finale sequences (Intermission 2/3) the CenterPrint text is always shown
// regardless of hold time.
func (cp *Centerprint) activeCenterText(state State) (string, float64) {
	if state.CenterPrint != "" {
		if state.Intermission == 2 || state.Intermission == 3 {
			return state.CenterPrint, 1
		}
		if alpha := centerprintVisualAlpha(state); alpha > 0 {
			return state.CenterPrint, alpha
		}
	}
	if cp.IsActive() {
		return cp.manualMessage, 1
	}
	return "", 0
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

func centerprintBackgroundMode() int {
	return max(0, min(3, cvar.IntValue(centerPrintBackgroundCVar)))
}

func centerprintFadeTail() float64 {
	if cvar.FloatValue(notifyFadeCVar) <= 0 {
		return 0
	}
	return max(0, cvar.FloatValue(notifyFadeCVar)*cvar.FloatValue(notifyFadeTimeCVar))
}

func centerprintVisualAlpha(state State) float64 {
	hold := state.CenterPrintHold
	if hold <= 0 {
		hold = centerPrintDefaultHold
	}
	elapsed := state.Time - state.CenterPrintAt
	if elapsed <= hold {
		return 1
	}
	fade := centerprintFadeTail()
	if fade <= 0 {
		return 0
	}
	if elapsed > hold+fade {
		return 0
	}
	return max(0, min(1, (hold+fade-elapsed)/fade))
}

func centerprintY(screenHeight int, message string) int {
	lineCount := 1
	for _, r := range message {
		if r == '\n' {
			lineCount++
		}
	}
	if lineCount <= 4 {
		return int(float64(screenHeight) * 0.35)
	}
	return 48 * screenHeight / 200
}

func regularCenterprintY(screenHeight int, message string) int {
	y := centerprintY(screenHeight, message)
	if cvar.IntValue("crosshair") != 0 && cvar.FloatValue("scr_viewsize") < 130 {
		y -= 8
	}
	return y
}

func drawCenterprintBackground(rc renderer.RenderContext, screenWidth, y, lines, maxChars, mode int, alpha float64) {
	if rc == nil || lines <= 0 || maxChars <= 0 || mode <= 0 {
		return
	}

	boxWidth := (maxChars + 2) * 8
	boxHeight := lines*8 + 8
	boxX := (screenWidth - boxWidth) / 2
	boxY := y - 4
	fillAlpha := centerprintBackgroundAlpha(alpha)

	switch mode {
	case 1:
		drawCenterprintFill(rc, boxX, boxY, boxWidth, boxHeight, 0, fillAlpha)
		rc.DrawFill(boxX, boxY, boxWidth, 1, 15)
		rc.DrawFill(boxX, boxY+boxHeight-1, boxWidth, 1, 15)
		rc.DrawFill(boxX, boxY, 1, boxHeight, 15)
		rc.DrawFill(boxX+boxWidth-1, boxY, 1, boxHeight, 15)
	case 2:
		drawCenterprintFill(rc, boxX, boxY, boxWidth, boxHeight, 0, fillAlpha)
	case 3:
		drawCenterprintFill(rc, 0, boxY, screenWidth, boxHeight, 0, fillAlpha)
	}
}

func centerprintBackgroundAlpha(alpha float64) float64 {
	return alpha * max(0, min(1, cvar.FloatValue(menuBGAlphaCVar)))
}

func drawCenterprintFill(rc renderer.RenderContext, x, y, w, h int, color byte, alpha float64) {
	if rc == nil || w <= 0 || h <= 0 || alpha <= 0 {
		return
	}
	if alpha >= 1 {
		rc.DrawFill(x, y, w, h, color)
		return
	}
	rc.DrawFillAlpha(x, y, w, h, color, float32(alpha))
}

func drawCenterprintLine(rc renderer.RenderContext, x, y int, text string, lineIndex int, alpha float64) {
	if rc == nil || alpha <= 0 {
		return
	}
	type characterAlphaDrawer interface {
		DrawCharacterAlpha(x, y int, num int, alpha float32)
	}
	if alphaDrawer, ok := rc.(characterAlphaDrawer); ok {
		for i, ch := range text {
			alphaDrawer.DrawCharacterAlpha(x+i*8, y, int(ch), float32(alpha))
		}
		return
	}
	for i, ch := range text {
		if shouldDrawCenterprintChar(lineIndex, i, alpha) {
			rc.DrawCharacter(x+i*8, y, int(ch))
		}
	}
}

func shouldDrawCenterprintChar(lineIndex, charIndex int, alpha float64) bool {
	switch {
	case alpha >= 0.875:
		return true
	case alpha >= 0.625:
		return (lineIndex+charIndex)%4 != 0
	case alpha >= 0.375:
		return (lineIndex+charIndex)%2 == 0
	default:
		return (lineIndex+charIndex)%4 == 0
	}
}
