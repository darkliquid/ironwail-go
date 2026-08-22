package game

import (
	"github.com/gogpu/gg/integration/ggcanvas"
	"github.com/gogpu/gpucontext"
)

// quakeuiHost adapts the engine to internal/quakeui.Host (ADR-0009, AC7). The
// engine implements the adapter with plain values only; no quakeui code lives
// in the game package.
type quakeuiHost struct {
	g *Game
}

// CVar reads an engine cvar as a plain float.
func (h *quakeuiHost) CVar(name string) float64 {
	if h == nil || h.g == nil || h.g.Host == nil || h.g.Host.CVar == nil {
		return 0
	}
	return h.g.Host.CVar.FloatValue(name)
}

// ExecuteCommandText queues an engine console command.
func (h *quakeuiHost) ExecuteCommandText(text string) {
	if h == nil || h.g == nil || h.g.Host == nil || h.g.Host.Cmd == nil {
		return
	}
	h.g.Host.Cmd.AddText(text)
}

// PlaySound plays a sound by name through the engine audio path.
func (h *quakeuiHost) PlaySound(name string) {
	if h == nil || h.g == nil {
		return
	}
	h.g.playMenuSound(name)
}

// Quit requests a clean engine shutdown from the ui loop.
func (h *quakeuiHost) Quit() {
	if h == nil || h.g == nil || h.g.Host == nil {
		return
	}
	h.g.Host.Abort("quakeui quit")
}

// GPUContextProvider exposes the gogpu DeviceProvider for the GPU-backed
// ggcanvas used by the Scenario A overlay (ADR-0011). Returns nil until the
// gogpu renderer has been created (startup), and after it has been released.
func (h *quakeuiHost) GPUContextProvider() gpucontext.DeviceProvider {
	if h == nil || h.g == nil {
		return nil
	}
	return h.g.gpuContextProvider()
}

// SurfaceView returns the current frame's GPU texture view from the active
// render context, suitable for RenderDirect(sv, sw, sh). This is how the
// engine-owned loop hands the swapchain surface to the UI composite pass
// without the UI depending on engine renderer internals.
func (h *quakeuiHost) SurfaceView() gpucontext.TextureView {
	if h == nil || h.g == nil {
		return gpucontext.TextureView{}
	}
	return h.g.currentUISurfaceView()
}

// WindowSize reports the engine's current logical window size (0,0 when no
// renderer is available). The ui window mirrors it so the 320x200 menu
// transform and widget layout match the real viewport.
func (h *quakeuiHost) WindowSize() (width, height int) {
	if h == nil || h.g == nil || h.g.Renderer == nil {
		return 0, 0
	}
	w, hh := h.g.Renderer.Size()
	return w, hh
}

// RenderTarget returns the current frame's gogpu render target for the
// target-aware ggcanvas composite (canvas.Render), borrowed from the frame's
// live draw context. Nil when no frame is active.
func (h *quakeuiHost) RenderTarget() ggcanvas.RenderTarget {
	if h == nil || h.g == nil {
		return nil
	}
	return h.g.currentUIRenderTarget()
}
