package renderer

import (
	"fmt"

	"github.com/gogpu/gpucontext"
	"github.com/gogpu/wgpu"
)

// RenderIntoWorldTexture retargets the world render target to the provided
// gpuview texture view (ADR-0006, research 0006 §4). After this call the
// world/entity/polyblend passes render into view via the scene-target seam
// (currentWGPURenderTargetView) instead of the swapchain surface; the desktop
// compositor blits the gpuview texture as the base layer under the UI
// widgets, and the engine does NOT composite the world back to the surface.
// Returns an error for a nil or empty view.
func (dc *DrawContext) RenderIntoWorldTexture(view gpucontext.TextureView) error {
	if dc == nil || view.IsNil() {
		return fmt.Errorf("renderer: RenderIntoWorldTexture: nil view")
	}
	viewPtr := (*wgpu.TextureView)(view.Pointer())
	if viewPtr == nil {
		return fmt.Errorf("renderer: RenderIntoWorldTexture: empty view pointer")
	}
	dc.sceneRenderTarget = viewPtr
	dc.sceneRenderActive = true
	return nil
}

// DisableWorldTexture detargets the world render back to the swapchain
// surface. Called when the gpuview texture is unmounted (surface switch).
func (dc *DrawContext) DisableWorldTexture() {
	if dc == nil {
		return
	}
	dc.sceneRenderTarget = nil
	dc.sceneRenderActive = false
}
