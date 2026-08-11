//go:build js && wasm

// wasm_viewstate.go — plan 22 Phase B. The walkthrough inspector needs the
// camera/view state that runtimeViewState computes each frame; that helper is
// unexported, so this wasm-only method exposes it without widening the Game
// API for non-wasm builds.
package game

// WasmViewState returns the current camera origin/angles for the inspector
// bridge (cmd/ironwailgo/inspector_wasm.go). It delegates to the same
// runtimeViewState the renderer uses each frame, so the walkthrough's
// renderer layer panel shows the exact view the engine is drawing.
func (g *Game) WasmViewState() (origin, angles [3]float32) {
	return g.runtimeViewState()
}
