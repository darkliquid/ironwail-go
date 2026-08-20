package hud

import (
	"github.com/darkliquid/ironwail-go/internal/hud"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

// CenterprintWidget renders centered announcements and typewriter messages.
type CenterprintWidget struct {
	centerprint *hud.Centerprint
}

// NewCenterprintWidget constructs a centerprint sub-widget.
func NewCenterprintWidget(cp *hud.Centerprint) *CenterprintWidget {
	return &CenterprintWidget{centerprint: cp}
}

// Draw renders the active centerprint message.
func (w *CenterprintWidget) Draw(rc renderer.RenderContext, state hud.State, width, height int) {
	if w == nil || w.centerprint == nil {
		return
	}
	rc.SetCanvas(renderer.CanvasDefault)
	w.centerprint.Draw(rc, state, width, height)
}
