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

// RenderWorldIntoView renders the world into the provided gpuview texture
// view (ADR-0006, research 0006 §4). It runs the world render passes with the
// gpuview view as the color attachment via the scene-target seam, creating
// its own command encoder from the engine's raw wgpu device/queue (gogpu's
// Context is swapchain-scoped and OnDraw-only). Called by the quakui host
// from the gpuview OnRender callback, outside the engine's OnDraw.
func (r *Renderer) RenderWorldIntoView(view gpucontext.TextureView) error {
	if r == nil || view.IsNil() {
		return fmt.Errorf("renderer: RenderWorldIntoView: nil view")
	}
	dc := &DrawContext{renderer: r}
	dc.sceneRenderTarget = (*wgpu.TextureView)(view.Pointer())
	dc.sceneRenderActive = true
	if dc.renderer.WorldData() == nil {
		return nil
	}
	state := DefaultRenderFrameState()
	state.DrawWorld = true
	dc.renderWorldInternal(state)
	return nil
}
