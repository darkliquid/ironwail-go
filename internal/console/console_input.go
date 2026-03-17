// Package console implements the Quake console system.
//
// This file implements console input editing and history APIs.
package console

import "strings"

// InputLine returns the current contents of the user's input line as a string.
// The input line is the editable text at the bottom of the console where the
// user types commands.
func (c *Console) InputLine() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return string(c.inputLine)
}

// SetInputLine replaces the entire input line. This is used by tab completion
// and history recall to overwrite whatever the user had typed. Setting the
// input also resets the history cursor to the end (a fresh position).

// SetInputLine replaces the entire input line. This is used by tab completion
// and history recall to overwrite whatever the user had typed. Setting the
// input also resets the history cursor to the end (a fresh position).
func (c *Console) SetInputLine(text string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inputLine = []rune(text)
	c.historyPos = len(c.history)
}

// AppendInputRune appends a single character to the input line. Control
// characters (newline, carriage return, tab, and anything below ASCII 32) are
// silently ignored — those are handled by dedicated key handlers (CommitInput
// for Enter, BackspaceInput for backspace, etc.).

// AppendInputRune appends a single character to the input line. Control
// characters (newline, carriage return, tab, and anything below ASCII 32) are
// silently ignored — those are handled by dedicated key handlers (CommitInput
// for Enter, BackspaceInput for backspace, etc.).
func (c *Console) AppendInputRune(ch rune) {
	if ch == '\n' || ch == '\r' || ch == '\t' || ch < 32 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inputLine = append(c.inputLine, ch)
	c.historyPos = len(c.history)
}

// BackspaceInput removes the last rune from the input line, implementing the
// Backspace key. It is a no-op if the line is already empty.

// BackspaceInput removes the last rune from the input line, implementing the
// Backspace key. It is a no-op if the line is already empty.
func (c *Console) BackspaceInput() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.inputLine) == 0 {
		return
	}
	c.inputLine = c.inputLine[:len(c.inputLine)-1]
	c.historyPos = len(c.history)
}

// CommitInput finalises the current input line (the user pressed Enter).
// It saves the line to the command history (deduplicating consecutive
// identical entries and evicting the oldest when the history is full),
// clears the input line, and returns the committed text so the caller can
// pass it to the command system for execution.

// CommitInput finalises the current input line (the user pressed Enter).
// It saves the line to the command history (deduplicating consecutive
// identical entries and evicting the oldest when the history is full),
// clears the input line, and returns the committed text so the caller can
// pass it to the command system for execution.
func (c *Console) CommitInput() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	line := string(c.inputLine)
	trimmed := strings.TrimSpace(line)
	if trimmed != "" {
		if len(c.history) == 0 || c.history[len(c.history)-1] != line {
			c.history = append(c.history, line)
			if len(c.history) > MaxInputHistory {
				c.history = append([]string(nil), c.history[len(c.history)-MaxInputHistory:]...)
			}
		}
	}
	c.inputLine = nil
	c.historyPos = len(c.history)
	return line
}

// PreviousHistory moves the history cursor one entry backward (toward older
// commands) and replaces the input line with that entry, implementing the
// Up arrow key. If already at the oldest entry, it stays there.

// PreviousHistory moves the history cursor one entry backward (toward older
// commands) and replaces the input line with that entry, implementing the
// Up arrow key. If already at the oldest entry, it stays there.
func (c *Console) PreviousHistory() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.history) == 0 {
		return string(c.inputLine)
	}
	if c.historyPos > 0 {
		c.historyPos--
	}
	c.inputLine = []rune(c.history[c.historyPos])
	return string(c.inputLine)
}

// NextHistory moves the history cursor one entry forward (toward newer
// commands) and replaces the input line, implementing the Down arrow key.
// Moving past the newest entry clears the input line (returns to a fresh
// prompt).

// NextHistory moves the history cursor one entry forward (toward newer
// commands) and replaces the input line, implementing the Down arrow key.
// Moving past the newest entry clears the input line (returns to a fresh
// prompt).
func (c *Console) NextHistory() string {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.history) == 0 {
		c.inputLine = nil
		c.historyPos = 0
		return ""
	}
	if c.historyPos < len(c.history)-1 {
		c.historyPos++
		c.inputLine = []rune(c.history[c.historyPos])
		return string(c.inputLine)
	}
	c.historyPos = len(c.history)
	c.inputLine = nil
	return ""
}

