// Package console implements the Quake console system.
//
// The console provides a text-based interface for debugging, configuration,
// and command execution. It maintains a scrollable text buffer that can be
// displayed over the game screen, and supports:
//   - Printf-style formatted output with multiple severity levels
//   - Scrollback history with configurable buffer size
//   - Tab completion for commands and variables
//   - Mouse selection and copy to clipboard
//   - Clickable links (for file paths, URLs, etc.)
//   - Notification display for important messages
//   - Logging to file for debugging
//
// This implementation focuses on the core console logic (buffer management,
// printing, logging) without rendering dependencies. Drawing functions are
// implemented separately in the render package.
package console

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	DefaultTextSize  = 1024 * 1024
	MinTextSize      = 16384
	DefaultLineWidth = 78
	NumNotifyTimes   = 4
	MaxPrintMsg      = 4096
	Margin           = 1
	MaxInputHistory  = 32
)

type Console struct {
	mu sync.RWMutex

	text       []byte
	bufSize    int
	lineWidth  int
	totalLines int

	current    int
	x          int
	backScroll int

	notifyTimes [NumNotifyTimes]time.Time

	initialized bool
	debugLog    *os.File
	logFile     string

	inputLine  []rune
	history    []string
	historyPos int

	printCallback func(msg string)
}

var globalConsole = NewConsole(DefaultTextSize)

func NewConsole(bufSize int) *Console {
	if bufSize < MinTextSize {
		bufSize = MinTextSize
	}

	c := &Console{
		bufSize:   bufSize,
		lineWidth: DefaultLineWidth,
	}

	return c
}

func (c *Console) Init(customBufSize int) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	if customBufSize > 0 {
		if customBufSize < MinTextSize {
			customBufSize = MinTextSize
		}
		c.bufSize = customBufSize
	}

	c.text = make([]byte, c.bufSize)
	for i := range c.text {
		c.text[i] = ' '
	}

	c.lineWidth = DefaultLineWidth
	c.totalLines = c.bufSize / c.lineWidth
	c.current = c.totalLines - 1
	c.x = 0
	c.backScroll = 0
	c.initialized = true
	c.historyPos = 0

	return nil
}

func InitGlobal(bufSize int) error {
	return globalConsole.Init(bufSize)
}

func (c *Console) LineWidth() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lineWidth
}

func (c *Console) TotalLines() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.totalLines
}

func (c *Console) CurrentLine() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

func (c *Console) BackScroll() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.backScroll
}

func (c *Console) SetBackScroll(lines int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	maxScroll := c.totalLines - 10
	if maxScroll < 0 {
		maxScroll = 0
	}
	if lines > maxScroll {
		lines = maxScroll
	}
	if lines < 0 {
		lines = 0
	}
	c.backScroll = lines
}

func (c *Console) Scroll(lines int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if lines == 0 {
		return
	}

	c.backScroll += lines

	maxScroll := c.totalLines - 10
	if maxScroll < 0 {
		maxScroll = 0
	}

	if c.backScroll > maxScroll {
		c.backScroll = maxScroll
	}
	if c.backScroll < 0 {
		c.backScroll = 0
	}
}

func (c *Console) Resize(newWidth int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if newWidth == c.lineWidth || newWidth <= 0 {
		return
	}

	oldWidth := c.lineWidth
	oldTotalLines := c.totalLines
	oldCurrent := c.current
	oldNotifyTimes := c.notifyTimes
	c.lineWidth = newWidth
	c.totalLines = c.bufSize / c.lineWidth

	numLines := oldTotalLines
	if c.totalLines < numLines {
		numLines = c.totalLines
	}

	tmp := make([]byte, c.bufSize)
	copy(tmp, c.text)

	for i := range c.text {
		c.text[i] = ' '
	}

	numChars := oldWidth
	if c.lineWidth < numChars {
		numChars = c.lineWidth
	}

	for i := 0; i < numLines; i++ {
		srcLine := (c.current - i + oldTotalLines) % oldTotalLines
		dstLine := c.totalLines - 1 - i
		if dstLine < 0 {
			break
		}
		for j := 0; j < numChars; j++ {
			c.text[dstLine*c.lineWidth+j] = tmp[srcLine*oldWidth+j]
		}
	}

	c.current = c.totalLines - 1
	c.backScroll = 0

	for i := range c.notifyTimes {
		c.notifyTimes[i] = time.Time{}
	}
	for i := 0; i < NumNotifyTimes; i++ {
		srcLine := oldCurrent - i
		dstLine := c.current - i
		if srcLine < 0 || dstLine < 0 {
			break
		}
		c.notifyTimes[dstLine%NumNotifyTimes] = oldNotifyTimes[srcLine%NumNotifyTimes]
	}
}

