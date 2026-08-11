// playback_control.go — pause/step intent for the engine's frame producers.
//
// Desktop builds never set these (the walkthrough inspector is wasm-only), so
// the values stay at their zero state and every frame runs normally. The wasm
// boot (wasm_frameloop.go) exposes them to the browser via the inspector.
package game

import (
	"sync/atomic"

	cl "github.com/darkliquid/ironwail-go/internal/client"
)

// playbackPaused is 1 when the walkthrough should freeze frame production.
var playbackPaused atomic.Bool

// playbackSteps is the number of frames still to advance while paused (stepping
// advances exactly N frames then re-pauses).
var playbackSteps atomic.Int64

// RunRuntimeFrameUnlessPaused advances one host frame unless paused; a queued
// step drains one frame. Zero-value state (desktop) always runs.
func (g *Game) RunRuntimeFrameUnlessPaused(dt float64, cb gameCallbacks) cl.TransientEvents {
	paused := playbackPaused.Load()
	if paused {
		steps := playbackSteps.Load()
		if steps <= 0 {
			return cl.TransientEvents{}
		}
		playbackSteps.Add(-1)
	}
	return g.RunRuntimeFrame(dt, cb)
}
