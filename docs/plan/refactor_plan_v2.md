# Refactor Plan v2 — Subpackage Extraction (Corrected)

This plan supersedes the earlier `subpackage_*`, `deep_*`, `architectural_*`, and
`aggressive_*` plan docs. Those docs describe an ideal end state but their
"move these files" steps are **not directly executable** against the current
code. This document records the real state, the actual coupling obstacles, and
a corrected, parity-preserving migration strategy.

---

## 1. Ground truth (audited 2026-08-05)

### Package sizes

| Package | Root `.go` files | Tests | Subpackages present |
| :--- | :--- | :--- | :--- |
| `internal/server` | 76 | mixed | `collision`, `commands`, `debug`, `edict`, `net`, `physics`, `qc`, `savegame`, `state`, `types` |
| `internal/renderer` | 108 (72 non-test) | 36 | `alias`, `decal`, `gogpu`, `lightmap`, `oit`, `overlay`, `particle`, `pipeline`, `scrap`, `sky`, `surface`, `warpscale`, `world` |
| `internal/game` | 49 (28 non-test) | 21 | `camera`, `commands`, `ui` |

### Critical finding: the plan docs assume a false premise

The earlier docs assume the target files are self-contained and can be moved
into subpackages. In reality:

1. **The root `*Server` methods are the live implementation.** The frame loop
   (`s.Physics()`, `SV_ExecuteUserCommand`, `syncQCVMState`, multicast
   encoding) all live as methods on `*Server` and read private state
   (`s.QCVM`, `s.CVar`, `s.Edicts`, `s.Static`, `s.DebugTelemetry`) plus
   private methods (`s.playerClient()`, `s.runClientQCThinkWithMode`).
2. **Moving them is impossible** (Go circular-import rule): a subpackage
   (`server/physics`) cannot import its parent (`internal/server`). Any
   function operating on `*Server` must stay in `package server`.
3. **Root and subpackage are duplicated.** e.g. `server_physics.go` has its own
   `checkVelocity`, `addGravity`, `FlyMove`, `PhysicsWalk`, `PhysicsToss`,
   `PushEntity`, `Impact`, `ClipVelocity` — parallel reimplementations of the
   DI-free functions already in `physics/physics.go`. The root copy is the one
   actually driven by `s.Physics()`; the subpackage copy is only wired into the
   `movement.go` helpers (`CheckBottom`, `MoveStep`, `StepDirection`,
   `MoveToGoal`, `NewChaseDir`).

### Which subpackages are real vs. thin wrappers

| Subpackage | Status | Notes |
| :--- | :--- | :--- |
| `server/collision` | **Real, substantively implemented** | Area nodes, hulls, traces, DI-driven. Root `world.go` already delegates via `ensureCollisionSys()`. Only `touchLinks` (QC) and `areaTriggerEdicts` remain root-specific. |
| `server/edict` | **Real, substantively implemented** | Allocation, free lists, field parsing. Root `edict_compat.go` only holds parse helpers (`parseVec3`, `parseFloat32`, `normalizeFieldName`) duplicated from `edict/fields.go`. |
| `server/debug`, `server/state`, `server/savegame` | **Real** | Extracted previously. |
| `server/physics` | **Thin wrapper + partial port** | `System` wraps movement helpers; `physics/physics.go` reimplements leaf math but is NOT wired into the frame loop. |
| `server/net` | **Thin wrapper** | `NetworkManager` only forwards `StartParticle`/`StartSound`; real `server_net_*.go`/`sv_*.go`/`message.go` stay in root. |
| `server/commands` | **Thin wrapper** | `Handler` only lists allowed commands; real `SV_ExecuteUserCommand`/`RunClients`/`DropClient` stay in root. |
| `server/qc` | **Thin wrapper** | `Binding` only wraps `vm.ExecuteFunction`; real `syncQCVMState`/`syncQCVMGlobals`/`executeQCFunction` stay in root. |

### External importers (who consumes subpackages)

- `server/*` subpackages: imported **only** by `package server` (none imported
  from outside `internal/server`). Safe to reshape freely.
