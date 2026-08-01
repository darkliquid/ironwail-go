# Phase 3: Complete Zero-Sync QCVM — EntVars Removal

**Priority**: Critical (fixes trigger/door/button regressions)
**Status**: Planning
**Depends on**: Phase 1 & 2 (done — `syncQCVMGlobals` extracted, `touchLinks`/`LinkEdict` migrated)

---

## 1. Problem Statement

The engine has **two entity storage systems** that must stay in sync:

1. `Edict.Vars *EntVars` — Go struct with 78 typed fields
2. `s.QCVM.Edicts []byte` — flat byte array where QC bytecode operates

QC bytecode reads/writes **only** the QCVM byte array via `OP_LOAD_*`/`OP_STORE_*`.
Go code currently reads/writes **both** systems through three patterns:

| Pattern | Where | Behavior |
|---------|-------|----------|
| Accessor methods (`ent.Origin(s)`) | entity_accessors*.go | Dual-write to QCVM + EntVars. **Read**: QCVM first, falls back to EntVars if QCVM value is zero. |
| Direct `ent.Vars.*` reads | 306 call sites across 19 files | EntVars-only — QCVM never updated |
| Direct `ent.Vars.*` writes | Same 306 sites | EntVars-only — QCVM never updated |

### The zero-value fallback bug

Every accessor **reader** has this pattern:

```go
func (e *Edict) Origin(s *Server) [3]float32 {
    if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
        v := s.QCVM.EVector(e.Num, qc.EntFieldOrigin)
        if v != [3]float32{} || e.Vars == nil {  // ← BUG: zero is a valid value
            return v
        }
    }
    if e.Vars != nil {
        return e.Vars.Origin  // ← stale fallback
    }
    return [3]float32{}
}
```

If the QCVM has a legitimate zero value (origin `{0,0,0}`, velocity `{0,0,0}`,
health `0`, nextthink `0`), the accessor falls back to `EntVars` which may be
stale. This causes:
- Dead players reporting health=100 (stale EntVars)
- Entities at origin {0,0,0} using stale EntVars origin
- Nextthink=0 (correct) but accessor returns stale EntVars value

### The 306 direct Vars accesses

306 production code sites read/write `ent.Vars.*` directly, bypassing the QCVM
entirely. These are the remaining sync layer dependents. The per-frame
`syncQCVMGlobals()` + client-entity sync mitigates this, but it's fragile —
any QC mutation to a non-client entity between sync points is invisible to
direct `Vars` reads.

---

## 2. Implementation Waves

The migration proceeds in 9 waves, ordered by runtime impact and dependency.

### Wave 1: Simplify accessor read path (remove zero-fallback)

**Files**: `entity_accessors.go`, `entity_accessors_vec.go`
**Changes**: 62 accessor methods (40 float32 + 22 vec3)

