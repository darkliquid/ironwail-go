# QCVM Entity Sync — Architecture, Bugs, and Migration

> **Status:** The unified sync approach is **implemented** (commit `fe9e43c`).
> The selective pusher/non-pusher sync layer was replaced with
> `syncAllToQCVM`/`syncAllFromQCVM` that syncs ALL entities unconditionally.
> Typed accessor methods were added. The full unified storage proposal (removing
> `EntVars` entirely) is **not yet complete** — `EntVars` still exists as the Go
> struct, but the fragile selective sync is gone. This document consolidates
> three prior docs: `C_VS_GO_QCVM_PARITY.md`, `TRIGGER_LIFT_INVESTIGATION.md`,
> and `PHASE3_UNIFIED_STORAGE_PROPOSAL.md`.

## The Architectural Problem

### C Ironwail: Shared Memory (No Sync)

C's `qcvm->edicts` is a malloc'd array of `edict_t`. Each `edict_t` contains
engine fields AND an `entvars_t v` struct. QC bytecode's `OP_LOAD_*` /
`OP_STORE_*` read/write `&ed->v + field_offset` — the **exact same memory**
C engine code accesses via `ed->v.field`.

- `EDICT_TO_PROG(e)` = byte offset from `qcvm->edicts` to `e`
- `PROG_TO_EDICT(n)` = pointer arithmetic back to edict
- **No sync needed.** When QC sets `self.nextthink`, the engine sees it
  immediately. When the engine sets `ent->v.velocity`, QC sees it immediately.
- All entity fields (standard + extension) accessible by both C and QC through
  the same memory.

### Go ironwail-go: Dual Storage with Sync

```
s.Edicts []*Edict          (Go structs)
  └── Edict.Vars *EntVars  (78 "bound" typed fields — Go's source of truth)

s.QCVM.Edicts []byte       (flat byte array, ~105+ fields depending on mod)
  └── [entNum*EdictSize + 28 + fieldOfs*4]
```

- Go physics, networking, and area grid use `EntVars` (typed struct).
- QC bytecode reads/writes the `QCVM.Edicts` byte array.
- `syncEdictToQCVM` copies bound fields Go → QCVM before QC callbacks.
- `syncEdictFromQCVM` copies bound fields QCVM → Go after QC callbacks.
- **78 of ~105+ fields are synced.** Extension fields (`state`, `speed`,
  `wait`, `pos1`, `pos2`, `finaldest`, `think1`, `count`, `delay`,
  `killtarget`, `trigger_field`, `th_checkattack`, `customflags`,
  `target2/3/4`) exist **only in QCVM bytes** — Go never reads or writes them
  through the sync layer (some are read via direct-VM accessors; see below).
- **Pusher entities** (`MOVETYPE_PUSH`) used to require special handling because
  their state (velocity, nextthink, think) is set by QC but must be seen by
  Go's `PhysicsPusher`. The selective pusher sync is now gone — all entities
  sync unconditionally.

This sync layer is the root cause of every trigger/entity bug found.

## QC Callback Dispatch Points

There are five points where Go calls into QC. The old approach used selective
pusher/non-pusher snapshot/diff/restore at each point. The current approach
(commit `fe9e43c`) uses `executeQCFunction` as the **single sync point** — it
calls `syncAllToQCVM` before and `syncAllFromQCVM` after every QC callback.
Callers just set `self`/`other`/`time` globals and call it.

| Dispatch point | File:line | Notes |
| --- | --- | --- |
| `touchLinks` (trigger area touch) | `internal/server/world.go:649` | Sets globals, calls `executeQCFunction` |
| `Impact` (direct collision touch) | `internal/server/physics/leafs.go:373` | Sets globals, calls `executeQCFunction` |
| `PhysicsPusher` think | `internal/server/physics/leafs.go:666` | Sets globals, calls `executeQCFunction` |
| `executeQCFunction` (generic wrapper) | `internal/server/qc_trace.go:71` | Single sync point — `syncAllToQCVM`/`syncAllFromQCVM` |
| `executeQCFunctionLeavingGlobals` | `internal/server/qc_trace.go` | Same sync, doesn't save/restore globals (StartFrame) |

## Bugs Found and Fixed

### Bug 1: `executeQCFunction` missing pusher sync — FIXED

**The bug:** `executeQCFunction` (`qc_trace.go:69`), the generic QC callback
wrapper used by `RunThink` and other dispatch points, captured non-pusher
snapshots and synced them back after the callback, but did **not** capture or
sync pusher (`MOVETYPE_PUSH`) entities.

**The chain (qbj2 lift):**
1. Player touches `trigger_multiple` → `multi_touch` → `multi_trigger` →
   `SUB_UseTargets`