- `renderer/*` subpackages: only `world` is imported externally
  (`internal/game/game_init.go`, `game_commands_camdebug.go`); `alias`,
  `overlay`, `particle`, `pipeline`, `scrap`, `sky`, `surface`, `warpscale`,
  `decal`, `lightmap`, `oit`, `gogpu` have **zero external importers** — they
  are internal to the renderer facade.
- `game/*` subpackages (`camera`, `commands`, `ui`): **zero external
  importers**.

---

## 2. The correct migration strategy

Because root methods cannot move into subpackages, the end state is NOT "move
files." It is a two-step **delegation + deletion** refactor that converts root
methods into thin facade methods that forward to DI subpackage components, then
deletes the duplicated root implementations.

### Universal pattern (per domain)

1. **Widen the DI contract** in `internal/server/types` (or the target
   subpackage) so the subpackage function/component has everything it needs via
   interfaces — no `*Server` pointer.
2. **Convert the root `*Server` method** into a delegator that calls the
   subpackage component (e.g. `physics.System`), passing itself as the injected
   interface.
3. **Delete the duplicated root logic** (the parallel `checkVelocity`,
   `addGravity`, `FlyMove`, etc. that now live — or will live — in the
   subpackage).
4. When a root method needs private state that the subpackage contract cannot
   express (e.g. `playerClient`, `runClientQCThinkWithMode`, telemetry), keep
   the orchestration in root and pass the leaf as a **callback** into the
   subpackage, OR keep that specific method in root and only extract the
   pure-leaf math.

### The frame-loop problem

`Physics()` (the edict iteration + movetype dispatch) is the hardest to extract
because it touches `s.QCVM`, `s.CVar`, `s.Edicts`, `s.Static`, telemetry, and
private client-think methods. Options, in order of preference:

1. **Keep the loop in root, extract the leafs.** Move `PhysicsPusher`,
   `PhysicsNone`, `PhysicsNoClip`, `PhysicsStep`, `PhysicsToss`,
   `PhysicsWalk`, `PhysicsWalk` internals into `physics.System`, and have
   `Physics()` call `s.PhysicsSys.<Leaf>(ent)`. This is the lowest-risk first
   pass and immediately deletes the duplicated math.
2. Later, if the loop's remaining dependencies (`playerClient`,
   `runClientQCThinkWithMode`, `DebugTelemetry`) are themselves reduced to
   interfaces, the whole loop can move into `physics.System` as `StepFrame`.

---

## 3. Phase plan (revised, executable)

Each phase ends with `mise run verify` (test + build). Phases are ordered by
risk (lowest first) to bank verified wins and refine the pattern.

### Phase A — `server/edict` (lowest risk)

- Root `edict_compat.go` parse helpers (`parseVec3`, `parseFloat32`,
  `parseInt32`, `normalizeFieldName`, `parseStringFallbackInt32`) are already
  duplicated in `edict/fields.go`. **Delete** the root copies and route callers
  through `edict` exports.
- Verify no root file still references the deleted helpers.

### Phase B — `server/collision` (mostly done)

- Root `world.go` already delegates traces to `collision.System`. Remaining
  root-specific logic: `touchLinks` (QC touch callbacks) and
  `areaTriggerEdicts`. Extract these into `collision` by passing a `TouchFunc`
  callback for the QC coupling, or keep them in root. Prefer: add a
  `TouchProvider` callback to `collision.System` so `areaTriggerEdicts` /
  `touchLinks` become subpackage-owned.
- Delete duplicated `recursiveHullCheck`/`hullPointContents`/`boxOnPlaneSide`
  in root if they are now pure delegators (they already are).

### Phase C — `server/physics` (highest-value, medium risk)

- **Step C1**: Move the leaf movetype dispatchers into `physics.System`:
  `PhysicsPusher`, `PhysicsNone`, `PhysicsNoClip`, `PhysicsStep`,
  `PhysicsToss`, `PhysicsWalk`, plus `SV_WalkMove`, `SV_WallFriction`,
  `SV_CheckStuck`, `CheckWaterTransition`, `PhysicsWalk` internals.
- **Step C2**: Delete the root duplicate math (`checkVelocity`, `addGravity`,
  `FlyMove`, `PushEntity`, `Impact`, `ClipVelocity` free functions) and make
  root `*Server` methods delegate to `s.PhysicsSys`.
