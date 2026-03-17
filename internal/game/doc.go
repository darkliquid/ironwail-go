// Package game consolidates the top-level game state that was previously
// scattered across package-level variables in cmd/ironwailgo/main.go.
//
// The Game struct owns all subsystem references (host, server, client,
// renderer, audio, input, menu, HUD, draw) and all runtime caches (model
// caches, sound caches, dedup keys). Methods on Game implement the per-frame
// update loop, entity collection, audio synchronisation, input routing,
// camera/view computation, command registration, and demo helpers.
//
// The entry point (cmd/ironwailgo/main.go) creates a Game via New(), wires
// the renderer callbacks, and calls Run(). Everything else lives here.
package game
