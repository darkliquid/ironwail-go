// draw.go — Console rendering (2D overlay drawing)
//
// This file handles rendering the Quake console overlay using the 2D drawing
// system. The console is drawn character-by-character using Quake's bitmap
// font (conchars.lmp — a 128×128 texture containing a 16×16 grid of 8×8
// pixel glyphs). Each glyph index maps to an ASCII-like code point; setting
// bit 7 (values 128–255) selects an alternate "bronze" colour row, which is
// how Quake renders warnings and chat messages in a distinct colour.
//
// The file implements two visual modes:
//   - Full console: a translucent overlay covering the top half of the screen,
//     showing the scrollback buffer and the input prompt. Toggled with ~.
//   - Notify lines: a small set of recently-printed lines shown at the top of
//     the screen during gameplay (without the console open) that fade after a
//     few seconds. This lets important messages reach the player without
//     requiring the console to be open.
//
// Rendering is decoupled from the console's data model via the DrawContext
// interface, so this file has no dependency on OpenGL, Vulkan, or any
// specific graphics API.
package console

import (
	"strings"
	"sync"
	"time"

	"github.com/ironwail/ironwail-go/internal/cvar"
	qimage "github.com/ironwail/ironwail-go/internal/image"
)

// DrawContext is the rendering abstraction that decouples console drawing from
// any specific graphics backend. A renderer (OpenGL, Vulkan, software, etc.)
// implements this interface so the console can issue primitive draw calls
// without importing graphics-API-specific packages.
//
// DrawFill fills a rectangle with a palette colour (used for the console
// background). DrawCharacter renders a single glyph from Quake's bitmap font
// at the given pixel coordinates; "num" is the character index into the
// conchars texture (0–255).
type DrawContext interface {
	DrawFill(x, y, w, h int, color byte)
	DrawCharacter(x, y int, num int)
	DrawPic(x, y int, pic *qimage.QPic)
}

type scaledPicKey struct {
	src           *qimage.QPic
	width, height uint32
}

var scaledPicCache struct {
	mu   sync.Mutex
	pics map[scaledPicKey]*qimage.QPic
}

// Drawing constants for the console overlay.
//
// Quake's bitmap font (conchars) uses a fixed 8×8 pixel grid for each glyph.
// These constants define the character cell size, how long notify lines remain
// visible, and the minimum console height in pixels.
const (
	// consoleCharWidth is the width in pixels of a single character cell in
	// Quake's bitmap font (conchars.lmp). All text layout is a simple
	// multiple of this value — no variable-width font metrics are needed.
	consoleCharWidth = 8

	// consoleCharHeight is the height in pixels of a single character cell.
	consoleCharHeight = 8

	// consoleNotifyDefaultTTL is the fallback notify lifetime when con_notifytime
	// is unset or invalid. Classic Quake defaults to 3 seconds.
	consoleNotifyDefaultTTL = 3 * time.Second

	// consoleMinDrawHeight prevents the console from being drawn too small
	// to be readable — at least 4 rows of text.
	consoleMinDrawHeight = consoleCharHeight * 4

	consoleCursorBlinkHz = 4
)

var consoleNow = time.Now

// Draw is the package-level entry point that renders the global console.
// It delegates to the singleton globalConsole. When full is true, the
// entire drop-down console overlay is drawn; when false, only the compact
// notify lines are rendered over the game view. When forcedup is true,
// the console fills the entire screen (used before a map is loaded).
func Draw(rc DrawContext, screenWidth, screenHeight int, full bool, background *qimage.QPic, forcedup ...bool) {
	forced := len(forcedup) > 0 && forcedup[0]
	globalConsole.Draw(rc, screenWidth, screenHeight, full, background, forced)
}

// Draw renders either the full console overlay (when full == true) or just the
// compact notify lines (when full == false). It first recalculates the line
// width to match the current screen resolution (via Resize), then dispatches
// to drawFull or drawNotify.
//
// Safety checks ensure we never attempt to draw with invalid dimensions or
// nil contexts.
func (c *Console) Draw(rc DrawContext, screenWidth, screenHeight int, full bool, background *qimage.QPic, forcedup ...bool) {
	if c == nil || rc == nil || screenWidth < consoleCharWidth || screenHeight < consoleCharHeight {
		return
	}

	charsWide := screenWidth / consoleCharWidth
	if charsWide <= 0 {
		return
	}
	lineWidth := charsWide - Margin*2
	if lineWidth < 1 {
		lineWidth = 1
	}
	c.Resize(lineWidth)

	if full {
		forced := len(forcedup) > 0 && forcedup[0]
		c.drawFull(rc, screenWidth, screenHeight, charsWide, background, forced)
		return
	}
	c.drawNotify(rc, charsWide)
}

