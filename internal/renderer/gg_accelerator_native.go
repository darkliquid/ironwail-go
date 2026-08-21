//go:build !js

package renderer

// The gg/gpu package registers the GPU accelerator (SDF shapes) so
// ggcanvas.RenderDirect can flush GPU shapes onto the surface instead of
// falling back to CPU (ADR-0011 Scenario A native path). It is imported only
// on native: gg/gpu pulls wgpu/core, which is excluded on js/wasm.
import _ "github.com/gogpu/gg/gpu"
