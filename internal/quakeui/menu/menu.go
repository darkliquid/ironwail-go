// Package menu implements the gogpu/ui menu widget root and per-page row
// models (IRONWAIL-SPEC-001 §3.2, M3.2). The legacy menu.Manager state
// machine remains the source of truth: the widget reads State()/CursorFor()/
// TextBuffer()/HostSettings()/Mods()/SaveSlots() each frame and routes key
// events back through M_Key/M_Char (R1.2, G.13). Only the presentation moves
// to gogpu/ui; the action side is shared verbatim.
package menu

import (
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/quakeui/widgets"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// MenuRow is a single selectable row on a menu page: a label (drawn at the
// legacy layout position) and an optional value shown on the right.
type MenuRow struct {
	// Label is the row text (e.g. "SINGLE PLAYER", "MAX PLAYERS").
	Label string
	// Value is the right-aligned value (e.g. "ON", "16", "start"), or "".
	Value string
}

// MenuRoot is the gogpu/ui menu widget. It reads the legacy menu.Manager
// state each frame and exposes the active page's rows and cursor, which the
// Draw pass renders via the QuakeText widget. Key events are routed back to
// the manager's M_Key/M_Char action methods (navigation/actions preserved).
type MenuRoot struct {
	widget.WidgetBase

	mgr    *menu.Manager
	cvars  *cvar.CVarSystem
	text   *widgets.QuakeText
	rows   []MenuRow
	cursor int
}

// NewMenuRoot builds the menu widget root. cvars may be nil (headless tests);
// text is the QuakeText widget used to render rows.
func NewMenuRoot(mgr *menu.Manager, text *widgets.QuakeText) *MenuRoot {
	r := &MenuRoot{mgr: mgr, text: text}
	r.SetVisible(true)
	r.SetEnabled(true)
	return r
}

// SetCVars attaches the cvar system used for video/controls value reads.
func (r *MenuRoot) SetCVars(cvs *cvar.CVarSystem) {
	if r == nil {
		return
	}
	r.cvars = cvs
}

// Rows returns the active page's rows, refreshed from the manager state.
func (r *MenuRoot) Rows() []MenuRow {
	if r == nil || r.mgr == nil {
		return nil
	}
	r.refresh()
	return r.rows
}

// Cursor returns the active page's cursor position.
func (r *MenuRoot) Cursor() int {
	if r == nil {
		return 0
	}
	r.refresh()
	return r.cursor
}

// refresh rebuilds the row model and cursor from the current manager state.
func (r *MenuRoot) refresh() {
	if r == nil || r.mgr == nil {
		r.rows = nil
		r.cursor = 0
		return
	}
	state := r.mgr.State()
	r.rows = rowsForState(r.mgr, r.cvars, state)
	r.cursor = r.mgr.CursorFor(state)
}

// handleKey routes an engine key code to the manager's action path.
func (r *MenuRoot) handleKey(key int) {
	if r == nil || r.mgr == nil {
		return
	}
	r.mgr.M_Key(key)
}

// handleChar routes a rune to the manager's char action path (text entry).
func (r *MenuRoot) handleChar(ch rune) {
	if r == nil || r.mgr == nil {
		return
	}
	r.mgr.M_Char(ch)
}

// Layout sizes the menu root to the 320x200 menu viewport (spec §3.3 R1.5).
func (r *MenuRoot) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	size := c.Constrain(geometry.Sz(320, 200))
	r.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	return size
}

// Draw renders the active page rows via the QuakeText widget, at the legacy
// M_Draw layout positions (research 0001 §3): rows start at (84, 32) with a
// 20px stride, and the cursor arrow sits at x=54. The 320x200 menu viewport is
// scaled to the canvas size (min(canvasW/320, canvasH/200)) and centered,
// matching the legacy CanvasMenu transform (spec §3.3 R1.5).
func (r *MenuRoot) Draw(ctx widget.Context, canvas widget.Canvas) {
	if r == nil || r.text == nil {
		return
	}
	r.refresh()

	// Compute the menu viewport scale and centering offset.
	b := r.Bounds()
	canvasW := int(b.Max.X - b.Min.X)
	canvasH := int(b.Max.Y - b.Min.Y)
	if canvasW <= 0 || canvasH <= 0 {
		canvasW, canvasH = 320, 200
	}
	scale := float32(min(canvasW/320, canvasH/200))
	if scale < 1 {
		scale = 1
	}
	offsetX := float32(canvasW-320*int(scale)) / 2
	offsetY := float32(canvasH-200*int(scale)) / 2

	// Diagnostic backdrop: fill the menu viewport with a semi-transparent
	// black rect so the menu region is visible.
	canvas.DrawRect(geometry.NewRect(offsetX, offsetY, 320*scale, 200*scale), widget.RGBA(0, 0, 0, 0.5))

	for i, row := range r.rows {
		y := offsetY + float32(32+i*20)*scale
		r.text.DrawStringScaled(canvas, offsetX+84*scale, y, scale, row.Label)
		if row.Value != "" {
			r.text.DrawStringScaled(canvas, offsetX+160*scale, y, scale, row.Value)
		}
	}
	// Cursor arrow at the active row (legacy drawCursor uses char 12).
	if r.cursor >= 0 && r.cursor < len(r.rows) {
		r.text.DrawStringScaled(canvas, offsetX+54*scale, offsetY+float32(32+r.cursor*20)*scale, scale, string(rune(12)))
	}
}

// Event routes key/char events to the manager action path and invalidates the
// widget so the tree re-renders with the updated cursor/state.
func (r *MenuRoot) Event(ctx widget.Context, e event.Event) bool {
	if r == nil || r.mgr == nil {
		return false
	}
	if ke, ok := e.(*event.KeyEvent); ok {
		// Text runes (from OnTextInput) go through M_Char; navigation keys
		// through M_Key.
		if ke.Rune != 0 {
			r.handleChar(ke.Rune)
			ctx.Invalidate()
			return true
		}
		key := keyEventToEngine(ke)
		if key >= 0 {
			r.handleKey(key)
			ctx.Invalidate()
			return true
		}
	}
	return false
}
