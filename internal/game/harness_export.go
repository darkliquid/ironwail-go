//go:build js && wasm

// harness_export.go — build-tagged helpers for the Deno wasm harness
// (cmd/ironwailgo-harness). The harness drives one host frame per exported
// engine_advance call. gameCallbacks is unexported, so these accessors give
// the harness the exact runtime-loop frame wiring without widening the
// desktop Game API.
package game

import (
	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/input"
)

// DriveRuntimeFrame runs one host frame exactly like RunRuntimeFrame does
// (via the same gameCallbacks path) and returns the frame's transient
// events. The harness calls this synchronously from engine_advance.
func (g *Game) DriveRuntimeFrame(dt float64) {
	cb := gameCallbacks{g: g}
	g.RunRuntimeFrame(dt, cb)
}

// HarnessRuntimeStep mirrors the renderer OnUpdate wiring that the browser
// walkthrough relies on, minus the renderer/present concerns: poll input,
// sync gameplay input mode, apply mouse look/menu mouse, then run one host
// frame. The wasm harness calls this per engine_advance so injected mouse
// deltas and key states actually move the camera (the plain DriveRuntimeFrame
// path skips mouse-look because that lives on the renderer callback).
func (g *Game) HarnessRuntimeStep(dt float64) {
	g.pollRuntimeInputEvents()
	if g.Input != nil {
		g.syncGameplayInputMode()
		g.applyMenuMouseMove()
		g.applyGameplayMouseLook()
		g.updateRuntimeTextEditRepeat(dt)
	}
	cb := gameCallbacks{g: g}
	g.RunRuntimeFrameUnlessPaused(dt, cb)
}

var (
	_ host.FrameCallbacks = gameCallbacks{}
	_                     = input.KeyGame
)
