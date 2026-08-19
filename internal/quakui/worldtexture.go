package quakui

import (
	"github.com/gogpu/gpucontext"
	"github.com/gogpu/ui/core/gpuview"
)

// WorldTexture is the gpuview widget that presents the engine world. The
// desktop compositor blits its offscreen texture as the base layer under the
// UI widgets (ADR-0006, research 0006 §2-4). The engine renders the world
// into the texture via the OnRender callback, which routes through the Host's
// RenderIntoWorldTexture hook (M1.4a).
type WorldTexture struct {
	*gpuview.Widget
}

// NewWorldTexture builds the gpuview widget wired to the host's world render.
// Continuous(true) drives a per-frame render so the world updates live.
func NewWorldTexture(host Host, width, height int) *WorldTexture {
	w := gpuview.New(
		gpuview.Size(width, height),
		gpuview.OnRender(func(view gpucontext.TextureView) {
			if err := host.RenderIntoWorldTexture(view); err == nil {
				_ = host.RenderFrame()
			}
		}),
		gpuview.Continuous(true),
	)
	return &WorldTexture{Widget: w}
}