- **Step C3** (deferred): if the loop deps can be reduced to interfaces, move
  `Physics()` itself into `System.StepFrame`.

### Phase D — `server/commands`

- Root `SV_ExecuteUserCommand`, `executeClientStringCommand`,
  `handleClientStringCommand`, `parseClientNameCommand`,
  `parseClientColorCommand`, `clientStringCommandVerb/Args` are the command
  dispatch core. These read `*Client` and `*Server` heavily.
- Extract the pure string-parsing helpers (`clientStringCommandVerb`,
  `clientStringCommandArgs`, `parseClientNameCommand`,
  `parseClientColorCommand`) into `commands` first (pure, testable).
- Keep `SV_ExecuteUserCommand`/`RunClients`/`DropClient` in root; they are
  tightly coupled to client/net and are low-value to extract.

### Phase E — `server/qc`

- Root `syncQCVMState`/`syncQCVMGlobals`/`executeQCFunction`/
  `cacheQCFieldOffsets` are the real VM-sync work; `qc.Binding` is a thin
  wrapper.
- Extract the pure VM-field-offset mapping (`cacheQCFieldOffsets`,
  `EdictDefaultOffsets`) into `qc` and have root delegate.
- Keep `syncQCVMState`/`syncQCVMGlobals` in root (they need `*Server` edict
  access) OR widen `EntityStore` to support the sync and move them.

### Phase F — `server/net`

- Root `server_net_main.go`, `server_net_send.go`, `sv_client.go`, `sv_pvs.go`,
  `sv_stats.go`, `message.go` are the real network stack. These are the most
  coupled (client signon, PVS, datagram buffers, `MessageBuffer`).
- Highest-risk phase. Extract pure serialization (entity delta encoding,
  `WriteEntitiesToClient`) into `net` behind a `MessageWriter` interface, then
  delegators. Keep signon/client lifecycle in root.

### Phase G — `internal/renderer` (largest root)

- The renderer subpackages (`alias`, `overlay`, `particle`, `pipeline`, `scrap`,
  `sky`, `surface`, `warpscale`, `decal`, `lightmap`, `oit`, `gogpu`) have
  **zero external importers**; only `world` is imported by `game`.
- The facade `renderer.Renderer` (~200+ fields) is the coupling point. The
  `renderer/interfaces.go` `WorldPipeline`/`AliasBatcher`/`Compositor2D`/
  `GPUContext` contracts already exist.
- Extract by **delegation**, same pattern as server: root `Renderer` methods
  become thin forwarders to `world.Pipeline`, `alias.Batcher`,
  `overlay.Compositor2D`, `particle.System`, `warpscale.Pipeline`.
- `renderer/world` is the only externally-consumed subpackage; keep its public
  API stable.

### Phase H — `internal/game` (smallest root)

- Subpackages `camera`, `commands`, `ui` have zero external importers.
- `game/interfaces.go` already defines `SessionManager`, `CameraSystem`,
  `AssetCache`, `UIController`.
- Extract `game_camera*.go` into `camera`, `game_commands_*.go` into
  `commands`, `game_runtime_*.go`/`game_loop.go` into a new `runtime`,
  `game_audio*.go` into a new `audio`, `game_runtime_csqc*.go` into a new
  `csqc` — all by delegation, keeping `Game` as the facade.

---

## 4. Verification gates

- After every sub-step: `mise run verify` (test + build) and
  `go test ./internal/<pkg>/...`.
- Physical move (git mv) only after the delegation + deletion refactor for that
  domain is green, so each stage is a pure move with no behavior change.
- Re-run `smoke-all` after the server and renderer phases to confirm no
  behavioral parity regression.

## 5. Key risks

1. **Duplicate implementations drifting**: root and subpackage both maintain
   `FlyMove` etc. The only safe resolution is deliberate deletion of the root
   copy after wiring the delegator. Do NOT run both.
2. **Circular imports**: never make a subpackage import its parent. If a
   function needs `*Server`, it must stay in root or be passed a callback.