// lineFeed advances the write cursor to the next line in the ring buffer.
// If the user is scrolled up (backScroll > 0), the scroll position is
// incremented to keep the viewport stable — otherwise new output would
// cause the visible text to "jump". The new line is filled with spaces
// (the Quake "empty character" convention).

// InputLine returns the current input text from the global console.
func InputLine() string {
	return globalConsole.InputLine()
}

// SetInputLine replaces the global console's input line.

// SetInputLine replaces the global console's input line.
func SetInputLine(text string) {
	globalConsole.SetInputLine(text)
}

// AppendInputRune appends a character to the global console's input line.

// AppendInputRune appends a character to the global console's input line.
func AppendInputRune(ch rune) {
	globalConsole.AppendInputRune(ch)
}

// BackspaceInput removes the last character from the global console's input.

// BackspaceInput removes the last character from the global console's input.
func BackspaceInput() {
	globalConsole.BackspaceInput()
}

// CommitInput finalises the global console's input line and returns it.

// CommitInput finalises the global console's input line and returns it.
func CommitInput() string {
	return globalConsole.CommitInput()
}

// PreviousHistory recalls an older command in the global console's history.

// PreviousHistory recalls an older command in the global console's history.
func PreviousHistory() string {
	return globalConsole.PreviousHistory()
}

// NextHistory recalls a newer command in the global console's history.

// NextHistory recalls a newer command in the global console's history.
func NextHistory() string {
	return globalConsole.NextHistory()
}

// ---------------------------------------------------------------------------
// Package-level convenience functions
// ---------------------------------------------------------------------------
// The functions below delegate to the global singleton Console instance
// (globalConsole). They exist so that the vast majority of the engine can
// simply call  console.Printf(...)  without needing to carry a *Console
// pointer through every subsystem. This mirrors the original Quake C API
// where Con_Printf was a global function.
// ---------------------------------------------------------------------------

// Printf formats and prints a message to the global console.
func Printf(format string, args ...interface{}) {
	globalConsole.Printf(format, args...)
}

// DPrintf prints a developer-only message to the global console.
func DPrintf(developer bool, format string, args ...interface{}) {
	globalConsole.DPrintf(developer, format, args...)
}

// Warning prints a bronze-coloured warning to the global console.
func Warning(format string, args ...interface{}) {
	globalConsole.Warning(format, args...)
}

// DWarning prints a developer-only warning to the global console.
func DWarning(developer bool, format string, args ...interface{}) {
	globalConsole.DWarning(developer, format, args...)
}

// SafePrintf prints to the global console (see Console.SafePrintf for details).
func SafePrintf(format string, args ...interface{}) {
	globalConsole.SafePrintf(format, args...)
}

// CenterPrintf prints centred text to the global console.
func CenterPrintf(width int, format string, args ...interface{}) {
	globalConsole.CenterPrintf(width, format, args...)
}

// Clear wipes the global console scrollback.
func Clear() {
	globalConsole.Clear()
}

// Scroll adjusts the global console's scrollback position.
func Scroll(lines int) {
	globalConsole.Scroll(lines)
}

// Resize adjusts the global console's line width.
func Resize(newWidth int) {
	globalConsole.Resize(newWidth)
}

// GetLine retrieves a line from the global console's scrollback.
func GetLine(lineNum int) string {
	return globalConsole.GetLine(lineNum)
}

// Close shuts down the global console (closes debug log, etc.).
func Close() {
	globalConsole.Close()
}

// EnableDebugLog opens a debug log file for the global console.
func EnableDebugLog(filename string) error {
	return globalConsole.EnableDebugLog(filename)
}

// DisableDebugLog closes the global console's debug log file.
func DisableDebugLog() {
	globalConsole.DisableDebugLog()
}

// SetPrintCallback registers a print observer on the global console.
func SetPrintCallback(fn func(msg string)) {
	globalConsole.SetPrintCallback(fn)
}

// CurrentLine returns the most recent line index from the global console.
func CurrentLine() int {
	return globalConsole.CurrentLine()
}

// LineWidth returns the characters-per-line of the global console.
func LineWidth() int {
	return globalConsole.LineWidth()
}

// TotalLines returns the total line capacity of the global console.
func TotalLines() int {
	return globalConsole.TotalLines()
}
