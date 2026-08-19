package console

// FullConsoleSnapshot holds a point-in-time capture of the drop-down console state.
type FullConsoleSnapshot struct {
	Lines      []string
	InputLine  string
	CursorPos  int
	BackScroll int
	Title      string
}

// NotifyLineSnapshot represents a single active notify line and its opacity.
type NotifyLineSnapshot struct {
	Text  string
	Alpha float32
}

// SnapshotFull captures the visible scrollback lines and input state for the given row count.
func (c *Console) SnapshotFull(visibleRows int) FullConsoleSnapshot {
	if c == nil {
		return FullConsoleSnapshot{}
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	if visibleRows < 1 {
		visibleRows = 1
	}
	bottomLine := c.current - c.backScroll
	startLine := bottomLine - (visibleRows - 1)
	lines := make([]string, 0, visibleRows)
	for line := startLine; line <= bottomLine; line++ {
		lines = append(lines, string(c.lineBytesLocked(line)))
	}

	return FullConsoleSnapshot{
		Lines:      lines,
		InputLine:  string(c.inputLine),
		CursorPos:  c.cursorPos,
		BackScroll: c.backScroll,
		Title:      c.Title,
	}
}

// SnapshotNotify captures currently active fading notification lines.
func (c *Console) SnapshotNotify() []NotifyLineSnapshot {
	if c == nil {
		return nil
	}
	now := c.now()
	c.mu.RLock()
	defer c.mu.RUnlock()

	lines := make([]NotifyLineSnapshot, 0, NumNotifyTimes)
	for line := c.current - NumNotifyTimes + 1; line <= c.current; line++ {
		if line < 0 {
			continue
		}
		ts := c.notifyTimes[line%NumNotifyTimes]
		alpha := c.notifyAlpha(now, ts)
		if alpha <= 0 {
			continue
		}
		lines = append(lines, NotifyLineSnapshot{
			Text:  string(c.lineBytesLocked(line)),
			Alpha: float32(alpha),
		})
	}
	return lines
}