3. **Parity regression**: every deleted root method must have a live delegator
   producing identical behavior. Tie deletions to the parity tests
   (`frame_physics_parity_test.go`, `physics_parity_test.go`).
4. **`renderer/world` is externally consumed** — changing its public API breaks
   `internal/game`. Freeze it during Phase G.

---

## 6. Execution log

### Phase A — `server/edict` (DONE 2026-08-05)
- Deleted dead root parse helpers in `edict_compat.go`: `parseFloat32`,
  `parseInt32`, `parseStringFallbackInt32`, and the `stringEntFieldNames` map
  (all already duplicated in `edict/fields.go` / `savegame/savegame.go`).
- Kept `normalizeFieldName` (used by `server_qc_sync.go`) and `parseVec3`
  (used by `walkable_point_diagnostics_test.go`).
- Build + all server tests green.

### Phase B — `server/collision` (DONE 2026-08-05)
- Root `world.go` already delegates trace math to `collision.System`; the only
  root-specific parts are `touchLinks` (QC touch callback orchestration) and
  `areaTriggerEdicts` (thin wrapper) — correctly left in root (QC coupling not
  worth extracting behind a callback).
- Deleted dead `world_math.go`: `Vec3Len`, `Vec3Normalize`, `boxOnPlaneSide`
  had zero references (collision subpackage has its own `boxOnPlaneSide`).
- Build + all server tests green.

### Phase C — `server/physics` (DONE 2026-08-05, strategy corrected)
**Key correction to the original plan**: the subpackage `physics/physics.go`
leaf functions (`CheckVelocity`, `AddGravity`, `SV_CheckWater`, `PushEntity`,
`Impact`, `ClipVelocity`, `FlyMove`, `PhysicsWalk`, `PhysicsToss`,
`PhysicsNoclip`, `PhysicsNone`) were **dead code** — behaviorally-divergent
simplified ports that nothing called. The LIVE implementations live on
`*Server` in `server_physics*.go` and are behaviorally richer (player pre/post
think, `SV_CheckStuck`, WaterJump, angles/avel, bounce backoff,
`CheckWaterTransition`). Deleting root in favor of the subpackage would have
broken parity.

**What was done instead** (delete the inferior duplicate):
- Deleted `physics/physics.go` (the dead leaf cluster).
- Removed the unused `System` methods `CheckVelocity`, `AddGravity`,
  `SV_CheckWater`, `PushEntity`, `FlyMove` from `physics/system.go`.
- Removed the now-dead `cfg`, `timing`, `exec` deps from `System`/`NewSystem`
  (retained `MovementEngine` surface only needs `col`, `store`, `sh`).
- Updated `NewPhysicsSystem` (root) and the two call sites to the 3-arg form.
- Build + all server tests green; full module builds.

**Net effect**: `server/physics` is now a lean, honest `MovementEngine`
component (CheckBottom/MoveStep/StepDirection/MoveToGoal/NewChaseDir) with no
duplicated or dead physics math. The live frame-loop physics stays in root
`server_physics*.go` where it belongs (it cannot move without circular
imports).

### Phase D — `server/commands` (DONE 2026-08-05)
- Extracted the four pure client command string-parsing helpers into
  `commands/parse.go` as exported functions: `ClientStringCommandVerb`,
  `ClientStringCommandArgs`, `ParseClientNameCommand`, `ParseClientColorCommand`.
- Root `server_user_commands.go` now delegates to `srvcmds.*` (the local
  helpers became thin wrappers); removed the now-unused `strconv` import.
- Added `parse_test.go` covering verb/args/name/color parsing.
- `commands` subpackage was previously a dead wrapper (its `Handler`/
  `AllowedCommands` are referenced by nobody); it now carries real, tested
  logic. Build + all server tests green.

### Phase E — `server/qc` (DONE 2026-08-05)
- Moved the live `defaultEntFieldOffsets` table and `normalizeFieldName` into
  `server/qc/offsets.go` as exported `DefaultEntFieldOffsets()` and
  `NormalizeFieldName()`.
- Root `server_qc_sync.go` `EdictDefaultOffsets()` now delegates to
  `srvqc.DefaultEntFieldOffsets()`; `savegame_server.go` and `server.go`
  call sites updated to the delegator.
