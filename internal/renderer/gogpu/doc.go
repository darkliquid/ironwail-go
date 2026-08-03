// Package gogpu (renderer/gogpu) provides input device mapping and
// backend integration for the GoGPU application framework.
//
// # Purpose
//
// This package bridges the gogpu application framework's input system
// (keyboard, mouse, gamepad events) to Quake's internal/input
// abstraction. It translates platform-specific key codes, mouse
// motion deltas, and button states into Quake's input layer
// (Key_Event, IN_MouseMove, etc.).
//
// # Original C lineage
//
// Mirrors the input handling in gl_vidsdl.c (SDL2 keyboard/mouse
// event processing) and in_main.c (input command building).
// The C version used SDL2 directly; the Go version uses gogpu's
// input package which abstracts over platform backends.
//
// # Role in the engine
//
// Called by the renderer's OnDraw callback to poll input events
// each frame and forward them to the input package (internal/input).
// The input package then feeds into client usercmd generation.
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/renderer/gogpu -count=1
package gogpu
