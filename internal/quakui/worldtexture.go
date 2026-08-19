package quakui

import (
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// WorldTexture is the UI widget that presents the engine world. It implements
// externalTextureWidget so that gogpu/ui's compositor layer tree blits its
// offscreen GPU texture as the base layer under the UI widgets (ADR-0006,
// research 0006 §2-4).
type WorldTexture struct {
	widget.WidgetBase

	host    Host
	width   int
	height  int
	texture gpucontext.TextureView
	release func()
}

// NewWorldTexture builds the world texture widget wired to the host's world render.
func NewWorldTexture(host Host, width, height int) *WorldTexture {
	if width <= 0 {
		width = 320
	}
	if height <= 0 {
		height = 200
	}
	w := &WorldTexture{
		host:   host,
		width:  width,
		height: height,
	}
	w.SetVisible(true)
	w.SetEnabled(true)
	w.SetRepaintBoundary(true)
	return w
}

// Texture returns the underlying GPU texture view for compositor blitting.
func (w *WorldTexture) Texture() gpucontext.TextureView {
	if w == nil {
		return gpucontext.TextureView{}
	}
	return w.texture
}

// ViewportSize returns the current texture dimensions.
func (w *WorldTexture) ViewportSize() (int, int) {
	if w == nil {
		return 0, 0
	}
	return w.width, w.height
}

// Layout adapts the world texture to the container constraints, updating width/height
// and marking the texture for recreation if dimensions changed.
func (w *WorldTexture) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	width := int(c.MaxWidth)
	height := int(c.MaxHeight)
	if width <= 0 {
		width = w.width
	}
	if height <= 0 {
		height = w.height
	}
	if width != w.width || height != w.height {
		w.width = width
		w.height = height
		if w.release != nil {
			w.release()
			w.release = nil
		}
		w.texture = gpucontext.TextureView{}
	}
	size := geometry.Sz(float32(w.width), float32(w.height))
	w.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	return size
}

// Draw acquires/re-creates the GPU texture if needed, invokes the host render frame,
// and schedules the next animation frame for continuous game world rendering.
func (w *WorldTexture) Draw(ctx widget.Context, canvas widget.Canvas) {
	if w == nil || !w.IsVisible() {
		return
	}

	if w.texture.IsNil() && ctx != nil {
		if provider, ok := ctx.(widget.GPUTextureProvider); ok {
			texAny, release := provider.CreateGPUTexture(w.width, w.height)
			if tex, ok := texAny.(gpucontext.TextureView); ok && !tex.IsNil() {
				w.texture = tex
				w.release = release
			}
		}
	}

	if !w.texture.IsNil() && w.host != nil {
		if err := w.host.RenderIntoWorldTexture(w.texture); err == nil {
			_ = w.host.RenderFrame()
		}
	}

	if sched, ok := ctx.(widget.AnimationScheduler); ok {
		sched.ScheduleAnimationFrame()
	}
	w.SetNeedsRedraw(true)
}

// Event handles events directed at the world texture (consumes none).
func (w *WorldTexture) Event(ctx widget.Context, e event.Event) bool {
	return false
}

// Children returns nil as WorldTexture is a leaf widget.
func (w *WorldTexture) Children() []widget.Widget {
	return nil
}

// Unmount releases the offscreen GPU texture when the widget is unmounted.
func (w *WorldTexture) Unmount() {
	if w.release != nil {
		w.release()
		w.release = nil
	}
	w.texture = gpucontext.TextureView{}
}
