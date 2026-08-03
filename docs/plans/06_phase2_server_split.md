# Phase 2: Split internal/server/ into Sub-Packages

## Current State

The `internal/server/` package has **73 Go files** totaling ~28,000 lines,
all in a single `package server`. The `Server` struct owns everything:
edicts, physics, collision, networking, savegame, QuakeC hooks, entity
state, client management, map loading, and debug telemetry.

### File Grouping by Logical Domain

| Domain | Files | Lines | Key Types |
|--------|-------|-------|-----------|
| Physics/Collision | physics.go, physics_loop.go, movement.go, world.go, world_math.go, world_model.go, synthetic_bsp_helper.go | ~4,500 | TraceResult, MoveType, hulls |
| Network/Protocol | sv_send.go, sv_client.go, message.go, sv_stats.go, types_protocol.go | ~3,400 | MessageBuffer, NetMessageType |
| Entity/QC | edict.go, entity_accessors.go, entity_accessors_vec.go, qc_fields.go, qc_trace.go, server_qc_sync.go, types_entities.go, types_flags.go | ~3,200 | Edict, EntityState |
| Server Lifecycle | server.go, sv_main.go, frame.go, sv_pvs.go, user.go, user_spawn.go, rules.go, skill.go, spawn_parms.go, types.go | ~5,000 | Server, ServerStatic |
| Savegame | savegame.go, savegame_text.go | ~800 | — |
| Debug | debug_telemetry.go, debug_trigger.go, svdbg.go | ~1,000 | — |
| Tests | ~30 _test.go files | ~12,000 | — |

### Coupling Analysis

The `Edict` type is used **104 times** across physics, world, sv_send,
sv_client, and movement — it's the core shared type. The `Server` struct
is passed as `s *Server` to nearly every function. Entity accessors
(`entity_accessors.go`, 170+ methods) take `*Server` as their first
parameter.

This tight coupling means we **cannot** simply move files into
sub-packages — the sub-packages would need to import `server` for the
`Server` and `Edict` types, creating import cycles if `server` also
imports the sub-packages.

## Strategy: Interface-Based Extraction

Instead of moving files physically, we extract **interfaces** that define
the contract between the server and each sub-domain. Sub-packages
depend on the interface, not the concrete `Server` struct.

### Key Interfaces to Define

```go
// PhysicsWorld is the contract between the server and the physics
// subsystem. The server implements this; physics code depends on it.
type PhysicsWorld interface {
    Edicts() []*Edict
    EdictNum(num int) *Edict
    NumEdicts() int
    Gravity() float32
    MaxVelocity() float32
    // ... trace and link methods
}

// CollisionWorld is the contract for BSP collision queries.
type CollisionWorld interface {
    SV_Trace(start, mins, maxs, end [3]float32, moveType MoveType, passEntity *Edict) TraceResult
    SV_Move(start, mins, maxs, end [3]float32, moveType MoveType, passEntity *Edict) TraceResult
    SV_LinkEdict(edict *Edict)
    SV_TouchLinks(edict *Edict)
    PointInLeaf(pos [3]float32) int
    LeafContents(leafIndex int) int
}

// NetEncoder is the contract for network message encoding.
type NetEncoder interface {
    WriteEntitiesToClient(client *Client, buf *MessageBuffer)
    WriteClientDataToMessage(client *Client, buf *MessageBuffer)
    WriteDelta(buf *MessageBuffer, from, to *EntityState, entityNum int)
}
```

## Revised Phase 2 Plan

Given the tight coupling, a full physical split is high-risk and would
touch every file. Instead, we take an **incremental approach**:

### Step 2.1: Document the Domain Boundaries (Low Risk)

Add a file-level `// This file belongs to the <domain> subsystem` header
to every server file, clarifying which logical domain it belongs to.
This makes the structure visible without any code movement.

**Actions:**
- Add domain header comments to all 73 files
- Update `internal/server/doc.go` to describe the domain structure
- No code changes, no test changes

**Verification:** `mise run build && mise run test`

### Step 2.2: Extract Types into Shared Types Package (Low Risk)

Move shared types (`Edict`, `EntityState`, `TraceResult`, `MoveType`,
`SolidType`, etc.) into a new `internal/server/types/` package that
both `server` and future sub-packages can import without cycles.

**Target Structure:**
```
internal/server/
├── types/
│   ├── doc.go          # Shared types package
│   ├── entity.go       # Edict, EntityState
│   ├── flags.go        # MoveType, SolidType, DeadFlag, TakeDamage
│   ├── trace.go        # TraceResult, hull types
│   └── protocol.go     # NetMessageType, ClientState, SignonStage
```

**Actions:**
- Create `internal/server/types/` package
- Move `types.go`, `types_entities.go`, `types_flags.go`,
  `types_protocol.go` content into the new package
- The `server` package re-exports via type aliases for backward
  compatibility: `type Edict = types.Edict`
- All external callers (game, host) continue using `server.Edict`
  via the alias — no changes needed outside the server package

**Verification:** `mise run build && mise run test`

### Step 2.3: Extract Savegame Sub-Package (Low Risk)

