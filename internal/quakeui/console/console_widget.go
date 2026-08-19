// Package console implements the gogpu/ui console widget (IRONWAIL-SPEC-001
// §3.2, M4.1). It reads the legacy console.Console data (ring buffer, input
// line, backscroll, notify timestamps) via existing accessors and presents it
// as a widget; the console package itself is untouched (spec §4.1).
package console

import (
	"time"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/quakeui/widgets"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// consoleNotifyDefaultTTL mirrors the legacy console's default notify TTL.
const consoleNotifyDefaultTTL = 3 * time.Second

// ConsoleWidget renders the dropdown console: scrollback rows, the ']'
// prompt + input line with blink cursor, a scroll indicator when backscrolled,
// and notify lines with con_notify* fade alpha. The widget reads the console
// data each frame; the console package is the source of truth.
type ConsoleWidget struct {
	widget.WidgetBase

	con  *console.Console
	text *widgets.QuakeText

	rows   []string
	input  string
	cursor int
	scrolled bool
}

// NewConsoleWidget builds the console widget from the console data source and
// the QuakeText widget used to render rows.
func NewConsoleWidget(con *console.Console, text *widgets.QuakeText) *ConsoleWidget {
	cw := &ConsoleWidget{con: con, text: text}
	cw.SetVisible(true)
	cw.SetEnabled(true)
	return cw
}

// refresh reads the console state into the widget's cached fields.
func (cw *ConsoleWidget) refresh() {
	if cw == nil || cw.con == nil {
		cw.rows = nil
		cw.input = ""
		cw.cursor = 0
		cw.scrolled = false
		return
	}
	cw.input = cw.con.InputLine()
	cw.cursor = cw.con.CursorPos()
	cw.scrolled = cw.con.BackScroll() > 0
	cw.rows = cw.visibleRows()
}

// visibleRows returns the scrollback rows visible in the console window,
// mirroring the legacy draw: the window ends at current-backScroll and shows
// a fixed number of rows above it.
func (cw *ConsoleWidget) visibleRows() []string {
	if cw == nil || cw.con == nil {
		return nil
	}
	// The legacy draw shows (height/8 - 2) rows; for the widget we expose the
// last 12 lines (a reasonable default window) ending at current-backScroll.
	const window = 12
	current := cw.con.CurrentLine()
	back := cw.con.BackScroll()
	bottom := current - back
	start := bottom - (window - 1)
	if start < 0 {
		start = 0
	}
	rows := make([]string, 0, window)
	for line := start; line <= bottom; line++ {
		if line < 0 {
			continue
		}
		rows = append(rows, cw.con.Line(line))
	}
	return rows
}

// Rows returns the visible scrollback rows.
func (cw *ConsoleWidget) Rows() []string {
	if cw == nil {
		return nil
	}
	cw.refresh()
	return cw.rows
}

// InputLine returns the current console input line.
func (cw *ConsoleWidget) InputLine() string {
	if cw == nil {
		return ""
	}
	cw.refresh()
	return cw.input
}

// Prompt returns the console input prompt glyph (']').
func (cw *ConsoleWidget) Prompt() rune {
	if cw == nil || cw.text == nil {
		return ']'
	}
	return cw.text.Prompt()
}

// Scrolled reports whether the console is backscrolled (scroll indicator).
func (cw *ConsoleWidget) Scrolled() bool {
	if cw == nil {
		return false
	}
	cw.refresh()
	return cw.scrolled
}

// NotifyAlpha returns the fade alpha for a notify line printed at ts,
// mirroring the legacy console.notifyAlpha (con_notifytime TTL + con_notifyfade
// fade tail).
func (cw *ConsoleWidget) NotifyAlpha(ts time.Time) float64 {
	if cw == nil || cw.con == nil {
		return 0
	}
	if ts.IsZero() {
		return 0
	}
	ttl := consoleNotifyDefaultTTL
	fade := time.Duration(0)
	if cv := cw.con.CVar; cv != nil {
		if secs := cv.FloatValue("con_notifytime"); secs > 0 {
			ttl = time.Duration(secs * float64(time.Second))
		}
		if cv.BoolValue("con_notifyfade") {
			if secs := cv.FloatValue("con_notifyfadetime"); secs > 0 {
				fade = time.Duration(secs * float64(time.Second))
			}
		}
	}
	now := time.Now()
	remaining := ts.Add(ttl + fade).Sub(now)
	if remaining <= 0 {
		return 0
	}
	if fade <= 0 || remaining >= fade {
		return 1
	}
	return float64(remaining) / float64(fade)
}

// Layout sizes the console widget to the given constraints.
func (cw *ConsoleWidget) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	size := c.Constrain(geometry.Sz(320, 200))
	cw.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	return size
}

// Draw renders the console rows, prompt + input line, and scroll indicator
// via the QuakeText widget, at the legacy console draw positions (8px cells,
// prompt at the bottom row).
func (cw *ConsoleWidget) Draw(ctx widget.Context, canvas widget.Canvas) {
	if cw == nil || cw.text == nil {
		return
	}
	cw.refresh()

	// Scroll indicator at the top when backscrolled.
	if cw.scrolled {
		cw.text.DrawString(canvas, 8, 0, "^^^")
	}

	// Scrollback rows, 8px pitch.
	for i, line := range cw.rows {
		cw.text.DrawString(canvas, 8, float32(8+i*8), line)
	}

	// Prompt + input line at the bottom.
	bottomY := float32(200 - 8)
	cw.text.DrawString(canvas, 8, bottomY, string(cw.Prompt())+cw.input)
}

// Event consumes no input (console key handling is engine-side).
func (cw *ConsoleWidget) Event(ctx widget.Context, e event.Event) bool { return false }
