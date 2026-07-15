# C vs Go QCVM Parity Investigation

## Executive Summary

The C Ironwail QCVM and the Go ironwail-go QCVM have a fundamental
architectural difference: **C shares a single memory space for server edict
state and QCVM entity fields; Go maintains two separate stores (Go `EntVars`
structs and QCVM byte arrays) that must be bidirectionally synchronized.**

This sync layer is the source of every trigger/entity bug found so far. The
sync is field-selective (only "bound" fields are synced), relies on
snapshot/diff/restore around every QC callback, and is missing in several
dispatch paths. C has no sync at all — it just sets `self`/`other`/`time`
globals and calls `PR_ExecuteProgram`.

## Architectural Comparison

### C Ironwail: Shared Memory

```
qcvm->edicts (malloc'd array)
  └── edict_t
        ├── engine fields (free, area, baseline, alpha, ...)
        └── entvars_t v  ← QC bytecode reads/writes here directly
                             C engine code reads/writes ed->v.field directly
```

- `EDICT_TO_PROG(e)` = byte offset from `qcvm->edicts` to `e`
- `PROG_TO_EDICT(n)` = pointer arithmetic back to edict
- QC `OP_LOAD_*` reads `&ed->v + field_offset` — same memory C accesses
- QC `OP_STOREP_*` writes `&ed->v + field_offset` — same memory
- **No sync needed.** When QC sets `self.nextthink`, the engine sees it
  immediately. When the engine sets `ent->v.velocity`, QC sees it immediately.
- **All entity fields** (standard + extension) are accessible by both C and QC
  through the same memory. There is no "Go doesn't know about this field"
  problem.

### Go ironwail-go: Dual Storage with Sync

```
s.Edicts []*Edict          (Go structs with typed EntVars)
  └── Edict.Vars *EntVars  (only 66 "bound" fields)

s.QCVM.Edicts []byte       (flat byte array, all entity fields)
  └── [entNum*EdictSize + 28 + fieldOfs*4]  (includes extension fields)
```

- Go physics, networking, and area grid use `EntVars` (typed struct)
- QC bytecode reads/writes `QCVM.Edicts` byte array
- **syncEdictToQCVM**: copies bound EntVars fields → QCVM bytes (before QC)
- **syncEdictFromQCVM**: copies bound QCVM bytes → EntVars fields (after QC)
- **Only 66 bound fields** are synced. Extension fields (state, speed, wait,
  pos1, pos2, finaldest, think1, count, delay, killtarget, trigger_field,
  th_checkattack, customflags, target2/3/4, etc.) exist ONLY in QCVM bytes.
- **Pusher entities** (MOVETYPE_PUSH) require special handling because their
  state (velocity, nextthink, think, ltime) is set by QC but must be seen by
  Go's PhysicsPusher. Non-pusher mutations are handled by snapshot/diff/restore.

## Parity Gaps Found

### Gap 1: `executeQCFunction` was missing pusher sync (FIXED)

**C**: `PR_ExecuteProgram` shares memory — no sync needed.

**Go**: `executeQCFunction` (the generic QC callback wrapper) captured
non-pusher snapshots but NOT pusher snapshots. When a non-pusher think
(e.g., `DelayedUse`) called `SUB_UseTargets` targeting a pusher (e.g.,
`func_train`), the pusher's QCVM mutations (velocity, nextthink, think)
were lost — never synced back to Go.

**Fix applied**: Added `capturePusherSnapshots`/`syncPushersToQCVM`/
`syncMutatedPushersFromQCVM` to `executeQCFunction` and
`executeQCFunctionLeavingGlobals`.

**Status**: Fixed and tested.

### Gap 2: `Impact` was missing pusher sync (FIXED)

**C**: `SV_Impact` calls `PR_ExecuteProgram` — shared memory, no sync.

**Go**: `Impact` (direct collision touch dispatch) synced only the two
colliding entities, not pushers. If a touch callback targeted a pusher,
mutations were lost.

**Fix applied**: Added pusher snapshot/sync to both touch dispatch blocks in
`Impact`.

**Status**: Fixed and tested.

### Gap 3: Non-WALK client missing `LinkEdict(ent, true)` (FIXED)

**C**: `SV_Physics_Client` always calls `SV_LinkEdict(ent, true)` after
the movetype switch. This fires `SV_TouchLinks` for trigger detection.

