//go:build !js

package renderer

// gg/gpu registers the SDF GPU accelerator so ggcanvas.RenderDirect and the
// target-aware canvas.Render can flush GPU shapes to a surface view directly
// instead of CPU readback (ADR-0011 Scenario A). Native only: gg/gpu pulls
// wgpu/core, excluded on js/wasm.
//
// NOTE (investigation in progress): binding the accelerator into the engine's
// wgpu device was suspected of breaking swapchain lifetimes when drawing via
// the bare RenderDirect(path). Re-enabled to verify the target-aware
// canvas.Render(shared-encoder) path is the correct fix.
import _ "github.com/gogpu/gg/gpu"
