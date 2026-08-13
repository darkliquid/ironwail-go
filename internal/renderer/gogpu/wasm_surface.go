//go:build js && wasm

package gogpu

import (
	"fmt"
	"syscall/js"

	"github.com/gogpu/gputypes"
	"github.com/gogpu/wgpu"
)

// GetCanvasSurface creates a WebGPU surface from an HTML5 canvas element.
func GetCanvasSurface(instance *wgpu.Instance, canvasID string) (*wgpu.Surface, error) {
	if instance == nil {
		return nil, fmt.Errorf("wasm_surface: nil wgpu instance")
	}

	doc := js.Global().Get("document")
	if doc.IsUndefined() || doc.IsNull() {
		return nil, fmt.Errorf("wasm_surface: document unavailable")
	}

	canvas := doc.Call("getElementById", canvasID)
	if canvas.IsUndefined() || canvas.IsNull() {
		return nil, fmt.Errorf("wasm_surface: canvas element #%s not found", canvasID)
	}

	surface, err := instance.CreateSurfaceFromCanvas(canvas)
	if err != nil {
		return nil, fmt.Errorf("wasm_surface: CreateSurfaceFromCanvas failed: %w", err)
	}

	return surface, nil
}

// GetBrowserPreferredCanvasFormat queries navigator.gpu.getPreferredCanvasFormat().
func GetBrowserPreferredCanvasFormat() gputypes.TextureFormat {
	nav := js.Global().Get("navigator")
	if nav.IsUndefined() || nav.IsNull() {
		return gputypes.TextureFormatBGRA8Unorm
	}
	gpu := nav.Get("gpu")
	if gpu.IsUndefined() || gpu.IsNull() {
		return gputypes.TextureFormatBGRA8Unorm
	}
	getPreferred := gpu.Get("getPreferredCanvasFormat")
	if getPreferred.IsUndefined() || getPreferred.IsNull() {
		return gputypes.TextureFormatBGRA8Unorm
	}
	fmtStr := gpu.Call("getPreferredCanvasFormat").String()
	switch fmtStr {
	case "rgba8unorm":
		return gputypes.TextureFormatRGBA8Unorm
	case "bgra8unorm":
		return gputypes.TextureFormatBGRA8Unorm
	case "rgba8unorm-srgb":
		return gputypes.TextureFormatRGBA8UnormSrgb
	case "bgra8unorm-srgb":
		return gputypes.TextureFormatBGRA8UnormSrgb
	default:
		return gputypes.TextureFormatBGRA8Unorm
	}
}
