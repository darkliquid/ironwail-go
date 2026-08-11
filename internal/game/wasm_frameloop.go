//go:build js && wasm

// wasm_frameloop.go — plan 22 Phase B/C: the walkthrough's pause/step controls
// plus the headless frame loop used when the browser lacks WebGPU.
package game

import (
	"log/slog"
	"time"
)

// WasmSetPaused toggles the walkthrough frame loop. When paused the frame loop
// keeps ticking for the inspector but skips advancing Host.Frame.
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

// RunWasmHeadlessLoop is the no-WebGPU walkthrough fallback: it drives host
// frames at ~60 Hz for the data panels (the renderer is absent, so there is no
// canvas). pause/step still apply via the shared gate.
func (g *Game) RunWasmHeadlessLoop() {
	slog.Info("Ironwail-Go WASM headless inspector loop started (no WebGPU)")
	last := time.Now()
	cb := gameCallbacks{g: g}
	for {
		now := time.Now()
		dt := now.Sub(last).Seconds()
		last = now
		if g.Host != nil && g.Host.IsAborted() {
			return
		}
		g.RunRuntimeFrameUnlessPaused(dt, cb)
		time.Sleep(time.Second / 60)
	}
}
