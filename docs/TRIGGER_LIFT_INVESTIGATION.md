# Trigger/Lift Investigation — qbj2 start

## Problem

Some triggers in qbj2 start do not work. Specifically, a trigger that should
cause a lift (func_train) to move downwards does nothing.

## Map Entity Layout (qbj2 start)

The "lift" is a **func_train** (not a func_plat), named `lift_main`. It follows
path_corners `lift_mainup` (z=-2000) and `lift_maindown` (z=-6608).

### Entity chain

| Entity | classname | targetname | target | model | Notes |
|--------|-----------|------------|--------|-------|-------|
| | trigger_multiple | | lift_main_buttonbottom | *35 | Player touches this |
| | func_button | lift_main_buttonbottom | lift_main | *34 | Pressed by trigger |
| | trigger_multiple | | lift_main_buttonright | *40 | |
| | func_button | lift_main_buttonright | lift_main | *39 | |
| | trigger_multiple | | lift_main_buttonleft | *42 | |
| | func_button | lift_main_buttonleft | lift_main | *41 | |
| | trigger_multiple | | lift_main_buttontop | *75 | |
| | func_button | lift_main_buttontop | lift_main | *74 | |
| | path_corner | lift_mainup | lift_maindown | | origin=(-112, 656, -2000), speed=600, wait=-1 |
| | path_corner | lift_maindown | lift_mainup | | origin=(-112, 656, -6608), speed=600, wait=-1 |
| | func_train | lift_main | lift_mainup | *73 | spawnflags=3 (PATH_SPEED+TOGGLE), dmg=10, sounds=2 |

### Trigger flow

1. Player touches `trigger_multiple` → `multi_touch()` → `multi_trigger()` → `SUB_UseTargets()`
2. `SUB_UseTargets` calls `find(world, targetname, self.target)` → finds `func_button` by targetname
3. Calls `button.use()` → `button_use()` → `button_fire()` → `SUB_CalcMove(self.pos2, speed, button_wait)`
4. Button moves to pos2; on arrival `button_wait()` fires → `SUB_UseTargets()` → finds `func_train` by targetname
5. Calls `train.use()` → `train_use()` → `train_next()` → `SUB_CalcMove(corner.origin - self.mins, speed, train_wait)`
6. Train moves to next path_corner

## Copper Mod QC Differences (qbj2 progs.dat)

This is a **Copper** mod (not vanilla Quake). Key QC differences:

### CheckValidTouch (triggers.qc)
Copper's `CheckValidTouch()` adds two checks vs vanilla:
- `other.movetype == MOVETYPE_NOCLIP` → return FALSE
- `other.flags & CFL_LIMBO` → return FALSE (coop teleport limbo)

### multi_touch (triggers.qc)
Copper uses `self.th_checkattack()` instead of directly calling `multi_trigger()`.
`th_checkattack` is set to `multi_trigger` or `multi_trigger_coop` during spawn.

### multi_trigger (triggers.qc)
Copper adds:
- `CFL_LOCKED` customflags check
- `count` decrement with self-removal at 0
- `SUB_UseTargetsSilent()` for trigger_secret
- `sound_delayed()` with delay support

### plat_spawn_inside_trigger (plats.qc)
Copper **defers** trigger setup when plat has a `target` field:
```c
if (self.target != string_null) {
    self.think = plat_try_find_trigger;
    self.nextthink = time + 0.1;
    return;
}
```
`plat_try_find_trigger` finds an existing trigger_once/multiple by targetname
and repurposes it as the plat's activation zone (sets `t.touch = plat_center_touch`).

### func_train_setup (plats.qc)
Sets `self.nextthink = self.ltime + 0.1` and `self.think = func_train_find`.
The train starts on the second frame to ensure targets have spawned.

### func_train_find (plats.qc)
If train has `targetname` (i.e., is triggered), does NOT start moving.
Only starts moving if `self.targetname == string_null`.

### train_next (plats.qc)
Uses `findunlockedtarget(world)` instead of `find()` — supports `target2`,
`target3`, `target4` fields and skips `CFL_LOCKED` entities.

### SUB_CallAsSelf (subs.qc)
Copper wrapper for self-swapping pattern:
```c
void(void() fun, entity newself) SUB_CallAsSelf = {
    entity oself = self;
    self = newself;
    fun();
    self = oself;
};
```

## Architecture: QC Execution Path

The runtime uses **QCVM bytecode** (progs.dat), NOT the Go-native quakego package.
`pkg/qgo/quakego` is a separate experimental module not imported at runtime.

### Key files
- `internal/server/world.go` — `LinkEdict`, `areaTriggerEdicts`, `touchLinks`
- `internal/server/physics.go` — `Impact`, `PushMove`, `PushEntity`, `PhysicsPusher`
- `internal/server/physics_loop.go` — `Physics()` main loop, `force_retouch` handling
- `internal/server/server_qc_sync.go` — Go↔QCVM bidirectional sync
- `internal/server/server.go` — `ServerHooks` (Find, Spawn, SetModel, etc.)
- `internal/server/qc_trace.go` — `executeQCFunction` with snapshot/restore
- `internal/server/edict.go` — `ED_ParseEdict`, field parsing
- `internal/server/sv_main.go` — `loadMapEntities`, spawn dispatch

