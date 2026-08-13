//go:build js && wasm

// wasm_frameloop.go — plan 22 Phase B/C: the walkthrough's pause/step controls
// plus the headless frame loop used when the browser lacks WebGPU.
package game

import (
	"log/slog"
	"syscall/js"
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

// wasmStartPaused sets the walkthrough frame loop to start paused. It is
// called from the shared runtime loop before the browser rAF driver takes
// over, so the synthetic room loads frozen; the user steps frame-by-frame
// with the walkthrough's Play/Step controls (WasmSetPaused/WasmStepFrames).
func (g *Game) wasmStartPaused() { g.WasmSetPaused(true) }

// StartWasmRendererFrameLoop is the browser frame driver. gogpu's App.Run
// main loop must NOT run in wasm (its WaitEvents no-ops and ContinuousRender
// becomes a hot loop with no rAF — that starves the page event loop and locks
// the tab). This rAF loop is the sole driver: per animation frame it runs the
// engine update through StepWasmFrame (which calls the registered OnUpdate
// and requests a redraw), keeping the browser responsive.
func (g *Game) StartWasmRendererFrameLoop() {
	slog.Info("starting continuous WASM WebGPU frame loop (requestAnimationFrame); engine paused at boot — use Play/Step to advance")
	window := js.Global().Get("window")
	if window.IsUndefined() || window.IsNull() {
		slog.Warn("WASM renderer loop: window object unavailable")
		return
	}

	last := time.Now()
	var frameFunc js.Func

	// watchdog: if the renderer never produces a GPU world frame within this
	// many rAF callbacks, stop it and degrade to the headless loop. gogpu's
	// wasm App.Run can busy-loop (WaitEvents no-op) and leak memory without
	// ever presenting; this bounds the damage so the walkthrough stays usable.
	const watchdogFrames = 120 // ~2s at 60fps
	var idleSince = -1

	frameFunc = js.FuncOf(func(this js.Value, args []js.Value) any {
		if g.Host != nil && g.Host.IsAborted() {
			frameFunc.Release()
			return nil
		}

		now := time.Now()
		dt := now.Sub(last).Seconds()
		if dt > 0.1 {
			dt = 0.1
		}
		last = now

		// Step update callback (runs input, physics, srvTime, and stores
		// frame state) and request WebGPU redraw.
		if g.Renderer != nil {
			g.Renderer.StepWasmFrame(dt)

			// Best-effort CPU present: when a world frame is available, blit
			// it onto the canvas so the viewport shows real engine output
			// even if gogpu's swapchain never presents on wasm.
			if rr, ok := g.Renderer.(interface{ WasmBlitPresent() bool }); ok {
				if rr.WasmBlitPresent() || g.WasmPaused() {
					idleSince = -1
				} else if idleSince < 0 {
					idleSince = 0
				} else {
					idleSince++
				}
			}

			// Watchdog: no frame produced for too long while playing => renderer is
			// spinning without presenting. Stop it and run the headless
			// inspector loop (frames/panels keep working).
			if idleSince > watchdogFrames {
				slog.Warn("WASM renderer produced no frames while playing — stopping and degrading to headless loop")
				g.RecordWasmTelemetry("renderer", "watchdog degraded renderer to headless loop after timeout")
				g.Renderer.Stop()
				g.Renderer = nil
				frameFunc.Release()
				go g.RunWasmHeadlessLoop()
				return nil
			}
		}

		window.Call("requestAnimationFrame", frameFunc)
		return nil
	})

	window.Call("requestAnimationFrame", frameFunc)
}