func (c *Console) GetLine(lineNum int) string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if lineNum > c.current {
		return ""
	}

	line := lineNum % c.totalLines
	if line < 0 {
		line += c.totalLines
	}

	start := line * c.lineWidth
	if start < 0 || start >= len(c.text) {
		return ""
	}

	end := start + c.lineWidth
	if end > len(c.text) {
		end = len(c.text)
	}

	text := c.text[start:end]
	for len(text) > 0 && text[len(text)-1] == ' ' {
		text = text[:len(text)-1]
	}

	return string(text)
}

func (c *Console) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.text != nil {
		for i := range c.text {
			c.text[i] = ' '
		}
	}

	c.current = c.totalLines - 1
	c.x = 0
	c.backScroll = 0
	c.inputLine = nil
	c.historyPos = len(c.history)

	for i := range c.notifyTimes {
		c.notifyTimes[i] = time.Time{}
	}
}

func (c *Console) InputLine() string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return string(c.inputLine)
}

func (c *Console) AppendInputRune(ch rune) {
	if ch == '\n' || ch == '\r' || ch == '\t' || ch < 32 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.inputLine = append(c.inputLine, ch)
	c.historyPos = len(c.history)
}

func (c *Console) BackspaceInput() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.inputLine) == 0 {
		return
	}
	c.inputLine = c.inputLine[:len(c.inputLine)-1]
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
			if len(c.history) > MaxInputHistory {
				c.history = append([]string(nil), c.history[len(c.history)-MaxInputHistory:]...)
			}
		}
	}
	c.inputLine = nil
	c.historyPos = len(c.history)
	return line
}

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

func (c *Console) lineFeed() {
	if c.backScroll > 0 {
		c.backScroll++
		if c.backScroll > c.totalLines-10 {
			c.backScroll = c.totalLines - 10
		}
	}

	c.x = 0
	c.current++

	line := c.current % c.totalLines
	start := line * c.lineWidth
	end := start + c.lineWidth
	if end > len(c.text) {
		end = len(c.text)
	}
	for i := start; i < end; i++ {
		c.text[i] = ' '
	}
}

func (c *Console) printRaw(txt string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized || c.text == nil {
		return
	}

	var cr bool
	mask := byte(0)

	if len(txt) > 0 {
		if txt[0] == 1 || txt[0] == 2 {
			mask = 128
			txt = txt[1:]
		}
	}

	txt = strings.TrimPrefix(txt, "[skipnotify]")

	for len(txt) > 0 {
		ch := txt[0]
		txt = txt[1:]

		if cr {
			c.current--
			cr = false
		}

		if c.x == 0 {
			c.lineFeed()
			if len(c.notifyTimes) > 0 {
				c.notifyTimes[c.current%NumNotifyTimes] = time.Now()
			}
		}

		switch ch {
		case '\n':
			c.x = 0
		case '\r':
			c.x = 0
			cr = true
		default:
			line := c.current % c.totalLines
			pos := line*c.lineWidth + c.x
			if pos < len(c.text) {
				c.text[pos] = ch | mask
			}
			c.x++
			if c.x >= c.lineWidth {
				c.x = 0
			}
		}
	}
}

func (c *Console) Printf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	c.printRaw(msg)

	if c.printCallback != nil {
		c.printCallback(msg)
	}

	c.debugLogWrite(msg)
}

func (c *Console) DPrintf(developer bool, format string, args ...interface{}) {
	if !developer {
		return
	}
	msg := fmt.Sprintf(format, args...)
	c.printRaw(msg)
	c.debugLogWrite(msg)
}

func (c *Console) Warning(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	c.Printf("\x02Warning: %s", msg)
}

