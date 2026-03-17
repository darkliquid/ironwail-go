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

// Console is the core state for the Quake-style drop-down developer console.
//
// Architecturally, the console serves as the bridge between the human operator
// and the engine's command system (cmdsys) and console variable (cvar) system.
// It owns:
//   - A scrollback buffer — a ring buffer of fixed-width text lines stored as
//     a flat []byte slice. Each line occupies exactly lineWidth bytes, and the
//     buffer wraps when "current" exceeds totalLines.
//   - An input line with cursor — a slice of runes the user is currently typing.
//   - A command history — remembering previously entered commands so the user
//     can recall them with the up/down arrow keys.
//   - Notification timestamps — tracking when each of the most recent lines was
//     printed so the renderer can display them briefly during gameplay.
//
// The struct is safe for concurrent use: a sync.RWMutex guards all mutable
// state so that the game loop's rendering goroutine can read the buffer while
// the main thread writes to it.
type Console struct {
	// mu protects every field below. Writers take a full Lock; readers (draw,
	// accessors) take RLock for minimal contention.
	mu sync.RWMutex

	// text is the flat ring buffer backing the scrollback. It is allocated once
	// during Init and never re-allocated. Characters are stored as raw bytes
	// (with Quake's high-bit colour convention — setting bit 7 makes a char
	// display in the alternate "bronze" colour).
	text []byte

	// bufSize is the total capacity of text in bytes. Together with lineWidth
	// it determines totalLines = bufSize / lineWidth.
	bufSize int

	// lineWidth is the current number of characters per console line. It is
	// adjusted at runtime via Resize when the screen resolution changes.
	lineWidth int

	// totalLines is the total number of lines that fit in the ring buffer.
	totalLines int

	// current is the index of the most recently written line (the "head" of
	// the ring). It only ever increments; modular arithmetic wraps it into
	// the buffer range.
	current int

	// x is the column position within the current line where the next
	// character will be written. When x reaches lineWidth, a new line begins.
	x int

	// backScroll is how many lines the user has scrolled upward from the
	// bottom of the console. 0 means "viewing the latest output".
	backScroll int

	// notifyTimes records the wall-clock time at which each of the most recent
	// NumNotifyTimes lines was printed. The draw code uses these to decide
	// whether a line should still be visible in the notify area.
	notifyTimes [NumNotifyTimes]time.Time

	// initialized guards one-time setup of the text buffer.
	initialized bool

	// debugLog is an optional file handle for writing all console output to
	// disk. Used via the "condebug" console command for debugging sessions.
	debugLog *os.File

	// logFile stores the path of the currently-open debug log file.
	logFile string

	// inputLine holds the runes the user is currently typing at the bottom of
	// the console. Using []rune (not []byte) so that multibyte characters are
	// handled correctly when backspacing or measuring width.
	inputLine []rune

	// history stores previously committed input lines. Up/down arrow keys
	// navigate this list.
	history []string

	// historyPos is the cursor into history. When equal to len(history) the
	// user is editing a fresh (empty) line; otherwise it indexes a recalled
	// entry.
	historyPos int

	// printCallback is an optional hook invoked after every Printf call.
	// External systems (e.g. a network broadcast or a GUI widget) can register
	// here to receive a copy of every console message.
	printCallback func(msg string)
}

// globalConsole is the process-wide singleton Console instance. Most callers
// interact with the console through the package-level convenience functions
// (Printf, Clear, Scroll, etc.) which delegate to this instance. A singleton
// makes sense because the engine only ever has one console, and many
// subsystems need to print to it without passing a *Console around.
var globalConsole = NewConsole(DefaultTextSize)

// NewConsole allocates a Console with the given buffer capacity. The buffer
// is not usable until Init is called — NewConsole only records the desired
// size. This two-phase construction mirrors the original C Quake pattern
// where Con_Init ran after command-line parsing so the user could override
// the buffer size via "-consize".
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

// Init performs one-time initialisation of the console buffer. It allocates
// the scrollback ring, fills it with spaces (the Quake convention for "empty"
// characters), and positions the write cursor at the last line. Calling Init
// more than once is a no-op, making it safe for subsystems that may race to
// initialise.
//
// customBufSize overrides the default buffer capacity if positive. This allows
// the engine's startup code to honour a user-specified console size.
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

