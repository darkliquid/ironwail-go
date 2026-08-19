package hud

import (
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/hud"
	"github.com/darkliquid/ironwail-go/internal/quakeui/widgets"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// CrosshairWidget renders the center-screen crosshair glyph from the
// crosshair cvar, mirroring the legacy hud.Crosshair (M5.2). The glyph is
// hidden during intermission/cutscene and at viewsize >= 130.
type CrosshairWidget struct {
	widget.WidgetBase

	cvars *cvar.CVarSystem
	text  *widgets.QuakeText
	glyph int
}

// NewCrosshairWidget builds the crosshair widget from the cvar system and the
// QuakeText widget used to render the glyph.
func NewCrosshairWidget(cvs *cvar.CVarSystem, text *widgets.QuakeText) *CrosshairWidget {
	cw := &CrosshairWidget{cvars: cvs, text: text}
	cw.SetVisible(true)
	cw.SetEnabled(true)
	cw.refresh()
	return cw
}

// refresh reads the crosshair cvar into the glyph (mirroring
// hud.Crosshair.UpdateCvar: 0 off, <0 custom, >1 dot char 15, 1 '+').
func (cw *CrosshairWidget) refresh() {
	if cw == nil || cw.cvars == nil {
		cw.glyph = 0
		return
	}
	value := cw.cvars.FloatValue("crosshair")
	switch {
	case value == 0:
		cw.glyph = 0
	case value < 0:
		cw.glyph = int(-value) & 255
	case value > 1:
		cw.glyph = 15
	default:
		cw.glyph = int('+')
	}
}

// Glyph returns the crosshair character glyph (0 = hidden).
func (cw *CrosshairWidget) Glyph() int {
	if cw == nil {
		return 0
	}
	cw.refresh()
	return cw.glyph
}

// Hidden reports whether the crosshair should not be drawn for the given
// state (intermission, cutscene, or viewsize >= 130).
func (cw *CrosshairWidget) Hidden(state hud.State) bool {
	if cw == nil {
		return true
	}
	if state.Intermission != 0 || state.InCutscene {
		return true
	}
	if cw.cvars != nil && cw.cvars.IntValue("viewsize") >= 130 {
		return true
	}
	return cw.Glyph() == 0
}

// Layout sizes the crosshair widget.
func (cw *CrosshairWidget) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	size := c.Constrain(geometry.Sz(8, 8))
	cw.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	return size
}

// Draw renders the crosshair glyph via the QuakeText widget, centered on the
// canvas (the legacy crosshair canvas is centered on the viewport midline).
func (cw *CrosshairWidget) Draw(ctx widget.Context, canvas widget.Canvas) {
	if cw == nil || cw.text == nil {
		return
	}
	if cw.Hidden(hud.State{}) {
		return
	}
	glyph := cw.Glyph()
	if glyph == 0 {
		return
	}
	b := cw.Bounds()
	canvasW := int(b.Max.X - b.Min.X)
	canvasH := int(b.Max.Y - b.Min.Y)
	if canvasW <= 0 || canvasH <= 0 {
		canvasW, canvasH = 320, 200
	}
	// Center the 8x8 glyph on the canvas.
	x := float32(canvasW/2 - 4)
	y := float32(canvasH/2 - 4)
	cw.text.DrawString(canvas, x, y, string(rune(glyph)))
}

// Event consumes no input.
func (cw *CrosshairWidget) Event(ctx widget.Context, e event.Event) bool { return false }