// drawFull renders the complete drop-down console overlay. The layout is:
//
//	┌────────────────────────────── screenWidth ──────────────────────────────┐
//	│ (optional) "^^^" scroll indicator row  (if backScroll > 0)            │
//	│ scrollback line N-2                                                   │
//	│ scrollback line N-1                                                   │
//	│ scrollback line N   (most recent, adjusted for backScroll)            │
//	│ ] user input line_                                                    │
//	└───────────────────────────────────────────────────────────────────────┘
//
// The console occupies the top half of the screen (screenHeight / 2). A solid
// fill is drawn behind the text for readability. Lines are fetched from the
// ring buffer under the read lock, copied out, then drawn after releasing the
// lock to minimise lock contention with the printing goroutine.
func (c *Console) drawFull(rc DrawContext, screenWidth, screenHeight, charsWide int, background *qimage.QPic, forcedup bool) {
	consoleHeight := screenHeight / 2
	if forcedup || consoleHeight < consoleMinDrawHeight {
		consoleHeight = screenHeight
	}
	if consoleHeight <= 0 {
		return
	}

	if pic := scaledBackgroundPic(background, screenWidth, consoleHeight); pic != nil {
		rc.DrawPic(0, 0, pic)
	} else {
		rc.DrawFill(0, 0, screenWidth, consoleHeight, 0)
	}

	c.mu.RLock()
	current := c.current
	backScroll := c.backScroll
	inputLine := append([]rune(nil), c.inputLine...)
	cursorPos := c.cursorPos
	visibleRows := consoleHeight/consoleCharHeight - 2
	if visibleRows < 1 {
		visibleRows = 1
	}
	bottomLine := current - backScroll
	startLine := bottomLine - (visibleRows - 1)
	lines := make([][]byte, 0, visibleRows)
	for line := startLine; line <= bottomLine; line++ {
		lines = append(lines, c.lineBytesLocked(line))
	}
	c.mu.RUnlock()

	if backScroll > 0 {
		drawByteText(rc, consoleCharWidth, 0, []byte(strings.Repeat("^", max(0, charsWide-2))), charsWide-2)
	}

	y := consoleCharHeight
	for _, line := range lines {
		drawByteText(rc, consoleCharWidth, y, line, charsWide-2)
		y += consoleCharHeight
	}

	prompt := append([]rune{']'}, inputLine...)
	visiblePrompt, cursorCol := clipPromptWithCursor(prompt, cursorPos+1, max(1, charsWide-3))
	drawRuneText(rc, consoleCharWidth, consoleHeight-consoleCharHeight, visiblePrompt)
	drawBlinkCursor(rc, consoleCharWidth+cursorCol*consoleCharWidth, consoleHeight-consoleCharHeight, consoleNow())
}

func scaledBackgroundPic(pic *qimage.QPic, width, height int) *qimage.QPic {
	if pic == nil || width <= 0 || height <= 0 {
		return nil
	}
	key := scaledPicKey{src: pic, width: uint32(width), height: uint32(height)}
	scaledPicCache.mu.Lock()
	defer scaledPicCache.mu.Unlock()
	if scaledPicCache.pics == nil {
		scaledPicCache.pics = make(map[scaledPicKey]*qimage.QPic)
	}
	if cached := scaledPicCache.pics[key]; cached != nil {
		return cached
	}
	scaled := scaleQPicNearest(pic, width, height)
	if scaled == nil {
		return nil
	}
	scaledPicCache.pics[key] = scaled
	return scaled
}

func scaleQPicNearest(pic *qimage.QPic, width, height int) *qimage.QPic {
	if pic == nil || width <= 0 || height <= 0 || pic.Width == 0 || pic.Height == 0 {
		return nil
	}
	if int(pic.Width) == width && int(pic.Height) == height {
		return pic
	}

	srcW := int(pic.Width)
	srcH := int(pic.Height)
	if len(pic.Pixels) < srcW*srcH {
		return nil
	}

	scaled := &qimage.QPic{
		Width:  uint32(width),
		Height: uint32(height),
		Pixels: make([]byte, width*height),
	}
	for y := range height {
		srcY := y * srcH / height
		rowOffset := y * width
		srcRowOffset := srcY * srcW
		for x := range width {
			srcX := x * srcW / width
			scaled.Pixels[rowOffset+x] = pic.Pixels[srcRowOffset+srcX]
		}
	}
	return scaled
}

