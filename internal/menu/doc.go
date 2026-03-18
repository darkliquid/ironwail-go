// Package menu implements the game menu state machine, navigation, and menu
// drawing orchestration.
//
// # Purpose
//
// The package manages whether menus are active, which menu screen is selected,
// how cursor movement works, and how menu input redirects away from gameplay.
//
// # High-level design
//
// Manager tracks menu state, cursor position, draw-resource access, and the
// input system's current key destination. It toggles menus on and off, handles
// high-level key presses, and draws menu pictures through a renderer context.
//
// # Role in the engine
//
// This is the front-end UI layer built on top of draw, input, and renderer
// services.
//
// # Original C lineage
//
// The package corresponds to menu.c/menu.h and their M_* state transitions.
//
// # Deviations and improvements
//
// The Go port is intentionally smaller than full Ironwail and currently covers
// the core menu flow rather than every screen and feature from the original.
// An explicit manager object and enum-style state replace one large file full
// of screen-specific statics, which makes navigation logic easier to test.
package menu