- Deleted the dead `Binding` stub (`binding.go` + `binding_test.go`): its
  `RunThink` unconditionally returned false (would break parity if wired) and
  nothing referenced it.
- Added `offsets_test.go` covering the field table and name normalization.
- `server/qc` is now a real, tested home for VM field-offset data. Build + all
  server tests green.

### Phase F — `server/net` (DONE 2026-08-05, minimal)
- The `net` subpackage's `NetworkManager` delegator is **wired and live**
  (server.go constructs it, `interfaces_test.go` asserts it satisfies
  `NetworkBroadcaster`) — unlike the dead `qc.Binding`/`commands.Handler`
  stubs. No change needed there.
- Deleted the dead `coordWireSize`/`angleWireSize` wrappers in root
  `message.go` (zero callers; `srvtypes.CoordWireSize`/`AngleWireSize` are
  the live versions).
- The core network encoding (`server_net_send.go`, `sv_pvs.go`, `sv_stats.go`,
  `sv_client.go`) is deeply coupled to `*Server`/`*Client` (signon, datagram
  buffers, PVS, entity delta encoding) and is **not worth extracting** — moving
  it would require a huge callback interface and high parity risk. Kept in root.
- Build + all server tests green.

### Phase G — `internal/renderer` (DONE 2026-08-05, dead-code cleanup)
**The renderer is already healthy** — unlike server, every renderer subpackage
is substantively implemented AND wired into the root facade. No delegation
rewrites needed.

**Cleaned 8 genuinely-dead symbols** (no references, no `nolint:unused` intent
markers):
- `decal_shared.go`: `buildDecalBasis`, `decalNormalize3`.
- `renderer_gogpu.go`: fields `decalScratchBuffer`, `externalBrushClusterTexture`,
  `externalBrushClusterView`.
- `renderer_gogpu_world_alias.go`: `aliasSceneUniformBytes`,
  `aliasUniformBufferSize`.
- `renderer_gogpu_world_lightmap.go`: `updateUploadedLightmapsLocked`.
- `renderer_gogpu_world_pipelines.go`: `createWorldSkyPipelineWithDepthWrite`,
  `createWorldSkyPipelineWithDepthState`.
- `renderer_gogpu_world_resources.go`: `createWorldTextureArrayFromRGBA`.
- `renderer_gogpu_world_translucent.go`: `gogpuWorldTranslucentLiquidFaceRenders`.

