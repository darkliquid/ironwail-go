//go:build js && wasm

package game

import "testing"

// TestWasmPlayControlStateMachine verifies the pause/step intent logic that
// backs the walkthrough Play/Step buttons: pause freezes, stepping queues
// exactly n frames (drained one per loop tick), and unpausing clears steps.
func TestWasmPlayControlStateMachine(t *testing.T) {
	g := &Game{}

	// Start running.
	g.WasmSetPaused(false)
	if g.WasmPaused() {
		t.Fatal("should start running")
	}

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
	if wasmPlay.steps != 3 {
		t.Fatalf("steps = %d, want 3", wasmPlay.steps)
	}

	// Draining one frame per loop tick consumes the queue.
	consume := func() {
		wasmPlay.mu.Lock()
		defer wasmPlay.mu.Unlock()
		if wasmPlay.steps > 0 {
			wasmPlay.steps--
		}
	}
	consume()
	if wasmPlay.steps != 2 {
		t.Fatalf("after one consume steps = %d, want 2", wasmPlay.steps)
	}
	consume()
	consume()
	if wasmPlay.steps != 0 {
		t.Fatalf("after draining steps = %d, want 0", wasmPlay.steps)
	}

	// Unpause clears any residual queued steps.
	g.WasmStepFrames(5)
	g.WasmSetPaused(false)
	if wasmPlay.steps != 0 {
		t.Fatalf("unpause should clear queued steps, got %d", wasmPlay.steps)
	}
	if g.WasmPaused() {
		t.Fatal("unpause did not clear pause")
	}
}
