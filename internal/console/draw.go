package console

import (
	"strings"
	"time"
)

type DrawContext interface {
	DrawFill(x, y, w, h int, color byte)
	DrawCharacter(x, y int, num int)
}

const (
	consoleCharWidth     = 8
	consoleCharHeight    = 8
	consoleNotifyTTL     = 3 * time.Second
	consoleMinDrawHeight = consoleCharHeight * 4
)

// Draw renders either the full console overlay or the compact notify lines.
func Draw(rc DrawContext, screenWidth, screenHeight int, full bool) {
	globalConsole.Draw(rc, screenWidth, screenHeight, full)
}

// Draw renders either the full console overlay or the compact notify lines.
func (c *Console) Draw(rc DrawContext, screenWidth, screenHeight int, full bool) {
	if c == nil || rc == nil || screenWidth < consoleCharWidth || screenHeight < consoleCharHeight {
		return
	}

	charsWide := screenWidth / consoleCharWidth
	if charsWide <= 0 {
		return
	}
	c.Resize(charsWide)

	if full {
		c.drawFull(rc, screenWidth, screenHeight, charsWide)
		return
	}
	c.drawNotify(rc, charsWide)
}

func (c *Console) drawFull(rc DrawContext, screenWidth, screenHeight, charsWide int) {
	consoleHeight := screenHeight / 2
	if consoleHeight < consoleMinDrawHeight {
		consoleHeight = screenHeight
	}
	if consoleHeight <= 0 {
		return
	}

	rc.DrawFill(0, 0, screenWidth, consoleHeight, 0)

	c.mu.RLock()
	current := c.current
	backScroll := c.backScroll
	inputLine := append([]rune(nil), c.inputLine...)
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
	drawRuneText(rc, consoleCharWidth, consoleHeight-consoleCharHeight, clipPrompt(prompt, charsWide-2))
}

func (c *Console) drawNotify(rc DrawContext, charsWide int) {
	now := time.Now()

	c.mu.RLock()
	current := c.current
	notifyTimes := c.notifyTimes
	lines := make([][]byte, 0, NumNotifyTimes)
	for line := current - NumNotifyTimes + 1; line <= current; line++ {
		if line < 0 {
			continue
		}
		ts := notifyTimes[line%NumNotifyTimes]
		if ts.IsZero() || now.Sub(ts) > consoleNotifyTTL {
			continue
		}
		lines = append(lines, c.lineBytesLocked(line))
	}
	c.mu.RUnlock()

	y := 0
	for _, line := range lines {
		drawByteText(rc, consoleCharWidth, y, line, charsWide-2)
		y += consoleCharHeight
	}
}

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

func drawByteText(rc DrawContext, x, y int, text []byte, maxChars int) {
	if rc == nil || maxChars == 0 {
		return
	}
	if maxChars > 0 && len(text) > maxChars {
		text = text[:maxChars]
	}
	for i, ch := range text {
		rc.DrawCharacter(x+i*consoleCharWidth, y, int(ch))
	}
}

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

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
