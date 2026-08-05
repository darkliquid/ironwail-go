//go:build js && wasm

package gogpu

import (
	"fmt"
	"syscall/js"

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