### Sync mechanism
Go `EntVars` struct ↔ QCVM edict byte array. Only **bound fields** are synced.
Fields not in Go EntVars (e.g., `state`, `speed`, `wait`, `pos1`, `pos2`,
`finaldest`, `think1`, `count`, `killtarget`, `trigger_field`, `th_checkattack`,
`customflags`, `target2/3/4`) exist **only in QCVM storage** and are NOT overwritten
during sync (sync writes only bound fields, does not clear).

### Touch dispatch paths
1. **touchLinks** (world.go:649) — trigger area touch: player moves into SOLID_TRIGGER
   - Captures pusher snapshots, syncs pushers to QCVM before callback
   - After callback: syncs trigger+player from QCVM, syncs mutated pushers, syncs spawned
2. **Impact** (physics.go:68) — direct collision touch (PushEntity/FlyMove)
   - Syncs e1+e2 to QCVM, executes touch, syncs back
   - **Does NOT sync pushers** — potential gap if touch callback mutates pusher state
3. **PhysicsPusher think** (physics.go:498) — think callback for MOVETYPE_PUSH
   - Syncs pushers to QCVM before think, syncs pushers back after

## PhysicsPusher Analysis

### movetime calculation (matches C SV_Physics_Pusher)
```
thinkTime = ent.NextThink
if thinkTime < LTime + FrameTime:
    movetime = thinkTime - LTime
    if movetime < 0: movetime = 0
else:
    movetime = FrameTime
```

### When NextThink = 0 (idle pusher)
- `thinkTime = 0`, `LTime > 0` → `movetime = 0 - LTime` (negative) → clamped to 0
- **PushMove not called** → **LTime does not advance**
- Think condition `thinkTime > oldLTime` → `0 > LTime` → FALSE → think doesn't fire
- This is **correct behavior** matching C: idle pushers don't advance LTime

### When touch sets NextThink via SUB_CalcMove
- `SUB_CalcMove` sets `nextthink = ltime + traveltime` and `velocity = ...`
- If `traveltime > FrameTime`: `thinkTime > LTime + FrameTime` → `movetime = FrameTime`
- PushMove called with velocity → LTime advances → train moves
- When `LTime >= thinkTime`: think fires (e.g., `button_wait`, `train_wait`)

### LTime semantics
`LTime` = pusher local time, only advances when PushMove is called with non-zero movetime.
This is by design — matches C. Idle pushers have frozen LTime.

## Potential Issues Identified

### 1. Impact does not sync pushers (physics.go:68-123)
`Impact` syncs only the two colliding entities. If a touch callback (e.g.,
`button_touch`) calls `SUB_UseTargets` which targets a pusher, the pusher's
state changes in QCVM may not be synced back to Go.

However, `executeQCFunction` (called inside Impact) does capture non-pusher
snapshots and sync them back. Pushers are excluded from this sync. The caller
(Impact) does not do pusher sync either. **This is a potential gap.**

Compare with `touchLinks` (world.go:744-762) which properly does:
```go
pusherSnapshots := s.capturePusherSnapshots()
s.syncPushersToQCVM()
// ... execute QC ...
s.syncMutatedPushersFromQCVM(pusherSnapshots)
```

Impact does NOT do this. If a button is touched via Impact (direct collision),
and the button's touch function targets a pusher, the pusher's mutations
(velocity, nextthink, think) would be lost.

### 2. Non-WALK client entities missing LinkEdict(ent, true) (physics_loop.go)
C's `SV_Physics_Client` always calls `SV_LinkEdict(ent, true)` after the
movetype switch. Go's `PhysicsWalk` does this (line 757), but the non-WALK
path (lines 117-172) does NOT call `LinkEdict(ent, true)`. This means trigger
touches may not fire for non-walking client entities (e.g., during intermission).

### 3. Copper QC-only fields not in Go EntVars
Fields like `state`, `count`, `speed`, `wait`, `customflags`, `th_checkattack`,
`trigger_field`, `target2/3/4` exist only in QCVM. The Go server cannot read
or write these. This is generally fine (QC manages them), but debugging is harder.

## Telemetry Observations (qbj2 start)

Running with `sv_debug_telemetry` showed:
- Two `func_plat` entities (ent 422, 424) with no targetname/target
  - `ltime=0.000`, `think_time=0.000`, `movetime=0.000` across all frames
  - This is expected: idle plats with NextThink=0 don't advance LTime
- Multiple `trigger_multiple` entities targeting `lift_main_button*`
- All triggers have `touch=730` (multi_touch function index) and `solid=1` (SOLID_TRIGGER)
- No touch events observed (player at spawn, not near triggers)

## C Reference (Ironwail)

