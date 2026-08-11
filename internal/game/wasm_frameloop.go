//go:build js && wasm

// wasm_frameloop.go — plan 22 Phase B/C: the wasm walkthrough boot drives host
// frames so the inspector's timing/edict/camera data is live, with pause and
// step-frame controls driven from the page. The frame loop and the browser's
// requestAnimationFrame/tick run on different goroutines, so the control state
// is guarded by a mutex.
package game

import (
	"log/slog"
	"sync"
	"time"
)

// wasmPlayControl is the pause/step state the browser inspector drives.
// Paused freezes the frame loop (state still renders); stepping advances the
// loop by exactly n frames then re-pauses.
type wasmPlayControl struct {
	mu     sync.Mutex
	paused bool
	steps  int // frames still to advance while paused
}

var wasmPlay = &wasmPlayControl{}

// WasmSetPaused toggles the walkthrough frame loop. When paused the loop
// stops advancing Host.Frame but keeps rendering state for the inspector.
func (g *Game) WasmSetPaused(paused bool) {
	wasmPlay.mu.Lock()
	wasmPlay.paused = paused
	if !paused {
		wasmPlay.steps = 0
	}
	wasmPlay.mu.Unlock()
}

// WasmPaused reports whether the walkthrough frame loop is paused.
func (g *Game) WasmPaused() bool {
	wasmPlay.mu.Lock()
	defer wasmPlay.mu.Unlock()
	return wasmPlay.paused
}

// WasmStepFrames advances the loop by exactly n frames then re-pauses
// (queued while the loop is paused or running).
func (g *Game) WasmStepFrames(n int) {
	if n <= 0 {
		return
	}
	wasmPlay.mu.Lock()
	defer wasmPlay.mu.Unlock()
	wasmPlay.paused = true
	wasmPlay.steps += n
}

// RunWasmInspectorLoop advances host frames in a loop for the browser
// walkthrough. It blocks (the wasm main goroutine owns the game), so it must
// be called from main_wasm after the inspector is installed.
func (g *Game) RunWasmInspectorLoop() {
	slog.Info("Ironwail-Go WASM inspector frame loop started")
	last := time.Now()
	for {
		now := time.Now()
		dt := now.Sub(last).Seconds()
		last = now
		if g.Host != nil && g.Host.IsAborted() {
			return
		}

		wasmPlay.mu.Lock()
		runFrame := !wasmPlay.paused
		if wasmPlay.steps > 0 {
			runFrame = true
			wasmPlay.steps--
		}
		wasmPlay.mu.Unlock()

		if runFrame {
			g.RunRuntimeFrame(dt, gameCallbacks{g: g})
		}
		time.Sleep(time.Second / 60)
	}
}
