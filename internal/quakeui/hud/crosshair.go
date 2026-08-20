package hud

import (
	"github.com/darkliquid/ironwail-go/internal/hud"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

// CrosshairWidget renders the crosshair overlay.
type CrosshairWidget struct {
	crosshair hud.Crosshair
}

// NewCrosshairWidget constructs a crosshair sub-widget.
func NewCrosshairWidget() *CrosshairWidget {
	return &CrosshairWidget{}
}

// Draw renders the centered crosshair at the screen coordinates.
func (w *CrosshairWidget) Draw(rc renderer.RenderContext, state hud.State, width, height int) {
	rc.SetCanvas(renderer.CanvasCrosshair)
	w.crosshair.Draw(rc, state, width, height)
}
