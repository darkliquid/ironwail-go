//go:build js && wasm

// harness_input.go — a minimal input.Backend for the Deno wasm harness.
//
// The game's input System reads mouse deltas through its Backend; with no
// backend, input.System.State() zeroes the deltas before the game's mouse-look
// code reads them, so injected yaw/pitch would never move the camera. This
// stub returns whatever the harness injected last frame.
package main

import (
	"github.com/darkliquid/ironwail-go/internal/game"
	"github.com/darkliquid/ironwail-go/internal/input"
)

// harnessBackend is the input backend used by the harness build. It reports
// the injected mouse deltas and no key state (keys are injected directly via
// System.HandleKeyEvent).
type harnessBackend struct{}

func (b *harnessBackend) Init() error      { return nil }
func (b *harnessBackend) Shutdown()        {}
func (b *harnessBackend) PollEvents() bool { return true }
func (b *harnessBackend) MouseDelta() (dx, dy int32) {
	return int32(harnessMouseDX), int32(harnessMouseDY)
}
func (b *harnessBackend) MousePosition() (x, y int32, valid bool)    { return 0, 0, false }
func (b *harnessBackend) ModifierState() input.ModifierState         { return input.ModifierState{} }
func (b *harnessBackend) SetTextMode(mode input.TextMode)            {}
func (b *harnessBackend) SetCursorMode(mode input.CursorMode)        {}
func (b *harnessBackend) ShowKeyboard(show bool)                     {}
func (b *harnessBackend) GamepadState(player int) input.GamepadState { return input.GamepadState{} }
func (b *harnessBackend) IsGamepadConnected(player int) bool         { return false }
func (b *harnessBackend) SetMouseGrab(grabbed bool)                  {}
func (b *harnessBackend) SetWindow(win any)                          {}

// harnessMouseDX/DY hold the injected mouse delta for the current frame.
var harnessMouseDX, harnessMouseDY int32

// setHarnessBackend attaches the stub backend to the game's input System so
// injected mouse deltas flow through the normal State() path.
func setHarnessBackend(g *game.Game) {
	if g == nil || g.Input == nil {
		return
	}
	_ = g.Input.SetBackend(&harnessBackend{})
}
