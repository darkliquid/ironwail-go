//go:build js && wasm

// wasm_frameloop.go — plan 22 Phase B/C: the walkthrough's pause/step controls
// exposed to the browser. The frame production itself is driven by the real
// renderer loop (RunRuntimeRendererLoop → OnUpdate → RunRuntimeFrameUnlessPaused);
// this file only mutates the shared pause/step intent the browser sets through
// window.ironwailInspector.
package game

// WasmSetPaused toggles the walkthrough frame loop. When paused the renderer
// loop keeps drawing but skips advancing Host.Frame, so the inspector shows a
// frozen simulation.
func (g *Game) WasmSetPaused(paused bool) {
	playbackPaused.Store(paused)
	if !paused {
		playbackSteps.Store(0)
	}
}

// WasmPaused reports whether the walkthrough frame loop is paused.
func (g *Game) WasmPaused() bool {
	return playbackPaused.Load()
}

// WasmStepFrames advances the loop by exactly n frames then re-pauses
// (queued while paused).
func (g *Game) WasmStepFrames(n int) {
	if n <= 0 {
		return
	}
	playbackPaused.Store(true)
	playbackSteps.Add(int64(n))
}