func (c *Console) DWarning(developer bool, format string, args ...interface{}) {
	if !developer {
		return
	}
	msg := fmt.Sprintf(format, args...)
	c.Printf("\x02Warning: %s", msg)
}

func (c *Console) SafePrintf(format string, args ...interface{}) {
	c.Printf(format, args...)
}

func (c *Console) CenterPrintf(width int, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	lines := strings.Split(msg, "\n")

	for _, line := range lines {
		if len(line) == 0 {
			c.Printf("\n")
			continue
		}

		lineWidth := utf8.RuneCountInString(line)
		if lineWidth < width {
			padding := (width - lineWidth) / 2
			c.Printf("%s%s\n", strings.Repeat(" ", padding), line)
		} else {
			c.Printf("%s\n", line)
		}
	}
}

func QuakeBar(length int) string {
	if length > 40 {
		length = 40
	}
	if length < 2 {
		length = 2
	}

	result := make([]byte, length+1)
	result[0] = '\x35'
	for i := 1; i < length-1; i++ {
		result[i] = '\x36'
	}
	result[length-1] = '\x37'
	result[length] = '\n'

	return string(result[:length+1])
}

func (c *Console) SetPrintCallback(fn func(msg string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.printCallback = fn
}

func (c *Console) debugLogWrite(msg string) {
	if c.debugLog != nil {
		c.debugLog.WriteString(msg)
	}
}

func (c *Console) EnableDebugLog(filename string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.debugLog != nil {
		c.debugLog.Close()
	}

	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	f, err := os.Create(filename)
	if err != nil {
		return err
	}

	c.debugLog = f
	c.logFile = filename

	timestamp := time.Now().Format("01/02/2006 15:04:05")
	fmt.Fprintf(f, "LOG started on: %s\n", timestamp)

	return nil
}

func (c *Console) DisableDebugLog() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.debugLog != nil {
		c.debugLog.Close()
		c.debugLog = nil
	}
}

func (c *Console) Close() {
	c.DisableDebugLog()
}

func (c *Console) NotifyTimes() [NumNotifyTimes]time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.notifyTimes
}

func (c *Console) ClearNotify() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.notifyTimes {
		c.notifyTimes[i] = time.Time{}
	}
}

func Printf(format string, args ...interface{}) {
	globalConsole.Printf(format, args...)
}

func DPrintf(developer bool, format string, args ...interface{}) {
	globalConsole.DPrintf(developer, format, args...)
}

func Warning(format string, args ...interface{}) {
	globalConsole.Warning(format, args...)
}

func DWarning(developer bool, format string, args ...interface{}) {
	globalConsole.DWarning(developer, format, args...)
}

func SafePrintf(format string, args ...interface{}) {
	globalConsole.SafePrintf(format, args...)
}

func CenterPrintf(width int, format string, args ...interface{}) {
	globalConsole.CenterPrintf(width, format, args...)
}

func Clear() {
	globalConsole.Clear()
}

func Scroll(lines int) {
	globalConsole.Scroll(lines)
}

func Resize(newWidth int) {
	globalConsole.Resize(newWidth)
}

func GetLine(lineNum int) string {
	return globalConsole.GetLine(lineNum)
}

func Close() {
	globalConsole.Close()
}

func EnableDebugLog(filename string) error {
	return globalConsole.EnableDebugLog(filename)
}

func DisableDebugLog() {
	globalConsole.DisableDebugLog()
}

func SetPrintCallback(fn func(msg string)) {
	globalConsole.SetPrintCallback(fn)
}

func CurrentLine() int {
	return globalConsole.CurrentLine()
}

func LineWidth() int {
	return globalConsole.LineWidth()
}

func TotalLines() int {
	return globalConsole.TotalLines()
}

func InputLine() string {
	return globalConsole.InputLine()
}

func AppendInputRune(ch rune) {
	globalConsole.AppendInputRune(ch)
}

func BackspaceInput() {
	globalConsole.BackspaceInput()
}

func CommitInput() string {
	return globalConsole.CommitInput()
}

func PreviousHistory() string {
	return globalConsole.PreviousHistory()
}

func NextHistory() string {
	return globalConsole.NextHistory()
}
