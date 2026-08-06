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
// The package exposes the renderer and render-context types; the canonical
// backend is GoGPU/WebGPU (no build tags select backends — the stubbed path
// is selected at the cmd level). It coordinates backend initialization, frame
// callbacks, surface helpers, and screen updates behind a unified package
// surface.
//
// # Sub-packages
//
// Portable leaf logic lives in sub-packages, each testable in isolation; the
// root `renderer` package keeps the GPU state (the flat Renderer struct is
// grouped into embedded domain value structs in renderer_gogpu.go) and the
// frame orchestration. The world sub-package (+ world/gogpu) owns the
// pure-Go geometry/vertex/texture helpers and the WGSL shaders; alias owns
// MDL interpolation; lightmap owns CPU lightmap compositing; decal owns mark
// lifetime/geometry; particle, sky, surface, scrap, oit, pipeline, and
// warpscale round out the remaining leafs. Buffer/texture creation helpers
// live in world/gogpu/resources.go as plain functions taking a wgpu device,
// with root delegators kept for external consumers.
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
// The Go port is more deliberately modular and supports a canonical GoGPU
// backend plus a stub path for headless development and tests. Adapter types,
// backend-neutral callbacks, and clearer package boundaries replace the
// original renderer's tightly intertwined C files.
//
// Recent additions include entity trail events in client_effects.go (rocket
// smoke, blood trails, grenade smoke, etc. dispatched from model flags during
// entity relinking), lightning beam rendering, and decal mark projection.
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/renderer/... -count=1
package renderer
