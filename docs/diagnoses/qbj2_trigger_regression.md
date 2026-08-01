# Diagnosis: qbj2 Button/Trigger Regression

## Summary

Buttons in the spawn area of qbj2's start map no longer trigger door
opening. Several other triggers also no longer work. The regression was
introduced in commit `570e806` ("feat(server): implement Direct-VM
Accessors & Zero-Sync QCVM (Phase 1)") and was partially patched in
commit `29a368b` ("fix(server): populate QCVM edict storage for all map
entity key-value pairs"), but the core problem remains.

## Root Cause

### The dual-storage desync

The engine has TWO entity storage systems that must stay in sync:

1. `Edict.Vars *EntVars` — Go struct with 78 typed fields
2. `s.QCVM.Edicts []byte` — flat byte array where QC bytecode operates

QC bytecode (`OP_STORE_*`, `OP_LOAD_*`) reads/writes **only** the QCVM
byte array. Go code can read/write either system:
- Accessor methods (`ent.Touch(s)`, `ent.SetNextThink(s, v)`) dual-write
  to **both** QCVM bytes and `EntVars`
- Direct struct access (`ent.Vars.Touch`) reads/writes **only** `EntVars`

### What changed in `570e806`

**Before** `570e806`, `executeQCFunction` (`qc_trace.go:69`) called:
1. `syncAllToQCVM()` — copy ALL entities' `EntVars` → QCVM bytes
2. `vm.ExecuteFunction(funcIdx)` — QC runs
3. `syncAllFromQCVM()` — copy ALL entities' QCVM bytes → `EntVars`
4. `syncSpawnedEdictsFromQCVM(prevNumEdicts)` — sync newly-spawned entities

This "nuclear" sync ensured `EntVars` and `QCVM.Edicts` stayed consistent
at every QC callback boundary. It was correct but slow (O(numEdicts *
numFields) per callback — the bottleneck that `qbj2_zetabyt` profiling
identified as 52% of CPU time).

**After** `570e806`, `executeQCFunction` calls:
1. (nothing — no pre-sync)
2. `vm.ExecuteFunction(funcIdx)` — QC runs
3. `syncSpawnedEdictsFromQCVM(prevNumEdicts)` — syncs **only** newly-spawned
   entities (edict numbers >= `prevNumEdicts`)

The `syncAllToQCVM()` and `syncAllFromQCVM()` calls were removed.
`EntVars` is no longer kept in sync with QCVM after QC callbacks.

### The killing blow: `syncQCVMState()` in `Physics()`

`physics_loop.go:62` calls `s.syncQCVMState()` at the **start of every
frame**. This function (`server_qc_sync.go:371`) does two things:

1. Sets QC globals (`time`, `world`, `mapname`, `serverflags`, `coop`,
   `deathmatch`) — **correct and necessary**
2. Calls `syncEdictToQCVM` for **ALL entities** — copies `EntVars` field
   values → QCVM bytes via reflection (`syncEntVarsToQC`)

Step 2 is the regression trigger. It **unconditionally overwrites** QCVM
entity fields with `EntVars` values. When `EntVars` is stale (because
`syncAllFromQCVM` was removed), this destroys QC mutations.

### Concrete failure scenario (button → counter → door)

1. Player walks into button area during `PhysicsWalk` → `SV_WalkMove`
   → `LinkEdict(player, true)` → `touchLinks(player)`
2. `touchLinks` finds overlapping `trigger_multiple`, fires its touch
   function: `multi_touch` → `SUB_UseTargets`
3. `SUB_UseTargets` calls `find(targetname, self.target)` to locate the
   `trigger_counter`, calls its `use` function
4. Counter decrements `count` in QCVM bytes. If count reaches 0, calls
   `SUB_UseTargets` again → finds `func_door` → calls `door_use`
5. `door_use` sets `self.nextthink = time + 0.1` and
   `self.think = door_go_down` via QC `OP_STORE_*` — writes **only** to
   QCVM bytes
6. `syncSpawnedEdictsFromQCVM` is called — only syncs NEW entities, not
   the door. Door's `EntVars.NextThink` remains **0** (stale)
7. `PhysicsPusher` reads `ent.NextThink(s)` (accessor) — sees the QCVM
   value from step 5. If `time + 0.1` hasn't been reached yet, the think
   doesn't fire this frame.
8. **Next frame**: `Physics()` starts → `syncQCVMState()` calls
   `syncEdictToQCVM` for ALL entities, including the door.
   `syncEntVarsToQC` copies `door.EntVars.NextThink = 0` → QCVM,
   **overwriting** the `nextthink = time + 0.1` that QC set in step 5.
9. The door's think **never fires**. The door never opens.

This same pattern affects any entity whose fields are mutated by QC
bytecode (not builtins) and then read in a later frame after
`syncQCVMState()` clobbers them. This includes:
- Doors (nextthink, think, velocity set by SUB_CalcMove)
- Trains (nextthink, think set by train_next)
- Counters (count field decremented by counter_use)
- Any entity targeted by SUB_UseTargets with a delay

### Secondary issue: `touchLinks` reads `ent.Vars.*` directly

`world.go:614-785` — `areaTriggerEdicts` and `touchLinks` read directly
from `ent.Vars.Touch`, `ent.Vars.Solid`, `ent.Vars.AbsMin`,
`ent.Vars.AbsMax` instead of using accessor methods. If these `EntVars`
fields are stale (e.g., because QC set them via `OP_STORE_*` after the
last `syncEdictFromQCVM`), triggers may not fire or may use wrong AABBs.

This is secondary because the critical fields (`Touch`, `Solid`) are
typically set during spawn and synced via `syncEdictFromQCVM` in
`loadMapEntities`. But `AbsMin`/`AbsMax` change at runtime and may be
stale if not dual-written by accessor methods.

### Why commit `29a368b` was a partial fix

Commit `29a368b` fixed a different but related issue: during map entity
parsing, `parseEdictFieldValue` only called `parseQCVMEdictFieldValue`
for keys NOT in the `EntVars` struct. Standard fields like `target`,
`targetname`, `classname` were written to `EntVars` but skipped for
QCVM. This meant `find()` (which searches QCVM bytes) couldn't locate
entities by targetname. The fix made `parseQCVMEdictFieldValue` run
unconditionally for all keys.

This fixed the `find()` failure but did NOT fix the
`syncQCVMState()` overwriting problem. Entities can now be found, but
their QC-mutated fields are still clobbered every frame.

## Fix Plan

### Phase 1: Stop `syncQCVMState()` from clobbering QC mutations (DONE)

**Files changed**:
- `internal/server/server_qc_sync.go` — Extracted `syncQCVMGlobals()` from
  `syncQCVMState()`. The new function sets only QC globals (time, world, mapname,
  serverflags, coop, deathmatch) without iterating entities.
- `internal/server/physics_loop.go` — Replaced per-frame `syncQCVMState()` with
  `syncQCVMGlobals()` + targeted client-entity sync (entities 1..maxClients).
  Client movement code (`SV_ClientThink` → `airMove` etc.) writes to
  `ent.Vars.*` directly, so those entities must be pushed to QCVM before
  `PhysicsWalk` reads via accessors.
- `internal/server/user.go` — Replaced `syncQCVMState()` in
  `runClientQCThinkWithMode` with `syncQCVMGlobals()`.
- `internal/server/rules.go` — Replaced `syncQCVMState()` in `NextLevel` dispatch
  with `syncQCVMGlobals()`.
- `internal/server/user_spawn.go` — Replaced `syncQCVMState()` in
  `runClientParseClientCommandQC` with `syncQCVMGlobals()`.
- `internal/server/server_qc_sync.go` — Fixed `ensureQCVMEdictStorage` to
  preserve existing QCVM data when growing the Edicts byte slice (was `make`
  which zeroed all data; now uses `copy`).

`syncQCVMState()` (with per-entity sync) is kept for map load/init paths where
`EntVars` is genuinely the source of truth: `SpawnServer`, `savegame` restore,
`runClientSpawnQC` (client spawn).

### Phase 2: Migrate `touchLinks`, `areaTriggerEdicts`, `LinkEdict` to accessor methods (DONE)

**File**: `internal/server/world.go`

- `areaTriggerEdicts` — reads `Touch(s)`, `Solid(s)`, `AbsMin(s)`, `AbsMax(s)`
  via accessors instead of `ent.Vars.*`.
- `touchLinks` — all entity field reads migrated to accessor methods.
- `LinkEdict` — migrated from `ent.Vars.*` to accessor methods (`Origin(s)`,
  `Mins(s)`, `Maxs(s)`, `Flags(s)`, `Solid(s)`, `ModelIndex(s)`). Writes
  AbsMin/AbsMax via `SetAbsMin`/`SetAbsMax` (dual-write) so both EntVars and
  QCVM see the updated AABB.

### Phase 3: Migrate remaining `ent.Vars.*` reads (TODO)

Audit and migrate all remaining production-code `ent.Vars.*` reads:

| File | Lines | Fields | Priority |
| --- | --- | --- | --- |
| `world.go` | clipToLinks | `Vars.Size`, `Vars.Owner` | High (collision) |
| `server_qc_sync.go` | newCheckClient | `Vars.Health`, `Vars.Flags`, `Vars.Origin`, `Vars.ViewOfs` | Medium |
| `user.go` | SV_ClientThink, airMove, etc. | All direct `Vars` reads/writes | High (client movement) |
| `host/server_browser.go` | 50 | `Vars.Frags` | Low |
| `user_spawn.go` | initClientSpawnFallback | Direct `Vars` writes | Medium |

### Phase 4: Complete EntVars removal (architectural, TODO)

Once all `ent.Vars.*` reads/writes are migrated to accessor methods:
1. Delete `EntVars` struct from `types_entities.go`
2. Delete `server_qc_sync.go` (sync layer)
3. Simplify `executeQCFunction` — no sync calls at all, matching C's
   zero-sync architecture
4. Update `savegame.go` to serialize QCVM bytes directly

## Verification

### Unit test

The existing `TestQBJ2ButtonRepro` (`qbj2_button_repro_test.go`) provides
the repro case. It requires `pak0.pak` (skips without it). Run with:

```
TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/server/... \
  -run TestQBJ2ButtonRepro -count=1 -v
```

After the fix, the counter's `count` field should decrement when
buttons are touched, and the door's `nextthink`/`think` should be set.

### Integration test

Run qbj2 start map and verify:
1. Buttons in spawn area open doors when pressed
2. Triggers throughout the map fire correctly
3. No regression in standard maps (start, e1m1)

### Regression check

Run the full test suite:
```
TMPDIR=.../.tmp CGO_ENABLED=0 go test ./... -count=1
```

## C Reference

- C Ironwail has NO sync layer — engine and QC share the same memory
  (`edict_t.v` struct). `SV_Physics` in `sv_phys.c` does NOT call any
  sync function. It just sets `pr_global_struct->time = sv.time` and
  calls `StartFrame`.
- The Go equivalent should be: set globals, run StartFrame, run physics.
  No per-entity sync needed if accessor methods are used everywhere.