// InitGlobal initialises the process-wide singleton console. This is the
// entry point called during engine startup (e.g. from Host_Init).
func InitGlobal(bufSize int) error {
	return globalConsole.Init(bufSize)
}

// LineWidth returns the current number of characters per line. The draw code
// uses this to know how many columns of text it can render.
func (c *Console) LineWidth() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lineWidth
}

// TotalLines returns the number of lines the ring buffer can hold. This is
// derived from bufSize / lineWidth and changes whenever the console is resized.
func (c *Console) TotalLines() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.totalLines
}

// CurrentLine returns the index of the most recently written line in the ring
// buffer. Combined with TotalLines it lets callers navigate the scrollback.
func (c *Console) CurrentLine() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.current
}

// BackScroll returns how many lines the user has scrolled upward from the
// latest output. 0 means "viewing the bottom / most recent text".
func (c *Console) BackScroll() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.backScroll
}

// SetBackScroll jumps to an absolute scroll position. Values are clamped to
// [0, totalLines-10] so the user always sees at least 10 lines of context.
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

// Scroll adjusts the scrollback position by the given number of lines
// (positive = scroll up into history, negative = scroll back toward latest).
// This is typically bound to mouse wheel or Page Up / Page Down keys.
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

// Resize adapts the console to a new character width (e.g. when the window
// resolution changes). This is non-trivial because the flat ring buffer stores
// lines at fixed offsets — changing the line width means re-laying-out every
// line in the buffer. The algorithm copies the old buffer into a temporary
// slice, clears the real buffer, then transfers as many characters per line as
// fit in the new width, preserving the most recent lines first.
//
// Notify timestamps are also re-mapped so that recently-printed lines continue
// to display in the notify area even after a resize.
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

// GetLine retrieves the text of a specific line from the ring buffer. The line
// index is absolute (not modular); GetLine applies the modular wrap internally.
// Trailing spaces are trimmed, matching the Quake convention where unused
// columns are filled with 0x20.
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

// Clear wipes the entire scrollback buffer, resets the write cursor, and
// clears the input line and notification timestamps. This is the handler
// for the "clear" console command.
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

// Dump writes the entire scrollback buffer to a file, stripping Quake's
// high-bit colour encoding (bit 7) so the output is clean ASCII. This is
// the handler for the "condump" console command, useful for capturing debug
// sessions or game logs.
func (c *Console) Dump(filename string) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	f, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer f.Close()

	// Find the oldest line that isn't entirely blank
	startLine := c.current - c.totalLines + 1
	if startLine < 0 {
		startLine = 0
	}

	for i := startLine; i <= c.current; i++ {
		lineNum := i % c.totalLines
		start := lineNum * c.lineWidth
		end := start + c.lineWidth
		if end > len(c.text) {
			end = len(c.text)
		}

		line := c.text[start:end]

		// Strip trailing spaces and Quake's high-bit colors
		cleanLine := make([]byte, 0, len(line))
		lastNonSpace := -1
		for j, ch := range line {
			cleanCh := ch & 0x7F
			cleanLine = append(cleanLine, cleanCh)
			if cleanCh != ' ' {
				lastNonSpace = j
			}
		}

		if lastNonSpace >= 0 {
			f.Write(cleanLine[:lastNonSpace+1])
		}
		f.WriteString("\n")
	}
	return nil
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

// printRaw writes a raw string into the scrollback ring buffer, one byte at
// a time. This is the lowest-level print routine — all other Printf variants
// ultimately funnel through here.
//
// Special handling:
//   - A leading byte of 0x01 or 0x02 activates Quake's "bronze" text mode
//     (bit 7 set on every character) for the entire message. 0x01 is used
//     for chat messages, 0x02 for warnings.
//   - The "[skipnotify]" prefix is stripped so tagged messages don't appear
//     in the on-screen notify area.
//   - Carriage return ('\r') causes the next character to overwrite the
//     current line from the beginning (used for progress indicators).
//   - Newline ('\n') starts a fresh line.
//   - Ordinary characters are written at position (current, x) and the
//     column counter x advances, wrapping to a new line at lineWidth.
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

