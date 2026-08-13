//go:build js && wasm

// wasm_blit.go — browser present path for the walkthrough.
//
// gogpu's App.Run() cannot drive a usable render loop on Go's wasm runtime
// (render-thread goroutine + no-op WaitEvents => busy-loop + JS re-entrancy
// stack faults), so the browser build may not get GPU presents at all. This
// is a best-effort CPU-side present: when a world frame IS rendered, copy it
// back off the GPU and blit it onto the <canvas> via putImageData so the
// walkthrough viewport shows real engine output without relying on gogpu's
// swapchain.
package renderer

import (
	"log/slog"
	"syscall/js"
)

// WasmBlitPresent copies the last rendered world texture onto the page
// canvas (2D context). Returns false when no frame is available (headless /
// renderer not producing) — the caller degrades gracefully.
func (r *Renderer) WasmBlitPresent() bool {
	data, width, height, ok := r.ReadbackWorldTexture()
	if !ok || width <= 0 || height <= 0 || len(data) < width*height*4 {
		return false
	}

	doc := js.Global().Get("document")
	if doc.IsUndefined() || doc.IsNull() {
		return false
	}
	canvas := doc.Call("querySelector", "canvas")
	if canvas.IsNull() || canvas.IsUndefined() {
		return false
	}
	ctx := canvas.Call("getContext", "2d")
	if ctx.IsNull() || ctx.IsUndefined() {
		return false
	}
	// Ensure the canvas is sized to the frame.
	canvas.Set("width", width)
	canvas.Set("height", height)

	// putImageData expects RGBA; the readback is BGRA on Deno/browser.
	imgBytes := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		srcRow := y * width * 4
		dstRow := y * width * 4
		for x := 0; x < width; x++ {
			src := srcRow + x*4
			dst := dstRow + x*4
			imgBytes[dst+0] = data[src+2] // R
			imgBytes[dst+1] = data[src+1] // G
			imgBytes[dst+2] = data[src+0] // B
			imgBytes[dst+3] = 255         // A
		}
	}
	// Copy into a JS buffer for putImageData.
	ab := js.Global().Get("ArrayBuffer").New(len(imgBytes))
	js.CopyBytesToJS(ab, imgBytes) //nolint:gocritic // byte-slice -> JS ArrayBuffer
	imgData := js.Global().Get("ImageData").New(
		js.Global().Get("Uint8ClampedArray").New(ab),
		width,
		height,
	)
	ctx.Call("putImageData", imgData, 0, 0)
	slog.Debug("wasm blit present", "width", width, "height", height)
	return true
}

// WasmFrameProduced reports whether gogpu has drawn at least one world frame
// since startup (monotonic counter). The rAF watchdog uses this to detect a
// renderer that is spinning without presenting (busy-loop / deadlock) so it
// can degrade to the headless loop instead of burning CPU + memory.
func (r *Renderer) WasmFrameProduced() bool {
	return statWorldDraws.Load() > 0
}
