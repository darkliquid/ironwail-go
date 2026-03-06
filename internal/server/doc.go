// Package server implements the running Quake game world, including edicts,
// physics, spatial queries, messaging, and QuakeC integration.
//
// # Purpose
//
// The package owns the authoritative simulation for a map: entities,
// movement, collision, precaches, clients, and per-frame game logic.
//
// # High-level design
//
// Server, ServerStatic, and Edict hold the main state, with helper files for
// movement, physics loops, world traces, user commands, and message buffers.
// At runtime the server loads world data, exposes builtin hooks to the QuakeC
// VM, advances physics, and emits messages that clients later parse.
//
// # Role in the engine
//
// This package is the gameplay authority between filesystem/model loading
// below and host/client orchestration above.
//
// # Original C lineage
//
// The direct counterparts are sv_main.c, sv_phys.c, sv_user.c, world.c,
// server.h, and the surrounding Quake server/physics code.
//
// # Deviations and improvements
//
// The Go port isolates the server into its own package and makes VM hooks,
// messaging, and shared world data explicit instead of relying on implicit
// global cross-calls. Typed structs, slices and maps, and ordinary errors
// replace much of the original pointer-heavy plumbing while keeping the
// server-authoritative Quake model intact.
package server