// drawNotify renders only the most recent lines that were printed within the
// last consoleNotifyTTL (3 seconds). These appear at the very top of the
// screen during gameplay — without the console being open — so the player
// can see kill messages, chat, and other transient information. Lines whose
// timestamps have expired are silently skipped.
func (c *Console) drawNotify(rc DrawContext, charsWide int) {
	now := consoleNow()
	centered := cvar.BoolValue("con_notifycenter")

	c.mu.RLock()
	current := c.current
	notifyTimes := c.notifyTimes
	type notifyLine struct {
		text  []byte
		alpha float64
	}
	lines := make([]notifyLine, 0, NumNotifyTimes)
	for line := current - NumNotifyTimes + 1; line <= current; line++ {
		if line < 0 {
			continue
		}
		ts := notifyTimes[line%NumNotifyTimes]
		alpha := notifyAlpha(now, ts)
		if alpha <= 0 {
			continue
		}
		lines = append(lines, notifyLine{text: c.lineBytesLocked(line), alpha: alpha})
	}
	c.mu.RUnlock()

	y := 0
	for i, line := range lines {
		if centered {
			drawCenteredNotifyText(rc, charsWide, y+16, line.text, i, line.alpha)
		} else {
			drawByteTextAlpha(rc, consoleCharWidth, y, line.text, charsWide-2, i, line.alpha)
		}
		y += consoleCharHeight
	}
}

// lineBytesLocked returns a defensive copy of the raw byte content for the
// given line number. Trailing spaces are trimmed. The caller MUST hold at
// least c.mu.RLock — this method does not acquire the lock itself, which is
// intentional: the draw methods snapshot several related fields (current,
// backScroll, notifyTimes) under a single lock acquisition for consistency,
// then call lineBytesLocked for each visible line.
func (c *Console) lineBytesLocked(lineNum int) []byte {
	if c.totalLines <= 0 || c.lineWidth <= 0 || len(c.text) == 0 {
		return nil
	}

	line := lineNum % c.totalLines
	if line < 0 {
		line += c.totalLines
	}

	start := line * c.lineWidth
	if start < 0 || start >= len(c.text) {
		return nil
	}

	end := start + c.lineWidth
	if end > len(c.text) {
		end = len(c.text)
	}

	text := append([]byte(nil), c.text[start:end]...)
	for len(text) > 0 && text[len(text)-1] == ' ' {
		text = text[:len(text)-1]
	}
	return text
}

// drawByteText renders a slice of raw bytes as a horizontal row of characters
// using the DrawContext. Each byte is a direct glyph index into Quake's bitmap
// font (0–255). The text is clipped to maxChars columns to prevent drawing
// outside the console area. This is used for scrollback lines, which are
// stored as []byte in the ring buffer.
func drawByteText(rc DrawContext, x, y int, text []byte, maxChars int) {
	drawByteTextAlpha(rc, x, y, text, maxChars, 0, 1)
}

func drawByteTextAlpha(rc DrawContext, x, y int, text []byte, maxChars, lineIndex int, alpha float64) {
	if rc == nil || maxChars == 0 {
		return
	}
	if maxChars > 0 && len(text) > maxChars {
		text = text[:maxChars]
	}
	for i, ch := range text {
		if shouldDrawNotifyChar(lineIndex, i, alpha) {
			rc.DrawCharacter(x+i*consoleCharWidth, y, int(ch))
		}
	}
}

func drawCenteredNotifyText(rc DrawContext, charsWide, y int, text []byte, lineIndex int, alpha float64) {
	if rc == nil || charsWide <= 0 || alpha <= 0 {
		return
	}
	trimmed := append([]byte(nil), text...)
	for len(trimmed) > 0 && trimmed[len(trimmed)-1] == ' ' {
		trimmed = trimmed[:len(trimmed)-1]
	}
	if len(trimmed) == 0 {
		return
	}
	x := (charsWide - len(trimmed)) * consoleCharWidth / 2
	for i, ch := range trimmed {
		if shouldDrawNotifyChar(lineIndex, i, alpha) {
			rc.DrawCharacter(x+i*consoleCharWidth, y, int(ch))
		}
	}
}

