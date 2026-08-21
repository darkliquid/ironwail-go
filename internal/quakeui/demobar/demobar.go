// Package demobar implements the display-only demo playback progress bar for
// the gogpu/ui widget stack (ADR-0015, SPEC-004 §5.4/AC11). It mirrors the
// legacy drawRuntimeDemoControls rendering (research 0001 §7): a 38-character
// track with a cursor, a status glyph ("\\x0D"/"II"/speed arrows), the base
// speed label, the demo name, and an M:SS readout. It is display-only — no
// mouse interaction (interactive scrubbing belongs to bd ironwail-go-cuy).
package demobar

import (
	"fmt"
	"math"
	"strings"

	"github.com/darkliquid/ironwail-go/internal/quakeui/gfx"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// DemoBarState is the per-frame snapshot the engine feeds the bar: plain
// values from the playback DemoState plus current client time.
type DemoBarState struct {
	// Playback is true while a demo is playing.
	Playback bool
	// Speed and BaseSpeed mirror DemoState.Speed/BaseSpeed (0 = paused).
	Speed     float32
	BaseSpeed float32
	// Progress is the playback fraction in [0,1] (DemoState.Progress).
	Progress float64
	// Name is the demo base name (DemoName).
	Name string
	// ClientTime is the current game clock in seconds (M:SS readout).
	ClientTime float64
	// Show is true when the bar should render (demo bar timeout active).
	Show bool
}

// StateProvider yields the demo-bar snapshot each frame.
type StateProvider interface {
	DemoBarState() DemoBarState
}

// timebarChars mirrors the legacy 38-char bar width.
const timebarChars = 38

// DemoBarRoot is the display-only demo progress bar widget. It renders via
// the conchars atlas onto the widget canvas (the Scenario A ggcanvas through
// the overlay stack). Event returns false — the bar never consumes input.
type DemoBarRoot struct {
	widget.WidgetBase

	provider StateProvider
	atlas    *gfx.ConcharsAtlas

	drawCount uint64
}

// NewDemoBarRoot builds the demo bar widget. conchars/palette feed the text
// atlas exactly like the menu/console/HUD roots.
func NewDemoBarRoot(provider StateProvider, conchars, palette []byte) *DemoBarRoot {
	r := &DemoBarRoot{
		provider: provider,
		atlas:    gfx.NewConcharsAtlas(conchars, palette),
	}
	r.SetVisible(true)
	r.SetEnabled(true)
	r.SetRepaintBoundary(true)
	return r
}

// SetProvider wires (or replaces) the per-frame demo state source.
func (r *DemoBarRoot) SetProvider(p StateProvider) {
	if r == nil {
		return
	}
	r.provider = p
}

// IsVisible reports whether the demo bar should render this frame.
func (r *DemoBarRoot) IsVisible() bool {
	if r == nil || r.provider == nil {
		return false
	}
	st := r.provider.DemoBarState()
	return st.Playback && st.Show
}

// State returns the current per-frame demo bar state (for tests).
func (r *DemoBarRoot) State() DemoBarState {
	if r == nil || r.provider == nil {
		return DemoBarState{}
	}
	return r.provider.DemoBarState()
}

// Layout sizes the bar to the window width (the bar is drawn in window
// coordinates mirroring the legacy Sbar canvas placement).
func (r *DemoBarRoot) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	w := c.MaxWidth
	h := c.MaxHeight
	if w <= 0 {
		w = 320
	}
	if h <= 0 {
		h = 200
	}
	sz := geometry.Sz(w, h)
	r.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), sz))
	return sz
}

