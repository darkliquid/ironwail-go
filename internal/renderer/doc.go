// Package renderer provides the engine's rendering abstraction and
// backend-specific implementations.
//
// # Purpose
//
// The package owns backend setup, frame presentation, drawing callbacks, and
// shared helpers for world, model, particle, texture, and screen rendering.
//
// # High-level design
//
// Build tags select concrete implementations such as WebGPU, OpenGL, or a stub
// backend, while the package exposes common renderer and render-context types.
// It coordinates backend initialization, frame callbacks, surface helpers, and
// screen updates behind a unified package surface.
//
// # Role in the engine
//
// This is the visual output subsystem connected to host, draw, HUD, menu,
// model, and input/window integration.
//
// # Original C lineage
//
// The corresponding Ironwail/Quake concepts span gl_vidsdl.c, gl_screen.c,
// gl_draw.c, gl_model.c, gl_rmain.c, gl_sky.c, gl_texmgr.c, and particle/model
// rendering code across the renderer.
//
// # Deviations and improvements
//
// The Go port is more deliberately modular and supports multiple backends with
// pure-Go build-tag selection, including a stub path for headless development
// and tests. Adapter types, backend-neutral callbacks, and clearer package
// boundaries replace the original renderer's tightly intertwined C files.
//
// Recent additions include entity trail events in client_effects.go (rocket
// smoke, blood trails, grenade smoke, etc. dispatched from model flags during
// entity relinking), lightning beam rendering, and decal mark projection.
package renderer