**Intentionally KEPT** (carry `//nolint:unused` "pending wiring"/"retained
intentionally" markers): `identityModelRotationMatrix`,
`buildBrushRotationMatrix`, `transformModelSpacePoint`,
`parseWorldspawnLiquidAlphaOverrides`, `mapVisTransparentWaterSafe`,
`randomMarkRotation`.

Build + all renderer tests green. The remaining 72 root renderer files are
legitimate facade/GPU-state code coupled to `*Renderer` (like server's
`server_net_*`), not extractable without circular imports.

### Phase H — `internal/game` (DONE 2026-08-05, dead-stub cleanup)
- **Deleted** the dead `game/commands` stub (`Dispatcher` was a constructor-only
  wrapper imported by nobody, with no real logic — it would be a parity trap if
  ever wired). Its real counterpart is the root `game_commands*.go` `*Game`
  methods, which are too coupled to `g.Input`/`g.Client`/`g.Host` to extract.
- **Cleaned `camera.System`**: removed the dead no-op `ComputeView` method
  (never called; would silently break camera if wired). Kept the live, tested
  `UpdateZoom` (used by `Game.UpdateZoom` delegation).
- **Deleted** dead root `Game.hasAnyGameplayBindings` (no callers).
- Kept `game/ui` (real utility: GUIDimensions/ConsoleDimensions) and the live
  `camera.System` zoom path.
- Build + all game subpackage tests green; golangci clean on game.

### Summary
All eight phases (A–H) complete. The original plan docs assumed files could be
moved into subpackages; the corrected strategy was to reconcile actual
duplication — deleting dead/divergent subpackage stubs and dead root helpers
rather than forcing delegations that would introduce circular imports or
parity regressions. Net result: `server/physics`, `server/qc`, `server/commands`
now hold real, tested, wired logic; renderer and game dead code removed; every
root keeps only facade/state code that legitimately couples to its megastruct.

### Interface-Seam Enabling Work (`Client` migration, DONE 2026-08-05)
The blocker to interface-injecting the private server methods was that
`*server.Client` (and the private methods taking it) lived in `package server`,
so a `types`-defined interface could not reference them. This change moves the
enabling seams into `internal/server/types`:

- **Moved `Client` → `internal/server/types/client.go`**. All its field types
  (`Edict`, `UserCmd`, `MessageBuffer`, `EntityState`, `SignonStage`) were
  already in `types`; added the `internal/net` import for `Socket` (verified
  no import cycle: `net` does not import `server/types`). Root keeps
  `type Client = srvtypes.Client` alias so all existing call sites compile
  unchanged.
- **Exported the private client-coupled methods**: `playerClient` →
  `PlayerClient`, `runClientQCThinkWithMode` → `RunClientQCThinkWithMode`,
  `syncSpawnedEdictsFromQCVM` → `SyncSpawnedEdictsFromQCVM` (callers updated
  in the physics/qc/world files).
- **Added `types.ClientThinker` interface** (`PlayerClient`,
  `RunClientQCThinkWithMode`, `SyncSpawnedEdictsFromQCVM`), re-exported as
  `server.ClientThinker`, with a compile-time assert
  `var _ ClientThinker = (*Server)(nil)` in `interfaces_test.go`.

This is the model for every private method that blocks extraction: move its
argument types into `types`, export the method, define a narrow interface in
`types`, assert `*Server` satisfies it. The `Physics()` frame loop can now be
extracted into `physics.System.StepFrame` (its remaining deps — `CVarReader`,
`TelemetrySink` — follow the same pattern). Build + all server tests green;
only the two pre-existing failures remain (demo parity missing progs.dat,
quakego line ceiling).

### Frame-Loop Extraction (`Physics()` → `physics.System.StepFrame`, DONE 2026-08-05)
The biggest single-file payoff: `server_physics_loop.go` shrank from 261 lines
to a 30-line delegator. The frame loop (QC StartFrame, per-edict movetype
dispatch, client pre/post think, SendInterval bookkeeping, force_retouch decay,
dev-stats, time advance) now lives in `physics.System.StepFrame` and is
unit-testable with mocks.

Seams added in `types/stepframe.go` (all satisfied by `*Server`, compile-time
asserted):
- `CVarReader` + `CvarHandle` — the cvar lookups the loop performs.
- `TelemetrySink` — `EventsEnabled`/`BeginFrame`/`EndFrame`/`LogEventf`.
- `FrameDriver` — bundles CVarReader + TelemetrySink + ClientThinker +
  time/static/dev-stats/QC-exec surfaces.

New `*Server` methods in `interfaces.go`: `BoolValue`, `Get`, `EventsEnabled`,
`BeginFrame`, `EndFrame`, `LogEventf` (thin forwards to `s.CVar`/`s.DebugTelemetry`).
Also exported `recordDevStatsEdicts` → `RecordDevStatsEdicts` and QC-sync methods
`syncQCVMGlobals` → `SyncQCVMGlobals`, `setQCTimeGlobal` → `SetQCTimeGlobal`
(removed a duplicate `SetQCTimeGlobal` wrapper in savegame_server.go).

The movetype leaf dispatchers (`PhysicsPusher`/`PhysicsNone`/...) stay on
`*Server` and are injected via `physics.MovetypeDispatch` — the frame loop
doesn't need to own the leaf algorithms to be testable.

Tests: `physics/stepframe_test.go` exercises the loop in isolation with mocks,
verifying movetype dispatch counts and freeze-non-clients time behavior, using
a real `qc.VM` as the `ServerHandle` (the accessor trap: `EdictData` needs
`vm.NumEdicts`/`EdictSize` set, and `Get` must return untyped nil for
unregistered cvars to avoid the typed-nil interface bug).

Verified: full module builds; all server + physics tests pass; only the two
pre-existing failures remain.