Remove the `if v != 0 || e.Vars == nil` zero-value check from all **reader**
methods. After this wave, readers always return the QCVM value when QCVM is
available (matching C's shared-memory model). Writers already dual-write.

**Before**:
```go
func (e *Edict) Origin(s *Server) [3]float32 {
    if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
        v := s.QCVM.EVector(e.Num, qc.EntFieldOrigin)
        if v != [3]float32{} || e.Vars == nil {
            return v
        }
    }
    if e.Vars != nil {
        return e.Vars.Origin
    }
    return [3]float32{}
}
```

**After**:
```go
func (e *Edict) Origin(s *Server) [3]float32 {
    if s != nil && s.QCVM != nil && s.QCVM.EdictSize > 28 {
        return s.QCVM.EVector(e.Num, qc.EntFieldOrigin)
    }
    if e.Vars != nil {
        return e.Vars.Origin
    }
    return [3]float32{}
}
```

The `EntVars` fallback remains only for the case where `s.QCVM == nil`
(tests that create edicts without a VM). The `Solid` accessor already has
this form (no zero-check) — it's the model to follow.

**Risk**: Entities at legitimate zero positions/velocities will now return
zero instead of falling back to EntVars. This is correct behavior, but tests
that set `ent.Vars.Origin = {0,0,0}` and expect the accessor to return
something else will break.

**Testing**: Run full test suite after this wave. Fix any test that relies
on the zero-fallback by using `SetOrigin(s, v)` instead of `ent.Vars.Origin = v`.

---

### Wave 2: Migrate `user.go` (SV_ClientThink + movement functions)

**File**: `internal/server/user.go`
**Scope**: 69 direct `Vars` references

This is the **highest-impact** wave — it contains the client movement code
(`SV_ClientThink`, `airMove`, `waterMove`, `noclipMove`, `waterJump`,
`dropPunchAngle`, `userFriction`, `accelerate`, `airAccelerate`) and
`ReadClientMove`.

**Key changes**:

1. `SV_ClientThink` (lines 356-401): Replace `ent.Vars.MoveType`, `.Origin`,
   `.Velocity`, `.Angles`, `.VAngle`, `.PunchAngle`, `.Flags`, `.FixAngle`,
   `.Health`, `.WaterLevel` reads with accessor calls. Replace the
   `clientMoveContext` struct construction to read via accessors.

2. `airMove`/`waterMove`/`noclipMove` (lines 259-318): Replace
   `ctx.player.Vars.Velocity` reads/writes with `ctx.player.SetVelocity(s, v)`
   and `ctx.player.Velocity(s)`. Replace `ctx.player.Vars.Angles`,
   `.MoveType`, `.TeleportTime` with accessors.

3. `accelerate`/`airAccelerate`/`userFriction` (lines 148-195): Replace
   `ctx.player.Vars.Velocity[0] += ...` with vector-level accessor calls.
   Pattern: read velocity via accessor, modify, write back via `SetVelocity`.

4. `ReadClientMove` (lines 483-521): Replace `client.Edict.Vars.VAngle`,
   `.Button0`, `.Button2`, `.Impulse` writes with `SetVAngle`, `SetButton0`,
   etc. These are QCVM-visible fields that QC spawn functions read.

5. `dropPunchAngle` (lines 189-196): Replace `ent.Vars.PunchAngle` with
   `ent.PunchAngle(s)` / `ent.SetPunchAngle(s, v)`.

6. `waterJump` (lines 249-256): Replace `.TeleportTime`, `.Flags`,
   `.MoveDir`, `.Velocity` with accessors.

7. Client command functions (lines 556-832): Replace `.Team`, `.NetName`,
   `.Frags` writes with accessors.

**After this wave**: Remove the client-entity sync in `physics_loop.go`
(the `for i := 1; i <= maxClients; i++` loop after `syncQCVMGlobals`).
Client movement code now writes directly to QCVM via accessors, so the
EntVars→QCVM push is no longer needed.

---

### Wave 3: Migrate `world.go` collision functions

**File**: `internal/server/world.go`
**Scope**: 38 direct `Vars` references (excluding `touchLinks`/`areaTriggerEdicts`/`LinkEdict` — already migrated)

**Key changes**:

1. `hullForEntity` (lines 106-182): Replace `ent.Vars.Solid`, `.Origin`,
   `.Mins`, `.Maxs`, `.ModelIndex` with accessors.

2. `clipToLinks` / `clipMoveToEntity` (lines 793-877): Replace
   `ent.Vars.AbsMin`, `.AbsMax`, `.Solid`, `.Flags`, `.Size`, `.Owner`
   with accessors.

3. `TestEntityPosition` (line 966): Replace `ent.Vars.Origin`, `.Mins`,
   `.Maxs` with accessors.

---

### Wave 4: Migrate `server.go` QC builtins and networking

**File**: `internal/server/server.go`
**Scope**: 54 direct `Vars` references

**Key changes**:

1. QC builtin implementations (`setorigin`, `setmodel`, `setsize`,
   `setspawnparms`, etc.): Replace `e.Vars.Origin`, `.Mins`, `.Maxs`,
   `.Size`, `.AbsMin`, `.AbsMax`, `.Model`, `.ModelIndex`, `.Model` writes
   with accessor calls. Many of these already dual-write via accessors
   then also push to VM via `vm.SetEVector` — remove the redundant VM push
   since the accessor handles it.

2. `SV_Aim` (lines 530-595): Replace `ent.Vars.Origin`, `.ViewOfs`,
   `.Team`, `.TakeDamage` reads with accessors.

3. `SV_CheckBottom` / `SV_MoveToGoal` / `SV_CheckBottom` (lines 599-646):
   Replace `.Flags`, `.Origin`, `.GroundEntity`, `.GoalEntity` with accessors.

4. Entity state snapshot (lines 893-899): Replace `.Origin`, `.Angles`,
   `.ModelIndex`, `.Frame`, `.Colormap`, `.Skin`, `.Effects` with accessors.

---

### Wave 5: Migrate `movement.go` monster AI movement

**File**: `internal/server/movement.go`
**Scope**: 38 direct `Vars` references

**Key changes**:

1. `SV_FlyMove` / `SV_StepDirection` / `SV_MoveToGoalEntity` (lines 31-299):
   Replace `.Angles`, `.IdealYaw`, `.YawSpeed`, `.Origin`, `.Mins`, `.Maxs`,
   `.Flags`, `.Enemy`, `.GroundEntity`, `.GoalEntity`, `.AbsMin`, `.AbsMax`
   with accessors.

2. `SV_WalkMove` / `SV_FlyMove` monster variants: Replace `.Velocity`,
   `.Origin` writes with accessor setters.

---

### Wave 6: Migrate `user_spawn.go` client spawn

**File**: `internal/server/user_spawn.go`
**Scope**: 33 direct `Vars` references

**Key changes**:

1. `initClientSpawnFallback` (lines 134-180): Replace all `ent.Vars.* = ...`
   assignments with `ent.Set*(s, ...)` calls. This is the Go-side fallback
   spawn that runs when QC has no `PutClientInServer` function.

2. `findLocalSpawnPoint` (line 116): Replace `.ClassName` read with accessor.

3. `ReadClientMove` angle/button writes (lines 386-390): Replace
   `client.Edict.Vars.VAngle` etc. with accessor setters.

4. `runClientPutInServerQC` (line 286): Replace `.Health`, `.ClassName`
   reads with accessors.

---

### Wave 7: Migrate server subsystems

**Files**: `sv_client.go` (11), `sv_stats.go` (10), `svdbg.go` (6),
`debug_telemetry.go` (6), `server_runtime.go` (5), `rules.go` (3),
`qc_trace.go` (1)

**Key changes**:

1. `sv_stats.go`: Replace `.Health`, `.WeaponModel`, `.CurrentAmmo`,
   `.ArmorValue`, `.WeaponFrame`, `.AmmoShells`, `.AmmoNails`,
   `.AmmoRockets`, `.AmmoCells`, `.Weapon` with accessor reads.

2. `sv_client.go`: Replace `.Message`, `.Sounds`, `.Frags`, `.Effects`,
   `.ModelIndex`, `.Origin`, `.Angles`, `.Frame`, `.Skin`, `.Model` with
   accessors.

3. `svdbg.go`, `debug_telemetry.go`: Replace `.Solid`, `.ClassName`,
   `.Touch`, `.AbsMin`, `.AbsMax`, `.Origin`, `.TargetName`, `.Target`,
   `.Model` with accessors.

4. `server_runtime.go`: Replace `.NetName`, `.Team`, `.Health`, `.DeadFlag`
   with accessors.

5. `rules.go`: Replace `.Frags`, `.Health`, `.DeadFlag` with accessors.

6. `qc_trace.go`: Replace `.ClassName` read with accessor.

---

### Wave 8: Migrate `host/` package

**Files**: `commands_gameplay_debug.go` (28), `commands_gameplay.go` (18),
`autosave.go` (12), `server_browser.go` (1), `commands_map.go` (1),
`commands_gameplay_save.go` (1)

**Key changes**: These are console command handlers and the autosave system.
Replace all `.Vars.*` reads/writes with accessor calls. The `host` package
already imports `server` — it can call `ent.SetOrigin(srv.Server, v)` etc.

**Note**: `host` package code receives `*server.Server` via `subs.Server`.
Accessor methods need the `*Server` receiver, so the call pattern becomes
`ent.Origin(subs.Server)` instead of `ent.Vars.Origin`.

---

### Wave 9: Delete sync layer and EntVars

**Files to delete**:
- `internal/server/server_qc_sync.go` (entire file — 400+ lines)
- `internal/server/types_entities.go` lines 165-549 (EntVars struct)
- `internal/server/edict.go` — `entVarsFieldIndex`, `buildEntVarsFieldIndex`,
  `stringEntFieldNames`, and the `EntVars` fallback branches in
  `parseEdictFieldValue`

**Files to update**:
- `internal/server/types_entities.go` — Remove `Vars *EntVars` field from
  `Edict` struct
- `internal/server/edict.go` — Simplify `parseEdictFieldValue` to write
  only to QCVM (remove EntVars reflection path)
- `internal/server/savegame.go` — Serialize `s.QCVM.Edicts` byte slice
  directly instead of reflecting over `EntVars`
- `internal/server/entity_accessors*.go` — Remove all `e.Vars` fallback
  branches (EntVars no longer exists)
- `internal/server/physics_loop.go` — Remove client-entity sync loop
  (no longer needed after Wave 2)

**After this wave**: The engine matches C Ironwail's zero-sync architecture.
`executeQCFunction` just sets globals and calls `vm.ExecuteFunction` — no
sync before or after. `syncQCVMGlobals` remains for frame-start global sync.

---

## 3. Testing Strategy

After each wave:
```
TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/server/... -count=1
TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/host/... -count=1
TMPDIR=.../.tmp CGO_ENABLED=0 go test ./... -count=1
```

Key parity tests to watch:
- `TestPhysicsWalkJump`, `TestPhysicsPusher`, `TestPhysicsStep` — physics
- `TestFrameProcessesClientMoveBeforePhysics` — client movement
- `TestTouchLinksSyncsQCChangesBackToGoEdicts` — trigger dispatch
- `TestRunClientSpawnQCRelinksClientAfterQCSpawnMove` — client spawn
- `TestLoadMapEntitiesPopulatesQCVM` — map entity loading

---

## 4. Estimated Effort

| Wave | Files | Sites | Effort | Risk |
|------|-------|-------|---------|------|
| 1 | 2 | 62 methods | Low | Medium (zero-value tests) |
| 2 | 1 | 69 | High | High (client movement) |
| 3 | 1 | 38 | Medium | Medium (collision) |
| 4 | 1 | 54 | High | Medium (QC builtins) |
| 5 | 1 | 38 | Medium | Medium (monster AI) |
| 6 | 1 | 33 | Medium | Low (spawn fallback) |
| 7 | 7 | 42 | Low | Low (read-only mostly) |
| 8 | 6 | 61 | Medium | Low (console commands) |
| 9 | 5+ | — | Medium | High (deletion) |

Total: ~306 sites across ~20 files.

---

## 5. C Reference

C Ironwail has NO sync layer. Engine and QC share the same `edict_t` struct:

```c
typedef struct edict_s {
    // engine fields
    entity_state_t baseline;
    // ...
    entvars_t v;  // QC-visible fields, same memory
} edict_t;
```

`SV_Physics` in `sv_phys.c` just sets `pr_global_struct->time = sv.time` and
calls `StartFrame`. No per-entity sync. `SV_LinkEdict` writes directly to
`ent->v.absmin`/`ent->v.absmax`. `SV_TouchLinks` reads from `ent->v.*`.

The Go port's dual-storage architecture was a workaround for Go's GC and
no-pointer-arithmetic constraints. The accessor methods bridge this gap by
reading/writing the QCVM byte array directly. Phase 3 completes the migration
by making the QCVM the sole source of truth and removing the EntVars mirror.
