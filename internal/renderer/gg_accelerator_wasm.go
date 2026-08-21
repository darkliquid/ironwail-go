//go:build js

package renderer

// On js/wasm the gg GPU accelerator is unavailable (wgpu/core is excluded),
// so ggcanvas.RenderDirect falls back to the CPU readback path. The viewer
// still renders the widget tree; this is the AC3b graceful path.
