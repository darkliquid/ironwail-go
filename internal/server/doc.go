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
// # Domain structure
//
// Although currently a single package, the server code is organised into
// logical domains. Each file carries a "// This file belongs to the <domain>
// subsystem" header identifying its domain.
//
//	┌──────────────────────┬───────────────────────────────────────────────┐
//	│ Domain               │ Files                                          │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ Physics/Collision    │ physics.go, physics_loop.go, movement.go,       │
//	│                      │ world.go, world_math.go, world_model.go,       │
//	│                      │ synthetic_bsp_helper.go                        │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ Network/Protocol     │ sv_send.go, sv_client.go, message.go,          │
//	│                      │ sv_stats.go, types_protocol.go                 │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ Entity/QC            │ edict.go, entity_accessors.go,                 │
//	│                      │ entity_accessors_vec.go, qc_fields.go,         │
//	│                      │ qc_trace.go, server_qc_sync.go,                 │
//	│                      │ types_entities.go, types_flags.go              │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ Server Lifecycle     │ server.go, sv_main.go, frame.go, sv_pvs.go,    │
//	│                      │ user.go, user_spawn.go, rules.go, skill.go,    │
//	│                      │ spawn_parms.go, types.go, sv_main_skybox.go,   │
//	│                      │ server_runtime.go                              │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ Savegame             │ savegame.go, savegame_text.go                  │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ Debug                │ debug_telemetry.go, debug_trigger.go, svdbg.go│
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ Tests                │ *_test.go                                      │
//	└──────────────────────┴───────────────────────────────────────────────┘
//
// Phase 2 of the modularisation plan (docs/plans/06_phase2_server_split.md)
// will progressively extract these domains into sub-packages.
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
//
// Recent additions include per-frame physics counters for profiling entity
// simulation work, edict count warnings when approaching MAX_EDICTS, autosave
// triggering before level changes, and per-entity gravity fields.
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/server -count=1
package server