// drawRuneText renders a slice of runes as a horizontal row of characters.
// Runes outside the 0–255 range (i.e. characters that don't exist in Quake's
// bitmap font) are replaced with '?' to avoid out-of-bounds glyph lookups.
// This is used for the input line, which is stored as []rune to support
// correct backspacing of multi-byte UTF-8 characters.
func drawRuneText(rc DrawContext, x, y int, text []rune) {
	if rc == nil {
		return
	}
	for i, ch := range text {
		num := int(ch)
		if num < 0 || num > 255 {
			num = int('?')
		}
		rc.DrawCharacter(x+i*consoleCharWidth, y, num)
	}
}

// clipPrompt truncates the input prompt to fit within maxChars columns. When
// the user's input is longer than the visible area, the prompt is clipped
// from the left (keeping the most recently typed characters visible) and the
// leading ']' prompt character is re-prepended. This mimics the behaviour of
// the original Quake console where long commands scroll the input area.
func clipPrompt(prompt []rune, maxChars int) []rune {
	if maxChars <= 0 {
		return nil
	}
	if len(prompt) <= maxChars {
		return prompt
	}
	clipped := make([]rune, 0, maxChars)
	clipped = append(clipped, ']')
	clipped = append(clipped, prompt[len(prompt)-(maxChars-1):]...)
	return clipped
}

func clipPromptWithCursor(prompt []rune, cursor, maxChars int) ([]rune, int) {
	if maxChars <= 0 {
		return nil, 0
	}
	if cursor < 1 {
		cursor = 1
	}
	if cursor > len(prompt) {
		cursor = len(prompt)
	}
	if len(prompt) <= maxChars {
		return prompt, cursor
	}
	if maxChars == 1 {
		return []rune{']'}, 1
	}

	content := prompt[1:]
	contentCursor := cursor - 1
	if contentCursor < 0 {
		contentCursor = 0
	}
	window := maxChars - 1
	start := 0
	if contentCursor > window {
		start = contentCursor - window
	}
	maxStart := len(content) - window
	if maxStart < 0 {
		maxStart = 0
	}
	if start > maxStart {
		start = maxStart
	}
	visible := make([]rune, 0, maxChars)
	visible = append(visible, ']')
	visible = append(visible, content[start:start+window]...)
	cursorCol := 1 + contentCursor - start
	if cursorCol < 1 {
		cursorCol = 1
	}
	if cursorCol > len(visible) {
		cursorCol = len(visible)
	}
	return visible, cursorCol
}

func drawBlinkCursor(rc DrawContext, x, y int, now time.Time) {
	if rc == nil {
		return
	}
	rc.DrawCharacter(x, y, consoleCursorGlyph(now))
}

func consoleCursorGlyph(now time.Time) int {
	frame := (now.UnixNano() / int64(time.Second/consoleCursorBlinkHz)) & 1
	return 10 + int(frame)
}

func consoleNotifyTTL() time.Duration {
	secs := cvar.FloatValue("con_notifytime")
	if secs <= 0 {
		return consoleNotifyDefaultTTL
	}
	return time.Duration(secs * float64(time.Second))
}

func notifyFadeDuration() time.Duration {
	if !cvar.BoolValue("con_notifyfade") {
		return 0
	}
	secs := cvar.FloatValue("con_notifyfadetime")
	if secs <= 0 {
		return 0
	}
	return time.Duration(secs * float64(time.Second))
}

func notifyAlpha(now, ts time.Time) float64 {
	if ts.IsZero() {
		return 0
	}
	fade := notifyFadeDuration()
	remaining := ts.Add(consoleNotifyTTL() + fade).Sub(now)
	if remaining <= 0 {
		return 0
	}
	if fade <= 0 || remaining >= fade {
		return 1
	}
	return float64(remaining) / float64(fade)
}

func shouldDrawNotifyChar(lineIndex, charIndex int, alpha float64) bool {
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

func (c *Console) NotifyLineCountAt(now time.Time) int {
	if c == nil {
		return 0
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	count := 0
	for line := c.current - NumNotifyTimes + 1; line <= c.current; line++ {
		if line < 0 {
			continue
		}
		if notifyAlpha(now, c.notifyTimes[line%NumNotifyTimes]) > 0 {
			count++
		}
	}
	return count
}

// max returns the larger of two ints. This is a local helper because Go
// versions prior to 1.21 did not provide a generic max built-in, and this
// package targets broad compatibility.
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