// Printf formats a message and writes it to the console scrollback, then
// invokes any registered print callback and writes to the debug log. This is
// the workhorse print function used throughout the engine — analogous to
// Con_Printf in the original C Quake source.
func (c *Console) Printf(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	c.printRaw(msg)

	if c.printCallback != nil {
		c.printCallback(msg)
	}

	c.debugLogWrite(msg)
}

// DPrintf ("developer printf") prints only when the developer flag is true
// (typically controlled by the "developer" cvar). This keeps verbose debug
// spew out of the console during normal gameplay while letting developers
// opt in with "developer 1".
func (c *Console) DPrintf(developer bool, format string, args ...interface{}) {
	if !developer {
		return
	}
	msg := fmt.Sprintf(format, args...)
	c.printRaw(msg)
	c.debugLogWrite(msg)
}

// Warning prints a message prefixed with "Warning: " in Quake's bronze
// (high-bit) text colour. The 0x02 leader byte triggers the bronze rendering
// path in printRaw, making warnings visually distinct.
func (c *Console) Warning(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	c.Printf("\x02Warning: %s", msg)
}

// DWarning is the developer-only variant of Warning. It prints a bronze
// warning message only when the developer flag is true.
func (c *Console) DWarning(developer bool, format string, args ...interface{}) {
	if !developer {
		return
	}
	msg := fmt.Sprintf(format, args...)
	c.Printf("\x02Warning: %s", msg)
}

// SafePrintf prints to the console. In the original C engine, Con_SafePrintf
// temporarily disabled screen updates to avoid re-entrant rendering during
// long operations (e.g. loading). In this Go port the operation is identical
// to Printf since Go's concurrency model handles re-entrancy differently,
// but the API is preserved for source compatibility with callers that
// distinguish between the two.
func (c *Console) SafePrintf(format string, args ...interface{}) {
	c.Printf(format, args...)
}

// CenterPrintf prints text horizontally centered within the given character
// width. Each line of the message is individually padded with leading spaces.
// This is used for title screens, MOTD banners, and other decorative output
// where centred text is desired.
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

// QuakeBar builds a decorative horizontal rule using Quake's special line-
// drawing characters (0x35 = left cap, 0x36 = middle segment, 0x37 = right
// cap). These are glyph indices into Quake's bitmap font (conchars.lmp).
// The bar is capped at 40 characters and terminated with a newline.
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

// SetPrintCallback registers an optional function that is called with a copy
// of every message written via Printf. This allows external subsystems (such
// as a network relay or a graphical log viewer) to observe console output
// without polling the scrollback buffer.
func (c *Console) SetPrintCallback(fn func(msg string)) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.printCallback = fn
}

// debugLogWrite appends raw message text to the debug log file, if one is
// currently open. It is intentionally not guarded by the mutex — callers
// (Printf, DPrintf) already hold the lock or call from a safe context.
func (c *Console) debugLogWrite(msg string) {
	if c.debugLog != nil {
		c.debugLog.WriteString(msg)
	}
}

// EnableDebugLog opens (or re-opens) a file for logging all console output.
// This is the handler for the "condebug" console command. A timestamp header
// is written at the top of the file to identify the session. The directory
// structure is created automatically if it doesn't exist.
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

// DisableDebugLog closes the debug log file, if one is open. Further console
// output will no longer be written to disk until EnableDebugLog is called again.
func (c *Console) DisableDebugLog() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.debugLog != nil {
		c.debugLog.Close()
		c.debugLog = nil
	}
}

// Close performs orderly shutdown of the console, closing any open debug log.
// It should be called during engine teardown (Host_Shutdown).
func (c *Console) Close() {
	c.DisableDebugLog()
}

// NotifyTimes returns a snapshot of the timestamps for the most recent
// NumNotifyTimes lines. The draw code compares these against the current
// time to decide which lines are recent enough to display in the notify area.
func (c *Console) NotifyTimes() [NumNotifyTimes]time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.notifyTimes
}

// ClearNotify zeroes all notification timestamps so that no lines appear in
// the notify area. This is typically called when transitioning between game
// states (e.g. loading a new map) to avoid stale messages lingering on screen.
func (c *Console) ClearNotify() {
	c.mu.Lock()
	defer c.mu.Unlock()
	for i := range c.notifyTimes {
		c.notifyTimes[i] = time.Time{}
	}
}
