//go:build js && wasm

// wasm_dom_input.go — installs the engine's DOM input backend for browsers.
// The renderer normally supplies a gogpu-backed adapter; the DOM backend
// covers the walkthrough's headless fallback (no WebGPU) and also acts as a
// receive-path for window key/mouse events when the renderer loop is active.
package game

import (
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/input"
)

// InstallWasmDOMInput ensures the input system has a working DOM backend. It
// is a no-op when the renderer already installed its gogpu adapter.
func (g *Game) InstallWasmDOMInput() {
	if g.Input == nil {
		return
	}
	if g.Input.Backend() != nil {
		return
	}
	dom := input.NewWASMBackend()
	if err := g.Input.SetBackend(dom); err != nil {
		slog.Warn("wasm input: failed to install DOM backend", "error", err)
	}
}
