package hud

import (
	"github.com/darkliquid/ironwail-go/internal/hud"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

// StatusBarWidget renders the status bar component.
type StatusBarWidget struct {
	status *hud.StatusBar
}

// NewStatusBarWidget constructs a status bar sub-widget.
func NewStatusBarWidget(status *hud.StatusBar) *StatusBarWidget {
	return &StatusBarWidget{status: status}
}

// DrawClassic renders the classic 320x48 status bar.
func (w *StatusBarWidget) DrawClassic(rc renderer.RenderContext, state hud.State) {
	if w == nil || w.status == nil {
		return
	}
	rc.SetCanvas(renderer.CanvasSbar)
	w.status.Draw(rc, state, 320, 48)
}

// DrawModern renders the modern corner HUD layout.
func (w *StatusBarWidget) DrawModern(rc renderer.RenderContext, state hud.State, sideAmmo bool) {
	if w == nil || w.status == nil {
		return
	}
	rc.SetCanvas(renderer.CanvasSbar2)
	w.status.DrawModern(rc, state, sideAmmo)
}