// Draw renders the progress bar at the legacy Sbar position: top center.
func (r *DemoBarRoot) Draw(ctx widget.Context, canvas widget.Canvas) {
	if r == nil || !r.IsVisible() || r.atlas == nil || canvas == nil {
		return
	}
	r.drawCount++

	st := r.provider.DemoBarState()

	// The legacy demo bar draws in Sbar space where (0,0) is the canvas
	// center and Y is up; the widget canvas is top-left anchored. Translate
	// the legacy coordinates to window space.
	winW := 320.0
	winH := 200.0
	if ctx != nil {
		ws := ctx.WindowSize()
		winW = float64(ws.Width)
		winH = float64(ws.Height)
	}
	centerX := int(winW / 2)
	centerY := int(winH / 2)

	// Legacy coords: x = 160 - timebarChars/2*8 (relative to 320-wide center),
	// y = -20 (20px above center in Sbar space where Y is up).
	x := centerX - timebarChars/2*8
	y := centerY - 20

	// Status glyph: "\x0D" arrow (or ">" with custom conchars), "II" when
	// paused, doubled/speed arrows, reversed when rewinding.
	status := string([]byte{13})
	if st.Speed == 0 {
		status = "II"
	} else if math.Abs(float64(st.Speed)) > 1 {
		status += status
	}
	if st.Speed < 0 {
		status = strings.Repeat("<", len(status))
	}

	// Base speed label (right of the status glyph).
	base := formatDemoBaseSpeed(st.BaseSpeed)

	// Demo name centered.
	name := st.Name

	// Progress + cursor.
	progress := st.Progress
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	cursorX := x + int(float64((timebarChars-1)*8)*progress)

	seconds := int(st.ClientTime)
	timeText := fmt.Sprintf("%d:%02d", seconds/60, seconds%60)

	r.drawString(canvas, x, y, status)
	if base != "" {
		r.drawString(canvas, x+(timebarChars-len(base))*8, y, base)
	}
	if name != "" {
		r.drawString(canvas, centerX-len(name)*4, y, name)
	}

	// Track: 128/129/130 end caps + 129 body + 131 cursor at barY.
	barY := y - 8
	r.drawCharacter(canvas, x-8, barY, 128)
	for i := 0; i < timebarChars; i++ {
		r.drawCharacter(canvas, x+i*8, barY, 129)
	}
	r.drawCharacter(canvas, x+timebarChars*8, barY, 130)
	r.drawCharacter(canvas, cursorX, barY, 131)

	// Time readout above the cursor.
	timeX := cursorX
	if colon := strings.IndexByte(timeText, ':'); colon >= 0 {
		timeX -= colon * 8
	}
	timeY := barY - 11
	r.drawString(canvas, timeX, timeY, timeText)
}

// drawCharacter draws one 8x8 conchars glyph at (x, y).
func (r *DemoBarRoot) drawCharacter(canvas widget.Canvas, x, y int, num int) {
	if r == nil || r.atlas == nil || canvas == nil {
		return
	}
	if num <= 0 || num == ' ' {
		return
	}
	if num > 255 {
		num = '?'
	}
	img := r.atlas.GlyphImage(byte(num))
	if img == nil {
		return
	}
	if x < -8 || y < -8 {
		// Glyph fully off-canvas; skip the draw.
		return
	}
	canvas.DrawImage(img, geometry.Pt(float32(x), float32(y)))
}

// drawString draws a console string at 8px-per-glyph spacing.
func (r *DemoBarRoot) drawString(canvas widget.Canvas, x, y int, str string) {
	for i, ch := range []byte(str) {
		r.drawCharacter(canvas, x+i*8, y, int(ch))
	}
}

// Event is a no-op: the demo bar is display-only (ADR-0015).
func (r *DemoBarRoot) Event(ctx widget.Context, e event.Event) bool {
	return false
}

// formatDemoBaseSpeed mirrors ui.FormatDemoBaseSpeed without importing
// internal/game (AC7 isolation): "2x", "1/2x", or "" for zero.
func formatDemoBaseSpeed(speed float32) string {
	if speed == 0 {
		return ""
	}
	absSpeed := math.Abs(float64(speed))
	if absSpeed >= 1 {
		return fmt.Sprintf("%gx", absSpeed)
	}
	return fmt.Sprintf("1/%gx", 1/absSpeed)
}
