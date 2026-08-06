// Package game consolidates the top-level game state that was previously
// scattered across package-level variables in cmd/ironwailgo/main.go.
//
// # Purpose
//
// The Game struct owns all subsystem references (host, server, client,
// renderer, audio, input, menu, HUD, draw) and all runtime caches (model
// caches, sound caches, dedup keys). Methods on Game implement the per-frame
// update loop, entity collection, audio synchronisation, input routing,
// camera/view computation, command registration, and demo helpers.
//
// # Original C lineage
//
// Mirrors the role of the C host.c Host_Frame function and the global
// state in host.h (host_basepal, host_colormap, host_screenchanged).
// The C engine used global variables for all subsystem pointers; the
// Go port consolidates them into the Game struct for testability and
// explicit dependency injection.
//
// # Role in the engine
//
// The entry point (cmd/ironwailgo/main.go) creates a Game via New(),
// wires the renderer callbacks, and calls Run(). Game then owns the
// entire engine lifecycle: startup, frame loop, shutdown. All other
// packages are either owned by Game or are leaf utility packages.
//
// # Sub-packages
//
// Portable leaf logic has been extracted into focused sub-packages, each
// testing its own math/helpers in isolation; the root `game` package keeps
// the frame orchestration and the delegators those helpers feed:
//
//	┌──────────┬───────────────────────────────────────────────────────────┐
//	│ camera   │ Cvar-only view computation: view bob/roll, idle sway,      │
//	│          │ viewmodel fudge, view angle math, chase camera helpers.   │
//	├──────────┼───────────────────────────────────────────────────────────┤
//	│ csqc     │ Pure CSQC image/rect helpers (nearest palette index,       │
//	│          │ clip draw rect, sub-pic from normalized rect, pic scale). │
//	├──────────┼───────────────────────────────────────────────────────────┤
//	│ audio    │ Pure audio helpers (sound name/volume formatting).        │
//	├──────────┼───────────────────────────────────────────────────────────┤
//	│ ui       │ Pure overlay/UI math (clamp, demo name, format helpers).  │
//	└──────────┴───────────────────────────────────────────────────────────┘
//
// Stateful view calculations that latch on the client (viewCalcGunAngle,
// viewApplyDamageKick, viewStairSmoothOffset) stay on the root because they
// reference `cl.Client` directly.
//
// # Key types
//
//   - [Game] — the top-level coordinator struct owning all subsystems
//   - [GameConfig] — startup configuration (basedir, gamedir, flags)
//   - [RenderFrameState] — per-frame render state passed to the renderer
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/game -count=1
package game
