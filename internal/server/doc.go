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
// # Sub-packages
//
// The formerly flat package has been decomposed into focused sub-packages
// over successive refactor passes. Each owns a portable leaf of server
// logic; the root `server` package keeps the orchestration/facade (session
// glue, QC hooks, and delegation seams):
//
//	┌──────────────────────┬───────────────────────────────────────────────┐
//	│ Subpackage           │ Owns                                           │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ types                │ Edict, Client, MessageBuffer, EntityState,     │
//	│                      │ protocol constants, and the narrow interfaces  │
//	│                      │ (PhysicsFacade, FrameDriver, ClientThinker,    │
//	│                      │ CollisionWorld, EntityStore, CVarReader) that  │
//	│                      │ sub-packages consume instead of the root.      │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ physics              │ physics.System: per-entity leaf algorithms      │
//	│                      │ (FlyMove, PushMove, PhysicsWalk/Toss/Step,      │
//	│                      │ Impact, RunThink) plus the frame loop           │
//	│                      │ (StepFrame) dispatching directly to those       │
//	│                      │ leafs. Also ClientMover (per-client move        │
//	│                      │ sim).                                           │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ collision            │ CollisionSystem: BSP model builders, hulls,     │
//	│                      │ areanodes, trace queries.                      │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ net                  │ Wire encoding: entity delta encoding,           │
//	│                      │ WriteClientData, spawn signon writers.          │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ edict                │ Edict allocation, map/savegame parsing, field   │
//	│                      │ accessors.                                     │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ state                │ Session/signon state manager.                   │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ commands             │ Server console command parse/dispatch.          │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ debug                │ Telemetry + svdbg logging.                      │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ qc                   │ Server-side QuakeC hook helpers.                │
//	├──────────────────────┼───────────────────────────────────────────────┤
//	│ savegame             │ Save/load serialization.                        │
//	└──────────────────────┴───────────────────────────────────────────────┘
//
// The root package now retains the facade responsibilities: `Server` owns
// the client/session loops (RunClients, DropClient), message buffers, QCVM
// lifecycle, and the delegators that route the remaining per-entity queries
// to `PhysicsSys`. Root physics files such as server_physics.go and
// server_physics_walk.go hold only the thin *Server delegators whose call
// sites (tests, QC builtins) still reference the root surface; the
// per-movetype frame dispatch no longer bounces through them (StepFrame
// calls System leafs directly).
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
// server-authoritative Quake model intact. Portability of the physics and
// net leaves is improved by the extracted sub-packages: the leaf algorithms
// run against narrow interfaces (types.PhysicsFacade etc.) injected by the
// root, so they can be unit-tested in isolation with mocks.
//
// Recent additions include per-frame physics counters for profiling entity
// simulation work, edict count warnings when approaching MAX_EDICTS, autosave
// triggering before level changes, and per-entity gravity fields.
//
// # Testing
//
//	TMPDIR=./.tmp CGO_ENABLED=0 go test ./internal/server/... -count=1
package server
