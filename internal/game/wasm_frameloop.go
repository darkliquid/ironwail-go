//go:build js && wasm

// wasm_frameloop.go — plan 22 Phase B: the wasm walkthrough boot drives host
// frames so the inspector's timing/edict/camera data is live. Runs the same
// per-frame work as HeadlessGameLoop but without the ticker/abort plumbing the
// desktop loop needs; 60 targeted steps keep the browser responsive.
package game

import (
	"log/slog"
	"time"
)

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
		g.RunRuntimeFrame(dt, gameCallbacks{g: g})
		time.Sleep(time.Second / 60)
	}
}
