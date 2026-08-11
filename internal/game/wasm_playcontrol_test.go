//go:build js && wasm

package game

import "testing"

// TestWasmPlayControlStateMachine verifies the pause/step intent logic that
// backs the walkthrough Play/Step buttons on the renderer-driven loop:
// pause freezes frame production, stepping queues exactly n frames (drained
// one per renderer tick), and unpausing clears steps.
func TestWasmPlayControlStateMachine(t *testing.T) {
	g := &Game{}

	// Show zero-state runs (desktop default): not paused, no steps.
	playbackPaused.Store(false)
	playbackSteps.Store(0)

	// Pause freezes.
	g.WasmSetPaused(true)
	if !g.WasmPaused() {
		t.Fatal("pause did not stick")
	}

	// Step 3 queues three frames and re-pauses.
	g.WasmStepFrames(3)
	if !g.WasmPaused() {
		t.Fatal("stepping should re-pause")
	}
	if got := playbackSteps.Load(); got != 3 {
		t.Fatalf("steps = %d, want 3", got)
	}

	// Draining the queue: three calls to the gate consume the three frames.
	cb := gameCallbacks{}
	for i := 0; i < 3; i++ {
		if g.RunRuntimeFrameUnlessPaused(0.016, cb); false {
			break
		}
		// The gate consumes one step per call; verify by inspecting state.
	}
	// The gate ran RunRuntimeFrame each time (g is a bare Game, method
	// tolerates nil Host). The step counter should now be 0.
	if got := playbackSteps.Load(); got != 0 {
		t.Fatalf("after draining steps = %d, want 0", got)
	}

	// Unpause clears any residual queued steps.
	g.WasmStepFrames(5)
	g.WasmSetPaused(false)
	if got := playbackSteps.Load(); got != 0 {
		t.Fatalf("unpause should clear queued steps, got %d", got)
	}
	if g.WasmPaused() {
		t.Fatal("unpause did not clear pause")
	}
}
