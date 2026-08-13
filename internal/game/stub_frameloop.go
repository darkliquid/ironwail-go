//go:build !(js && wasm)

package game

// wasmStartPaused is defined in wasm_frameloop.go (js/wasm). On non-wasm the
// shared runtime loop never calls it under the runtime.GOOS == "js" guard,
// but the compiler requires the symbol to exist; no-op keeps both targets
// type-checking.
func (g *Game) wasmStartPaused() {}

// StartWasmRendererFrameLoop is defined in wasm_frameloop.go (js/wasm); the
// browser rAF driver is unused on desktop. Kept as a no-op so the shared
// runtime loop type-checks on both targets.
func (g *Game) StartWasmRendererFrameLoop() {}
