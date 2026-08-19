package game

import (
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/quakui"
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

// quakuiHost adapts the engine to internal/quakui.Host (ADR-0009, AC7). The
// engine implements the adapter with gogpu/gpucontext types and plain values
// only; no quakui code lives in the game package.
type quakuiHost struct {
	g    *Game
	view gpucontext.TextureView
}

// GogpuApp returns the engine's gogpu.App, which desktop.Run takes over as
// the window/render-loop owner on ui_backend=1 (ADR-0006).
func (h *quakuiHost) GogpuApp() *gogpu.App {
	if h == nil || h.g == nil || h.g.Renderer == nil {
		return nil
	}
	return h.g.Renderer.GogpuApp()
}

// WorldTexture returns the current gpuview texture (nil/zero until mounted).
func (h *quakuiHost) WorldTexture() gpucontext.TextureView {
	if h == nil {
		return gpucontext.TextureView{}
	}
	return h.view
}

// RenderIntoWorldTexture stores the gpuview view the gpuview OnRender provides.
func (h *quakuiHost) RenderIntoWorldTexture(view gpucontext.TextureView) error {
	h.view = view
	return nil
}

// RenderFrame renders the world into the stored gpuview view via the
// renderer's RenderWorldIntoView (scene-target seam + raw wgpu encoders).
func (h *quakuiHost) RenderFrame() error {
	if h == nil || h.g == nil || h.g.Renderer == nil || h.view.IsNil() {
		return nil
	}
	return h.g.Renderer.RenderWorldIntoView(h.view)
}

// CVar reads an engine cvar as a plain float.
func (h *quakuiHost) CVar(name string) float64 {
	if h == nil || h.g == nil || h.g.Host == nil || h.g.Host.CVar == nil {
		return 0
	}
	return h.g.Host.CVar.FloatValue(name)
}

// KeyDest returns the plain enum routing destination.
func (h *quakuiHost) KeyDest() quakui.KeyDest {
	if h == nil || h.g == nil || h.g.Input == nil {
		return quakui.KeyDestGame
	}
	switch h.g.Input.KeyDest() {
	case input.KeyConsole:
		return quakui.KeyDestConsole
	}
	if h.g.Menu != nil && h.g.Menu.IsActive() {
		return quakui.KeyDestMenu
	}
	return quakui.KeyDestGame
}

// ExecuteCommandText queues an engine console command.
func (h *quakuiHost) ExecuteCommandText(text string) {
	if h == nil || h.g == nil || h.g.Host == nil || h.g.Host.Cmd == nil {
		return
	}
	h.g.Host.Cmd.AddText(text)
}

// PlaySound plays a sound by name through the engine audio path.
func (h *quakuiHost) PlaySound(name string) {
	if h == nil || h.g == nil {
		return
	}
	h.g.playMenuSound(name)
}

// Quit requests a clean engine shutdown from the ui loop.
func (h *quakuiHost) Quit() {
	if h == nil || h.g == nil || h.g.Host == nil {
		return
	}
	h.g.Host.Abort("quakeui quit")
}