Savegame is the most self-contained domain — it only needs `Server`,
`Edict`, and `MessageBuffer`, and has no callers from other server
files (only called from `host/commands_gameplay_save.go`).

**Target Structure:**
```
internal/server/
├── savegame/
│   ├── doc.go
│   ├── savegame.go       # SaveGame, LoadGame
│   └── savegame_text.go  # Text serialization helpers
```

**Actions:**
- Create `internal/server/savegame/` package
- Move `savegame.go` and `savegame_text.go` into it
- Move `savegame_test.go` and `savegame_text_test.go` into it
- The sub-package imports `internal/server` for `Server` and `Edict`
- The `host` package updates its import from `server.SaveGame` to
  `server/savegame.SaveGame`

**Verification:** `mise run build && mise run test`

### Step 2.4: Extract Debug Sub-Package (Low Risk)

Debug telemetry is self-contained — it only reads server state and
emits log lines. No other server code calls into the debug functions
(except the server's own `SV_Physics` and `SV_TouchLinks` which call
telemetry hooks).

**Target Structure:**
```
internal/server/
├── debug/
│   ├── doc.go
│   ├── telemetry.go     # Telemetry engine, event masks
│   ├── trigger.go       # Trigger touch debugging
│   └── svdbg.go         # Multiplayer debug logging
```

**Actions:**
- Create `internal/server/debug/` package
- Move `debug_telemetry.go`, `debug_trigger.go`, `svdbg.go` into it
- Move corresponding `_test.go` files
- Server calls into debug via an interface or function pointer
  (avoiding import cycle)

**Verification:** `mise run build && mise run test`

### Step 2.5: Extract Physics Interface (Medium Risk)

Define a `PhysicsWorld` interface and refactor physics functions to
take the interface instead of `*Server`. This is the biggest win
for testability — physics tests can mock the interface without
spinning up a full server.

**Actions:**
- Define `PhysicsWorld` interface in `internal/server/physics/` or
  keep in `server` but as a standalone interface type
- Refactor `SV_FlyMove`, `SV_WalkMove`, `SV_PushMove` to take
  `PhysicsWorld` instead of `*Server`
- Keep physics files in `package server` for now (physical move
  deferred to Phase 3 when the Renderer is split)
- Update tests to use mock `PhysicsWorld` where possible

**Verification:** `mise run test` — physics tests pass, behavior
unchanged

### Step 2.6: Extract Collision Interface (Medium Risk)

Define a `CollisionWorld` interface for trace/link/touch functions.
Refactor `SV_Trace`, `SV_LinkEdict`, `SV_TouchLinks` to use the
interface.

**Actions:**
- Define `CollisionWorld` interface
- Refactor trace functions to take `CollisionWorld`
- The `Server` struct implements both `PhysicsWorld` and
  `CollisionWorld`
- Tests can use the `synthetic_bsp_helper.go` to construct a
  mock `CollisionWorld`

**Verification:** `mise run test` — world/collision tests pass

### Step 2.7: Extract Network Encoder Interface (Medium Risk)

Define a `NetEncoder` interface for the send/clientdata functions.
This makes the protocol encoding testable in isolation.

**Actions:**
- Define `NetEncoder` interface
- Refactor `WriteEntitiesToClient`, `WriteClientDataToMessage` to
  take the interface or be standalone functions taking explicit
  parameters instead of `*Server`
- Update tests

**Verification:** `mise run test` — sv_send tests pass

## Risk Assessment

| Step | Risk | Files Changed | Tests Affected |
|------|------|---------------|----------------|
| 2.1 | Zero | 73 (comments only) | 0 |
| 2.2 | Low | ~8 (move + aliases) | 0 (aliases preserve API) |
| 2.3 | Low | ~5 (savegame) | ~2 (savegame tests) |
| 2.4 | Low | ~6 (debug) | ~3 (debug tests) |
| 2.5 | Medium | ~5 (physics refactor) | ~10 (physics tests) |
| 2.6 | Medium | ~4 (collision refactor) | ~5 (world tests) |
| 2.7 | Medium | ~4 (netcode refactor) | ~5 (send tests) |

## Constraints

- **Parity:** All physics and netcode behavior must not change
- **No build tags:** No `//go:build` directives
- **CGO off:** `CGO_ENABLED=0` always
- **File length:** All files under 1000 lines
- **Import cycles:** Sub-packages import `server` (or `server/types`),
  never the reverse. The `server` package may import sub-packages.
- **QuakeGo exempt:** `pkg/qgo/quakego` is not touched

## Execution Order

1. **Step 2.1** (domain headers) — zero risk, immediate clarity
2. **Step 2.2** (shared types) — enables future sub-packages
3. **Step 2.3** (savegame) — most self-contained, proves the pattern
4. **Step 2.4** (debug) — second most self-contained
5. **Step 2.5** (physics interface) — biggest testability win
6. **Step 2.6** (collision interface) — pairs with physics
7. **Step 2.7** (netcode interface) — protocol testability

Each step is independently committable. Steps 2.1-2.4 are low risk
and can be done quickly. Steps 2.5-2.7 require more careful testing.