2. Finds `func_button` → `button_use` → `button_fire` → `SUB_CalcMove` (button
   starts moving)
3. Button arrives → `button_wait` → `SUB_UseTargets` (with `delay=.5`)
4. Spawns `DelayedUse` entity (`MOVETYPE_NONE`) with `think=DelayThink`
5. 0.5s later, `DelayedUse` think fires via `RunThink` → `executeQCFunction`
6. `DelayThink` → `SUB_UseTargets` → finds `func_train` → `train_use` →
   `train_next` → `SUB_CalcMove` sets train velocity/nextthink in QCVM
7. **BUG:** `executeQCFunction` syncs non-pushers back but NOT pushers
8. Train's Go-side velocity/nextthink remain 0 → `PhysicsPusher` never moves it

**Fix:** Added `capturePusherSnapshots` / `syncPushersToQCVM` before QC execution
and `syncMutatedPushersFromQCVM` after, to both `executeQCFunction` and
`executeQCFunctionLeavingGlobals`. Test:
`TestExecuteQCFunctionSyncsPusherMutationsFromNonPusherThink`.

### Bug 2: `Impact` missing pusher sync — FIXED

**The bug:** `Impact` (`physics/leafs.go:373`) synced only the two colliding entities,
not pushers. If a touch callback (e.g., `button_touch`) called `SUB_UseTargets`
targeting a pusher, the pusher's mutations were lost.

**Fix:** Added pusher snapshot/sync to both touch dispatch blocks in `Impact`.
Test: `TestImpactSyncsPusherMutationsFromQCVM`.

### Bug 3: Non-WALK client missing `LinkEdict` — FIXED

**The bug:** C's `SV_Physics_Client` always calls `SV_LinkEdict(ent, true)` after
the movetype switch. Go's `PhysicsWalk` does this internally, but the non-WALK
path (`MOVETYPE_NONE` during intermission) in `physics/stepframe.go` did not.
Trigger touches wouldn't fire for stationary non-walking clients.

**Fix:** Added `s.LinkEdict(ent, true)` for non-WALK client entities before
`PlayerPostThink`.

## Remaining Parity Gaps (Unfixed)

| Gap | Description | Status |
| --- | --- | --- |
| 9 | `RunThink` does not clamp `thinkTime` to `s.Time` | Minor — C clamps, Go doesn't |
| 13 | `force_retouch` read once before loop, not per-entity | Minor — C reads inside loop |

Gaps 1-8, 10-12 are resolved — the selective sync layer that caused them was
replaced by `syncAllToQCVM`/`syncAllFromQCVM` in commit `fe9e43c`. The old
pusher/non-pusher classification, `capturePusherSnapshots`,
`syncPushersToQCVM`, `syncMutatedPushersFromQCVM`, and related functions are
gone (~170 lines of dead code removed). The double-sync redundancy in
`touchLinks` (old Gap 7) is eliminated — callers just set globals and call
`executeQCFunction`.

## Debugging Tools

- **`sv_debug_trigger` cvar:** Console output for trigger dispatch. Shows
  `trigger [touchlinks]` / `trigger [impact]` with ent#, classname, targetname,
  target, touch/use/think fn, `th_checkattack`, `customflags`, `state`, `wait`,
  `nextthink`. Also logs `find(targetname=...)` and `pusher synced` lines.
  Files: `internal/server/debug_trigger.go`, with calls in `world.go`,
  `physics/leafs.go`, `server.go`, `debug_telemetry.go`.
- **`sv_debug_telemetry` cvar:** Broader server-side telemetry for
  trigger/physics/QC activity. See `internal/server/debug_telemetry.go`.

## C Reference Functions

| C function | File | Go counterpart |
| --- | --- | --- |
| `SV_TouchLinks` | `world.c:336-380` | `touchLinks` (`world.go:649`) |
| `SV_AreaTriggerEdicts` | `world.c:286-324` | `areaTriggerEdicts` |
| `SV_LinkEdict` | `world.c:467-538` | `LinkEdict` |
| `SV_Physics` | `sv_phys.c:1226-1298` | `Physics` (`physics/stepframe.go:22`) |
| `SV_Physics_Pusher` | `sv_phys.c:618-652` | `PhysicsPusher` (`physics/leafs.go:666`) |
| `SV_PushMove` | `sv_phys.c:434-607` | `PushMove` (`physics/leafs.go:433`) |
| `SV_Impact` | `sv_phys.c:155-179` | `Impact` (`physics/leafs.go:373`) |
| `SV_RunThink` | `sv_phys.c` | `RunThink` (`physics/leafs.go:329`) |

