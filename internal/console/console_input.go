// Package console implements the Quake console system.
package console

import "strings"

func (c *Console) InputLine() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return string(c.inputLine)
}

func (c *Console) SetInputLine(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inputLine = []rune(text)
	c.cursorPos = len(c.inputLine)
	c.historyPos = len(c.history)
}

func (c *Console) AppendInputRune(ch rune) {
	if ch == '\n' || ch == '\r' || ch == '\t' || ch < 32 {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if c.cursorPos < 0 || c.cursorPos > len(c.inputLine) {
		c.cursorPos = len(c.inputLine)
	}
	if c.insertMode || c.cursorPos == len(c.inputLine) {
		c.inputLine = append(c.inputLine[:c.cursorPos], append([]rune{ch}, c.inputLine[c.cursorPos:]...)...)
	} else {
		c.inputLine[c.cursorPos] = ch
	}
	c.cursorPos++
	c.historyPos = len(c.history)
}

func (c *Console) BackspaceInput() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.inputLine) == 0 || c.cursorPos <= 0 {
		return
	}
	start := c.cursorPos - 1
	c.inputLine = append(c.inputLine[:start], c.inputLine[c.cursorPos:]...)
	c.cursorPos = start
	c.historyPos = len(c.history)
}

func (c *Console) DeleteInput() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cursorPos < 0 || c.cursorPos >= len(c.inputLine) {
		return
	}
	c.inputLine = append(c.inputLine[:c.cursorPos], c.inputLine[c.cursorPos+1:]...)
	c.historyPos = len(c.history)
}

func (c *Console) MoveCursorLeft(ctrl bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cursorPos <= 0 {
		c.cursorPos = 0
		return
	}
	if ctrl {
		c.cursorPos = wordBoundaryLeft(c.inputLine, c.cursorPos)
		return
	}
	c.cursorPos--
}

func (c *Console) MoveCursorRight(ctrl bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cursorPos >= len(c.inputLine) {
		c.cursorPos = len(c.inputLine)
		return
	}
	if ctrl {
		c.cursorPos = wordBoundaryRight(c.inputLine, c.cursorPos)
		return
	}
	c.cursorPos++
}

func (c *Console) MoveCursorStart() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cursorPos = 0
}

func (c *Console) MoveCursorEnd() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cursorPos = len(c.inputLine)
}

func (c *Console) CursorPos() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if c.cursorPos < 0 {
		return 0
	}
	if c.cursorPos > len(c.inputLine) {
		return len(c.inputLine)
	}
	return c.cursorPos
}

func (c *Console) ToggleInsertMode() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.insertMode = !c.insertMode
	return c.insertMode
}

func (c *Console) DeleteWordLeft() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cursorPos <= 0 {
		return
	}
	start := wordBoundaryLeft(c.inputLine, c.cursorPos)
	c.inputLine = append(c.inputLine[:start], c.inputLine[c.cursorPos:]...)
	c.cursorPos = start
	c.historyPos = len(c.history)
}

func (c *Console) DeleteWordRight() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cursorPos >= len(c.inputLine) {
		return
	}
	end := wordBoundaryRight(c.inputLine, c.cursorPos)
	c.inputLine = append(c.inputLine[:c.cursorPos], c.inputLine[end:]...)
	c.historyPos = len(c.history)
}

func (c *Console) CommitInput() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	line := string(c.inputLine)
	trimmed := strings.TrimSpace(line)
	if trimmed != "" {
		if len(c.history) == 0 || c.history[len(c.history)-1] != line {
			c.history = append(c.history, line)
		}
		if len(c.history) > MaxInputHistory {
			c.history = append([]string(nil), c.history[len(c.history)-MaxInputHistory:]...)
		}
	}

	c.inputLine = nil
	c.cursorPos = 0
	c.historyPos = len(c.history)
	c.historyBackup = nil
	return line
}

func (c *Console) PreviousHistory() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.history) == 0 {
		return string(c.inputLine)
	}
	if c.historyPos == len(c.history) {
		c.historyBackup = append([]rune(nil), c.inputLine...)
	}
	if c.historyPos > 0 {
		c.historyPos--
	}
	c.inputLine = []rune(c.history[c.historyPos])
	c.cursorPos = len(c.inputLine)
	return string(c.inputLine)
}

func (c *Console) NextHistory() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.history) == 0 {
		c.inputLine = nil
		c.cursorPos = 0
		c.historyPos = 0
		return ""
	}
	if c.historyPos < len(c.history)-1 {
		c.historyPos++
		c.inputLine = []rune(c.history[c.historyPos])
		c.cursorPos = len(c.inputLine)
		return string(c.inputLine)
	}
	c.historyPos = len(c.history)
	c.inputLine = append([]rune(nil), c.historyBackup...)
	c.cursorPos = len(c.inputLine)
	return string(c.inputLine)
}

func cursorClamp(pos, n int) int {
	if pos < 0 {
		return 0
	}
	if pos > n {
		return n
	}
	return pos
}

func wordBoundaryLeft(line []rune, pos int) int {
	pos = cursorClamp(pos, len(line))
	if pos == 0 {
		return 0
	}
	i := pos - 1
	for i > 0 && line[i] == ' ' {
		i--
	}
	for i > 0 && line[i-1] != ' ' {
		i--
	}
	return i
}

func wordBoundaryRight(line []rune, pos int) int {
	pos = cursorClamp(pos, len(line))
	if pos >= len(line) {
		return len(line)
	}
	i := pos
	for i < len(line) && line[i] == ' ' {
		i++
	}
	for i < len(line) && line[i] != ' ' {
		i++
	}
	return i
}

func InputLine() string         { return globalConsole.InputLine() }
func SetInputLine(text string)  { globalConsole.SetInputLine(text) }
func AppendInputRune(ch rune)   { globalConsole.AppendInputRune(ch) }
func BackspaceInput()           { globalConsole.BackspaceInput() }
func DeleteInput()              { globalConsole.DeleteInput() }
func MoveCursorLeft(ctrl bool)  { globalConsole.MoveCursorLeft(ctrl) }
func MoveCursorRight(ctrl bool) { globalConsole.MoveCursorRight(ctrl) }
func MoveCursorStart()          { globalConsole.MoveCursorStart() }
func MoveCursorEnd()            { globalConsole.MoveCursorEnd() }
func CursorPos() int            { return globalConsole.CursorPos() }
func ToggleInsertMode() bool    { return globalConsole.ToggleInsertMode() }
func DeleteWordLeft()           { globalConsole.DeleteWordLeft() }
func DeleteWordRight()          { globalConsole.DeleteWordRight() }
func CommitInput() string       { return globalConsole.CommitInput() }
func PreviousHistory() string   { return globalConsole.PreviousHistory() }
func NextHistory() string       { return globalConsole.NextHistory() }
func NotifyLineCount() int      { return globalConsole.NotifyLineCountAt(consoleNow()) }