Located at `/home/darkliquid/Projects/ironwail`. Key functions:
- `SV_TouchLinks` (world.c:336-380) — collects candidates into array, re-validates before callback
- `SV_AreaTriggerEdicts` (world.c:286-324) — recursive areanode traversal for trigger list
- `SV_LinkEdict` (world.c:467-538) — links entity into trigger_edicts or solid_edicts list
- `SV_Physics` (sv_phys.c:1226-1298) — main loop, force_retouch handling
- `SV_Physics_Pusher` (sv_phys.c:618-652) — pusher physics with LTime/movetime
- `SV_PushMove` (sv_phys.c:434-607) — moves pusher, pushes entities, handles blocking
- `SV_Impact` (sv_phys.c:155-179) — dispatches touch for two colliding entities

## Fixes Applied

### Fix 1: Impact pusher sync (physics.go)

`Impact` (direct collision touch dispatch) was missing pusher snapshot/sync
around touch callbacks. When a touch callback (e.g., `button_touch`) targeted
a MOVETYPE_PUSH entity (e.g., `func_train`), the pusher's mutated fields
(velocity, nextthink, think) were set in QCVM but never synced back to Go.

Added `capturePusherSnapshots` / `syncPushersToQCVM` before the callback and
`syncMutatedPushersFromQCVM` after, matching the pattern already used by
`touchLinks` (world.go:744-762).

**Test**: `TestImpactSyncsPusherMutationsFromQCVM` — verifies a touch callback
that writes velocity/nextthink/think on a pusher entity has those values
synced back to the Go edict.

### Fix 2: Non-WALK client LinkEdict (physics_loop.go)

C's `SV_Physics_Client` unconditionally calls `SV_LinkEdict(ent, true)` after
the movetype switch. Go's `PhysicsWalk` does this internally, but the non-WALK
client path (MOVETYPE_NONE during intermission, etc.) was missing this call,
meaning trigger touches wouldn't fire for stationary non-walking clients.

Added `s.LinkEdict(ent, true)` for non-WALK client entities before
`PlayerPostThink`, matching C's behavior.

## ROOT CAUSE FOUND: executeQCFunction missing pusher sync

### Debug logging
New cvar `sv_debug_trigger` prints to the in-game console:
- `trigger [touchlinks]` / `trigger [impact]` — when a trigger fires, showing ent#, classname, targetname, target, touch/use/think fn, th_checkattack, customflags, state, wait, nextthink
- `find(targetname=...) → ent=N ...` — when SUB_UseTargets searches for targets
- `pusher ent=N ... synced:` — when pusher state changes are synced

### The bug
`executeQCFunction` (qc_trace.go:69) is the generic QC callback wrapper used
by `RunThink`, `Impact`, and other dispatch points. It captured **non-pusher**
snapshots and synced them back after the callback, but did NOT capture or sync
**pusher** (MOVETYPE_PUSH) entities.

When a non-pusher think function (e.g. `DelayedUse` spawned by a button with
`delay=.5`) called `SUB_UseTargets` which targeted a pusher (e.g. `func_train`),
the pusher's mutated fields (velocity, nextthink, think) were set in QCVM but
**never synced back to Go**. The Go-side `PhysicsPusher` never saw the velocity
or nextthink, so the pusher never moved.

### The chain (from player debug logs)
1. Player touches `trigger_multiple` → `multi_touch` → `multi_trigger` → `SUB_UseTargets`
2. `find(targetname="lift_main_buttontop")` → finds `func_button` → calls `button_use`
3. `button_use` → `button_fire` → `SUB_CalcMove` (button starts moving)
4. Button arrives → `button_wait` → `SUB_UseTargets` (with `delay=.5`)
5. `SUB_UseTargets` spawns `DelayedUse` entity (MOVETYPE_NONE) with `think=DelayThink`
6. 0.5s later, `DelayedUse` think fires via `RunThink` → `executeQCFunction`
7. `DelayThink` → `SUB_UseTargets` → `find(targetname="lift_main")` → finds `func_train`
8. Calls `train_use` → `train_next` → `SUB_CalcMove` sets train velocity/nextthink in QCVM
9. **BUG**: `executeQCFunction` syncs non-pushers back but NOT pushers
10. Train's Go-side velocity/nextthink remain 0 → `PhysicsPusher` never moves it

### The fix
Added pusher snapshot/sync to `executeQCFunction` and `executeQCFunctionLeavingGlobals`:
```go
pusherSnapshots := s.capturePusherSnapshots()
s.syncPushersToQCVM()
// ... execute QC ...
s.syncMutatedPushersFromQCVM(pusherSnapshots)
```

This mirrors the pattern already used by `touchLinks` (world.go:744-762) and
the `Impact` fix applied earlier.

### Test
`TestExecuteQCFunctionSyncsPusherMutationsFromNonPusherThink` — verifies a
non-pusher think callback that writes velocity/nextthink/think on a pusher
entity has those values synced back to the Go edict.

### Previous fixes still relevant
1. **Impact pusher sync** (physics.go) — direct collision touches now sync pushers
2. **Non-WALK client LinkEdict** (physics_loop.go) — non-walking clients now
   get `LinkEdict(ent, true)` for trigger touch dispatch