**Go**: `PhysicsWalk` calls `s.LinkEdict(ent, true)` internally, but the
non-WALK path (MOVETYPE_NONE during intermission, etc.) in
`physics_loop.go` did NOT call `LinkEdict(ent, true)`. Trigger touches
wouldn't fire for stationary non-walking clients.

**Fix applied**: Added `s.LinkEdict(ent, true)` for non-WALK client entities
before `PlayerPostThink`.

**Status**: Fixed and tested.

### Gap 4: Selective field sync loses QC-only field mutations

**C**: All entity fields (standard + extension) are in shared memory. When QC
sets `self.th_checkattack = multi_trigger`, C engine code can read it (though
it typically doesn't need to — QC manages these fields internally).

**Go**: Only 66 "bound" fields are synced. Extension fields like
`th_checkattack`, `state`, `customflags`, `trigger_field`, etc. exist only in
QCVM bytes. The Go server never sees them.

**Impact**: Generally OK — QC manages these fields internally and the Go
engine doesn't need to read them. BUT: `syncEdictToQCVM` runs before every
callback and writes bound fields from Go → QCVM. If a bound field's Go value
is stale (e.g., `Think` was changed by QC but not synced back), the Go value
OVERWRITES the correct QCVM value.

**Example**: If `multi_trigger` sets `self.think = multi_wait` (bound field
`Think`), and this is synced back to Go, the next `syncEdictToQCVM` will write
the correct Go value back to QCVM. But if the sync-back failed (as in Gap 1),
the stale Go value (Think=0) would overwrite the QCVM value on the next frame.

**Status**: The root cause of the sync failures. Gaps 1 and 2 were specific
instances of this problem. The fix in Gap 1 should resolve most cases, but
the architecture remains fragile.

### Gap 5: `syncPushersToQCVM` can overwrite QCVM state with stale Go values

**C**: No sync — QC and engine share memory. No overwrite possible.

**Go**: `syncPushersToQCVM` is called before QC callbacks to push Go-side
pusher state to QCVM. If the Go-side state is stale (because a previous
callback's mutations weren't synced back), this overwrites the correct QCVM
values with stale Go values.

**Scenario**:
1. Frame N: Callback A sets pusher velocity in QCVM → synced to Go ✓
2. Frame N+1: `syncPushersToQCVM` pushes Go velocity → QCVM (correct, matches)
3. Frame N+1: Callback B runs but its pusher mutations are NOT synced back
4. Frame N+2: `syncPushersToQCVM` pushes stale Go velocity → QCVM, OVERWRITING
   the mutations from Callback B

**Status**: This is the pattern that caused the original trigger bug. The fix
in Gap 1 ensures all callbacks sync pushers back, which should prevent this.
But the pattern is inherently fragile — any new dispatch path that forgets
pusher sync will reintroduce the bug.

### Gap 6: Debounce/timing sensitivity

**C**: `multi_trigger` sets `self.nextthink = time + self.wait`. On the next
frame, `self.nextthink > time` causes early return. `self.nextthink` is in
shared memory — the engine's `SV_RunThink` sees it and doesn't fire the think.
The trigger's `touch` function runs, calls `multi_trigger`, which checks
`nextthink > time` and returns. Simple, direct.

**Go**: `multi_trigger` sets `self.nextthink` in QCVM. This must be synced
back to Go (`NextThink` is a bound field). On the next frame,
`syncEdictToQCVM` pushes Go's `NextThink` back to QCVM. If the sync works
correctly, the behavior matches C. If the sync fails (e.g., the callback's
mutations weren't synced back), `NextThink` stays 0 in Go, gets pushed as 0
to QCVM, and `multi_trigger` fires again — no debounce.

**Status**: The user reports the trigger "constantly fires" rather than being
debounced. This suggests `NextThink` is not being properly synced back after
`multi_trigger` sets it. The Gap 1 fix should address this for the
`executeQCFunction` path, but the `touchLinks` path was already handling
pusher sync. The issue might be in how `touchLinks` syncs the trigger entity
specifically.

### Gap 7: `touchLinks` does double sync with `executeQCFunction`

**C**: `SV_TouchLinks` calls `PR_ExecuteProgram` — shared memory, no sync.

**Go**: `touchLinks` does:
1. `syncEdictToQCVM(touchNum, touch)` — push trigger to QCVM
2. `syncEdictToQCVM(entNum, ent)` — push player to QCVM
3. `capturePusherSnapshots()` + `syncPushersToQCVM()` — push all pushers
4. `executeQCFunction(touch)` — which now ALSO does:
   a. `captureNonPusherQCVMEdictSnapshots()` — includes trigger
   b. `capturePusherSnapshots()` + `syncPushersToQCVM()` — redundant with 3
   c. Execute callback
   d. `syncMutatedNonPushersFromQCVM()` — syncs trigger back
   e. `syncMutatedPushersFromQCVM()` — syncs pushers back
5. `syncEdictFromQCVM(touchNum, touch)` — syncs trigger again (redundant with 4d)
6. `syncEdictFromQCVM(entNum, ent)` — syncs player
7. `syncMutatedPushersFromQCVM(pusherSnapshots)` — syncs pushers (redundant with 4e)

The redundancy is harmless but wasteful. The real concern is ordering:
- Step 3 pushes pushers to QCVM
- Step 4b pushes pushers to QCVM again (redundant)
- Step 4c executes the callback (which may mutate pushers)
- Step 4e syncs pushers back (from 4b's snapshot, not 3's)
- Step 7 syncs pushers back (from 3's snapshot)

If 4e already synced the pushers, step 7's snapshot (from step 3) is stale.
`syncMutatedPushersFromQCVM` compares with the snapshot — if the pusher was
already updated by 4e, the Go value now matches the QCVM value, so step 7
would see no change and skip. This should be safe but confusing.

**Status**: Not a bug, but the code is confusing and the double-sync is
wasteful. Should be cleaned up.

### Gap 8: `activator` global is not saved/restored around callbacks

**C**: `SV_Impact` and `SV_TouchLinks` save/restore `self` and `other`:
```c
old_self = pr_global_struct->self;
old_other = pr_global_struct->other;
// ... execute callback ...
pr_global_struct->self = old_self;
pr_global_struct->other = old_other;
```

They do NOT save/restore `activator` — `activator` is managed entirely by QC
code (`SUB_UseTargets` sets it).

**Go**: `captureQCExecutionContext`/`restoreQCExecutionContext` saves/restores
`self`, `other`, `depth`, `localUsed`, `xFunction`, `xFunctionIndex`,
`xStatement`. It does NOT save/restore `activator`. This matches C behavior.

**Status**: Parity achieved.

### Gap 9: `SV_RunThink` think time clamping

**C** (`SV_RunThink`):
```c
thinktime = ent->v.nextthink;
if (thinktime <= 0 || thinktime > qcvm->time + host_frametime)
    return true;
if (thinktime < qcvm->time)
    thinktime = qcvm->time;
```

C clamps `thinktime` down to `qcvm->time` if it's in the past. The think
fires with `pr_global_struct->time = thinktime`.

**Go** (`RunThink`):
```go
thinkTime := ent.Vars.NextThink
if thinkTime <= 0 || thinkTime > s.Time + s.FrameTime {
    return !ent.Free
}
```

Go does NOT clamp `thinkTime` to `s.Time`. If `thinkTime` is in the past
(e.g., `nextthink = 0.1` but `time = 17.0`), Go would set the QC `time` global
to 0.1, which is wrong. C would set it to 17.0 (clamped).

**Wait** — let me check: does Go's `RunThink` set the `time` global?

Looking at `RunThink` in physics.go:
```go
func (s *Server) RunThink(ent *Edict) bool {
    ...
    s.setQCTimeGlobal(s.Time)  // sets time to server time, not thinktime
    s.QCVM.SetGlobal("self", entNum)
    ...
}
```

Go always sets `time` to `s.Time`, regardless of `thinkTime`. C sets `time` to
the clamped `thinktime`. This is a subtle difference: in C, the think function
sees `time = thinktime` (which might be less than `qcvm->time` if the think
was scheduled in the past). In Go, the think function always sees `time = s.Time`.

For most cases this doesn't matter because `thinktime` is usually close to
`qcvm->time`. But for pushers with frozen `ltime`, `SUB_CalcMove` sets
`nextthink = ltime + traveltime`, which could be much less than `time`. C
would clamp it to `time`, but Go doesn't need to because `PhysicsPusher`
handles think dispatch differently (using `ltime`, not `time`).

Actually, `SV_RunThink` is used for non-pusher entities. `PhysicsPusher` has
its own think dispatch that uses `ltime`. So this difference only affects
non-pusher thinks, where `nextthink` is set using global `time` (not `ltime`),
so `thinktime` should always be close to `time`. The clamping in C is a safety
net for edge cases.

**Status**: Minor difference, unlikely to cause issues. Should be fixed for
parity but low priority.

### Gap 10: `PhysicsPusher` think sets `time` to global time, C does too

**C** (`SV_Physics_Pusher`):
```c
if (thinktime > oldltime && thinktime <= ent->v.ltime)
{
    ent->v.nextthink = 0;
    pr_global_struct->time = qcvm->time;
    pr_global_struct->self = EDICT_TO_PROG(ent);
    pr_global_struct->other = EDICT_TO_EDICT(qcvm->edicts);
    PR_ExecuteProgram (ent->v.think);
}
```

**Go** (`PhysicsPusher`):
```go
if thinkTime > oldLTime && thinkTime <= ent.Vars.LTime {
    ent.Vars.NextThink = 0
    s.setQCTimeGlobal(s.Time)
    s.QCVM.SetGlobal("self", entNum)
    s.QCVM.SetGlobal("other", 0)
    // ... execute think ...
}
```

Both set `time` to global server time. Both set `other` to world (0). Parity.

**Status**: Parity achieved.

### Gap 11: `touchLinks` does not save/restore `self`/`other` in the C style

**C** (`SV_TouchLinks`):
```c
old_self = pr_global_struct->self;
old_other = pr_global_struct->other;
// ... per-trigger:
pr_global_struct->self = EDICT_TO_PROG(touch);
pr_global_struct->other = EDICT_TO_PROG(ent);
PR_ExecuteProgram (touch->v.touch);
// ... after all triggers:
pr_global_struct->self = old_self;
pr_global_struct->other = old_other;
```

C saves/restores `self`/`other` ONCE, around the entire touch dispatch loop.
Each trigger callback gets `self`/`other` set fresh.

**Go** (`touchLinks`):
```go
ctx := captureQCExecutionContext(s.QCVM)
// ... per-trigger:
s.QCVM.SetGlobal("self", touchNum)
s.QCVM.SetGlobal("other", entNum)
s.executeQCFunction(int(touch.Vars.Touch))
// ... after all triggers:
restoreQCExecutionContext(s.QCVM, ctx)
```

Go saves/restores the full execution context (including self/other) ONCE,
around the entire loop. But `executeQCFunction` ALSO saves/restores the
context. So the restore happens twice — once inside `executeQCFunction` (per
trigger), and once in `touchLinks` (after all triggers). The double restore is
harmless but redundant.

**Status**: Parity achieved (behavior matches, code is redundant).

### Gap 12: `syncEdictToQCVM` before callback can overwrite QC-only fields

**C**: No sync — QC fields are in shared memory, not overwritten.

**Go**: `syncEdictToQCVM` writes bound fields from Go → QCVM before callbacks.
It does NOT touch unbound (QC-only) fields. So QC-only fields like
`th_checkattack`, `state`, `customflags` persist in QCVM across callbacks.

BUT: `clearQCVMEdictData` (called during `AllocEdict` and `FreeEdict`) zeroes
the ENTIRE QCVM edict data, including QC-only fields. If an edict is freed and
reallocated, all QC-only fields are zeroed. This matches C's `ED_ClearEdict`
which zeroes `ent->v` (all entity fields).

**Status**: Parity achieved for this specific case.

### Gap 13: `force_retouch` handling

**C** (`SV_Physics`):
```c
if (pr_global_struct->force_retouch)
{
    SV_LinkEdict (ent, true);
}
// ... movetype dispatch ...
// After the loop:
if (pr_global_struct->force_retouch)
    pr_global_struct->force_retouch--;
```

C reads `force_retouch` inside the loop (per-entity) and decrements it once
after the loop.

**Go** (`Physics`):
```go
forceRetouch := float32(0)
if s.QCVM != nil {
    forceRetouch = s.QCVM.GlobalFloat("force_retouch")
}
for i := 0; i < entityCap; i++ {
    if forceRetouch != 0 {
        s.LinkEdict(ent, true)
    }
    // ... movetype dispatch ...
}
if s.QCVM != nil {
    if forceRetouch := s.QCVM.GlobalFloat("force_retouch"); forceRetouch > 0 {
        next := forceRetouch - 1
        if next < 0 { next = 0 }
        s.QCVM.SetGlobal("force_retouch", next)
    }
}
```

Go reads `force_retouch` ONCE before the loop and caches it. C reads it
per-entity inside the loop. If QC code (e.g., `StartFrame`) modifies
`force_retouch` during the loop, C would see the new value for subsequent
entities; Go would not.

In practice, `force_retouch` is only set by QC code outside the physics loop
(e.g., `trigger_reactivate` sets `force_retouch = 2` inside a touch callback,
which runs during `LinkEdict(ent, true)` inside the loop). C would pick up the
new value for subsequent entities. Go would not — it cached the value at the
start.

**Status**: Minor parity gap. `force_retouch` is rarely modified during the
loop, so this is low impact. Should be fixed for full parity.

## Parity Plan

### Phase 1: Eliminate sync as a source of bugs (HIGH PRIORITY)

The sync layer is the root cause of all trigger bugs. Every dispatch path that
calls QC code must sync pushers and non-pushers back. Instead of patching each
path individually, **centralize the sync in `executeQCFunction`**:

1. **Make `executeQCFunction` the single sync point**: It should:
   - Capture ALL entity snapshots (pusher + non-pusher) before execution
   - Sync ALL entities (Go → QCVM) before execution
   - Execute the QC function
   - Sync ALL mutated entities (QCVM → Go) after execution
   - This eliminates the need for callers (`touchLinks`, `Impact`,
     `PhysicsPusher`, `RunThink`) to do their own sync

2. **Remove redundant sync from callers**: `touchLinks`, `Impact`, and
   `PhysicsPusher` should no longer do their own pusher/non-pusher sync since
   `executeQCFunction` handles it. They should only set `self`/`other`/`time`
   globals.

3. **Add a "sync all" function**: Instead of capturing/syncing subsets (pusher
   vs non-pusher), sync ALL entities in one pass. This is simpler and avoids
   the classification issue. Performance impact is negligible for typical
   entity counts.

### Phase 2: Fix remaining parity gaps (MEDIUM PRIORITY)

4. **Fix `RunThink` time clamping** (Gap 9): Clamp `thinkTime` to `s.Time`
   if it's in the past, matching C's `SV_RunThink`.

5. **Fix `force_retouch` per-entity read** (Gap 13): Read `force_retouch`
   inside the entity loop, not once before it. This matches C's behavior
   where QC code can modify `force_retouch` during the loop.

### Phase 3: Consider architectural improvements (LOW PRIORITY)

6. **Evaluate unifying entity storage**: The ideal fix is to eliminate the
   dual-storage architecture. Options:
   - Store all entity data in QCVM bytes and have Go read/write directly
     (matching C's shared-memory model)
   - Extend `EntVars` to include all QC extension fields (state, speed, wait,
     etc.) so they're all bound and synced
   - Use a single byte array for entity data with typed accessors

   This is a large refactor but would eliminate the entire class of sync bugs.

7. **Add a "dirty flag" system**: Instead of snapshot/diff/restore, mark
   entities as dirty when QC modifies them. After QC execution, sync only
   dirty entities. This is more efficient and less error-prone than the
   current snapshot approach.

### Phase 4: Testing and verification

8. **Add integration test**: Load qbj2 start map, teleport player to each
   trigger, verify the full activation chain works (trigger → button → train).

9. **Add C/Go comparison test**: Run the same scenario in both C Ironwail
   and Go, compare entity state after each frame.

10. **Add sync coverage test**: For every dispatch path that calls QC code,
    verify that pusher and non-pusher mutations are synced back.

## Summary of Changes Already Applied

| Fix | File | Description |
|-----|------|-------------|
| Impact pusher sync | physics.go | Added pusher snapshot/sync to both touch blocks |
| Non-WALK LinkEdict | physics_loop.go | Added `LinkEdict(ent, true)` for non-WALK clients |
| executeQCFunction pusher sync | qc_trace.go | Added pusher snapshot/sync to generic QC wrapper |
| executeQCFunctionLeavingGlobals pusher sync | qc_trace.go | Same for StartFrame path |
| Renderer log downgrade | world_geometry_gogpu.go, world_render_gogpu.go, diag_atlas.go | INFO → DEBUG for diagnostic logs |
| Trigger debug logging | debug_trigger.go, world.go, physics.go, server.go, debug_telemetry.go | New `sv_debug_trigger` cvar with console output |