## Unified Storage: Implementation Status

### What's Done (commit `fe9e43c`)

- **Single sync point:** `executeQCFunction` now calls `syncAllToQCVM` before
  and `syncAllFromQCVM` after every QC callback. Callers just set
  `self`/`other`/`time` globals and call it. The fragile pusher/non-pusher
  classification and selective snapshot/diff/restore are gone.
- **Typed accessor methods:** 157 accessor methods added to `Edict` in
  `internal/server/entity_accessors.go` that read/write directly to the QCVM
  byte array via `s.QCVM.EVector`/`EFloat`/`EInt`/`SetE*`. These bypass
  `EntVars` entirely, matching C's shared-memory model.
- **O(1) `NumForEdict`:** Cached `Num` field on `Edict` for fast entity-number
  lookup.
- **Cached extension field offsets:** Fields like `state`, `wait`, `speed`,
  `customflags`, `th_checkattack` are cached at `progs.dat` load time.
- **Removed ~170 lines of dead code:** 6 sync functions and 2 snapshot types
  deleted.
- **Relink optimization:** `syncAllFromQCVM` only relinks on `Solid`/`Model`
  changes, not `Origin` (matches C `SV_TouchLinks` behavior).
- **Partial direct-VM access in `server.go`:** ~27 call sites in `server.go`
  (area-grid, `SV_FindTouchedLeafs`, `LinkEdict` internals) now read/write
  the QCVM byte array directly via `vm.EFloat`/`vm.EVector`/`vm.SetE*`,
  bypassing `EntVars`. The `entity_accessors.go` methods are available but
  not yet widely adopted in the physics/movement hot paths.

### What Remains

- **`EntVars` struct still exists** (`internal/server/types_entities.go:165`).
  The accessor methods are available but not all hot-path code has migrated
  to them. `EntVars` is still the primary Go-side struct for physics and
  networking.
- **`server_qc_sync.go` still exists** (406 lines) with `syncEdictToQCVM` /
  `syncEdictFromQCVM` used by the sync-all functions. The per-edict sync
  functions are still field-selective (only bound fields), but since all
  entities are synced unconditionally, the "forgot to sync this dispatch path"
  class of bug is eliminated.
- **Steps 3-5 of the original migration plan not done:** Remove `EntVars`,
  simplify `savegame_server.go` for QCVM bytes, and simplify callback dispatch to
  match C exactly (no sync at all — just set globals and execute).

### Original Proposal (for reference)

The original 5-step plan was:

| Step | What | Status |
| --- | --- | --- |
| 1 | Add accessor methods to `Edict`, cache extension field offsets | **Done** (157 accessors) |
| 2 | Migrate hot-path code to accessors | **Partial** — accessors exist, `server.go` has ~27 direct-VM sites, but physics/movement still use `EntVars` |
| 3 | Remove sync functions (delete `server_qc_sync.go`, simplify `qc_trace.go`) | **Partially done** — old selective sync removed, sync-all replaces it |
| 4 | Remove `EntVars` struct (rewrite `savegame_server.go` for QCVM bytes) | **Not done** |
| 5 | Simplify callback dispatch (match C exactly — no sync, just set globals and execute) | **Not done** — sync-all still runs at every callback |

### Rejected Alternatives (for reference)

1. **Extend `EntVars` with all fields:** Would make sync complete but doesn't
   eliminate it. Would require rebuilding `EntVars` per mod.
2. **Mirror struct via `unsafe.Pointer`:** Zero-copy, fastest, but uses
   `unsafe` (rejected).

### What Gets Removed (When Steps 3-5 Complete)

- `EntVars` struct entirely
- `syncAllToQCVM` / `syncAllFromQCVM` (when all code uses accessors)
- `server_qc_sync.go` (the per-edict sync functions)
- `executeQCFunction`'s sync calls (simplified to just save/restore
  `self`/`other`/`time` globals, matching C)

## Key Files

| File | Role |
| --- | --- |
| `internal/server/edict.go` | `Edict` struct, `ED_ParseEdict` |
| `internal/server/server_qc_sync.go` | All sync functions (to be removed) |
| `internal/server/qc_trace.go:69` | `executeQCFunction` |
| `internal/server/physics/leafs.go:373` | `Impact` |
| `internal/server/physics/leafs.go:666` | `PhysicsPusher` |
| `internal/server/physics/stepframe.go:22` | `Physics` main loop |
| `internal/server/world.go:649` | `touchLinks` |
| `internal/server/debug_trigger.go` | `sv_debug_trigger` logging |
| `internal/server/debug_telemetry.go` | `sv_debug_telemetry` logging |
