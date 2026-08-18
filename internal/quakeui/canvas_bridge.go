package quakeui

import (
	"log/slog"

	"github.com/gogpu/gg"
	"github.com/gogpu/gg/integration/ggcanvas"
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/ui/render"
	"github.com/gogpu/ui/widget"
)

// canvasBridge owns the gg canvas that backs the gogpu/ui widget tree
// (spec §3.1, ADR-0002). It wraps a ggcanvas.Canvas created from the engine's
// gogpu GPU context provider, exposes the gg.Context as a widget.Canvas via
// render.NewCanvas, and presents the composited result onto the engine
// surface through a ggcanvas.RenderTarget.
//
// With a nil provider the bridge runs in software mode: a plain gg context is
// created and the canvas is never presented (headless tests).
type canvasBridge struct {
	provider gpucontext.DeviceProvider
	canvas   *ggcanvas.Canvas
	software *gg.Context
	w, h     int
}

// newCanvasBridge creates the bridge. The canvas is created lazily on the
// first ensure call so a headless host can construct without a GPU.
func newCanvasBridge(provider gpucontext.DeviceProvider) *canvasBridge {
	return &canvasBridge{provider: provider}
}

// ensure creates or resizes the backing canvas to the given logical size.
func (b *canvasBridge) ensure(w, h int) error {
	if b == nil {
		return nil
	}
	if b.canvas != nil {
		if b.canvas.Width() == w && b.canvas.Height() == h {
			return nil
		}
		return b.canvas.Resize(w, h)
	}
	if b.provider != nil {
		c, err := ggcanvas.New(b.provider, w, h)
		if err != nil {
			return err
		}
		b.canvas = c
		b.w, b.h = w, h
		return nil
	}
	// Software fallback for headless tests.
	b.software = gg.NewContext(w, h)
	b.w, b.h = w, h
	return nil
}

// widgetCanvas returns a widget.Canvas backed by the bridge's gg context.
func (b *canvasBridge) widgetCanvas(w, h int) widget.Canvas {
	if b == nil {
		return nil
	}
	if b.canvas != nil {
		return render.NewCanvas(b.canvas.Context(), w, h)
	}
	if b.software != nil {
		return render.NewCanvas(b.software, w, h)
	}
	return nil
}

// present composites the canvas onto the engine surface. The dc is the
// engine's gogpu.ContextRenderTarget (implements ggcanvas.RenderTarget). With
// a nil provider (software/headless) nothing is presented.
func (b *canvasBridge) present(dc ggcanvas.RenderTarget, w, h int) error {
	if b == nil || b.canvas == nil || dc == nil {
		return nil
	}
	b.canvas.MarkDirty()
	return b.canvas.Render(dc)
}

// close releases the canvas and software context.
func (b *canvasBridge) close() {
	if b == nil {
		return
	}
	if b.canvas != nil {
		if err := b.canvas.Close(); err != nil {
			slog.Debug("quakeui canvas close", "error", err)
		}
		b.canvas = nil
	}
	if b.software != nil {
		if err := b.software.Close(); err != nil {
			slog.Debug("quakeui software canvas close", "error", err)
		}
		b.software = nil
	}
}
