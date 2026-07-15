# Phase 3 Proposal: Unified Entity Storage

## Goal

Eliminate the dual-storage sync layer by making the QCVM byte array the
single source of truth for ALL entity fields, matching C's shared-memory
architecture. No `unsafe` package usage.

## C Architecture (Canonical)

```
qcvm->edicts = malloc'd array of edict_t
  each edict_t:
    engine fields (free, area, baseline, alpha, ...)
    entvars_t v          ← first 105 float-slots (standard fields)
    extension fields     ← additional slots (state, speed, pos1, etc.)
```

- QC bytecode reads/writes `&ed->v[fieldOffset]` directly
- C engine code reads/writes `ed->v.field` directly
- **Same memory. No sync. No copy.**

## Current Go Architecture (Problem)

```
s.Edicts []*Edict          (Go structs)
  each Edict:
    engine fields (Free, AreaPrev, AreaNext, ...)
    Vars *EntVars          ← 66 typed fields, Go's "source of truth"

s.QCVM.Edicts []byte       (flat byte array)
  each entity:
    28-byte header
    EntityFields float-slots  ← QC's "source of truth"
```

- Go physics/networking reads `ent.Vars.Origin` (typed struct)
- QC bytecode reads `vm.EFloat(entNum, EntFieldOrigin)` (byte array)
- Sync layer copies between them at every callback boundary
- Only 66 of ~213 fields are synced; extension fields are QCVM-only

## Proposed Go Architecture

### Core idea: Make QCVM byte array the single source of truth

The `Edict` struct keeps its engine fields (Free, AreaPrev, AreaNext, etc.)
but `Vars *EntVars` is replaced by **typed accessor methods** that read/write
directly to the QCVM byte array. No `unsafe` — use `math.Float32frombits` /
`math.Float32bits` with `encoding/binary.LittleEndian` (same as current
`vm_edict.go`).

### Design

```
type Edict struct {
    // Engine fields (unchanged)
    Free       bool
    AreaPrev   *Edict
    AreaNext   *Edict
    NumLeafs   int
    LeafNums   [32]int
    Baseline   EntityState
    Alpha      uint8
    Scale      uint8
    ForceWater     bool
    SendForceWater bool
    SendInterval   bool
    OldFrame       float32
    OldThinkTime   float32
    FreeTime   float32

    // Entity number — indexes into QCVM byte array
    Num int

    // Removed: Vars *EntVars
}
```

#### Field access via typed methods on Edict

```go
// The Server holds a reference to the QCVM for field access.
// Edict methods take a *qc.VM parameter (or the Server holds a
// non-exported pointer and Edict stores an index).

func (e *Edict) Origin(s *Server) [3]float32 {
    return s.QCVM.EVector(e.Num, qc.EntFieldOrigin)
}
func (e *Edict) SetOrigin(s *Server, v [3]float32) {
    s.QCVM.SetEVector(e.Num, qc.EntFieldOrigin, v)
}

func (e *Edict) Velocity(s *Server) [3]float32 {
    return s.QCVM.EVector(e.Num, qc.EntFieldVelocity)
}
func (e *Edict) SetVelocity(s *Server, v [3]float32) {
    s.QCVM.SetEVector(e.Num, qc.EntFieldVelocity, v)
}

func (e *Edict) Think(s *Server) int32 {
    return s.QCVM.EInt(e.Num, qc.EntFieldThink)
}
func (e *Edict) SetThink(s *Server, fn int32) {
    s.QCVM.SetEInt(e.Num, qc.EntFieldThink, fn)
}

// ... etc for all fields
```

#### Extension fields also accessible

```go
// QC-only fields accessible via FieldDefs lookup
func (e *Edict) State(s *Server) float32 {
    ofs := s.QCFieldState  // cached offset from FindField("state")
    return s.QCVM.EFloat(e.Num, ofs)
}
func (e *Edict) SetState(s *Server, v float32) {
    s.QCVM.SetEFloat(e.Num, s.QCFieldState, v)
}
```

### What gets removed

1. **`EntVars` struct** — no longer needed; fields read/written via accessors
2. **All sync functions**:
   - `syncEdictToQCVM` / `syncEdictFromQCVM`
   - `syncEntVarsToQC` / `syncEntVarsFromQC`
   - `syncPushersToQCVM` / `syncPushersFromQCVM`
   - `syncMutatedPushersFromQCVM` / `syncMutatedNonPushersFromQCVM`
   - `capturePusherSnapshots` / `captureNonPusherQCVMEdictSnapshots`
   - `qcSyncCacheForVM` / `entFieldBinding` / `buildQCFieldOffsets`
3. **`executeQCFunction` snapshot/restore** — simplified to just
   save/restore `self`/`other`/`time` globals (matching C)
4. **`touchLinks` pusher sync** — removed; just set globals and execute
5. **`Impact` pusher sync** — removed; just set globals and execute
6. **`PhysicsPusher` pusher sync** — removed; just set globals and execute

### What stays

1. **`Edict` struct** — engine fields (Free, AreaPrev, AreaNext, Baseline, etc.)
2. **`QCVM.Edicts []byte`** — the single source of truth
3. **`EFloat`/`EInt`/`EVector`/`EString`/`SetE*`** — already exist, no change
4. **`EntField*` constants** — field offsets, no change
5. **`FieldDefs` from progs.dat** — extension field lookup, no change
6. **`captureQCExecutionContext`/`restoreQCExecutionContext`** — saves
   `self`/`other` globals (matching C's `old_self`/`old_other`)

## Migration Strategy

### Step 1: Add accessor methods (non-breaking)

Add typed accessor methods to `Edict` that read/write via QCVM. Keep `Vars`
for now. This is purely additive.

**Files touched**:
- `internal/server/types_entities.go` — add accessor methods
- `internal/server/server.go` — cache extension field offsets at load time

**Effort**: Medium. ~60 accessors for standard fields + ~20 for common
extension fields. Each is a 2-line getter/setter.

**Risk**: None. Existing code keeps using `ent.Vars.Origin`. New code can
use `ent.Origin(s)`.

### Step 2: Migrate hot-path code to accessors (non-breaking)

Replace `ent.Vars.Origin` with `ent.Origin(s)` in physics, networking, world
code. This changes reads/writes to go directly to QCVM bytes instead of the
Go struct.

**Files touched**:
- `internal/server/physics.go` — ~30 field accesses
- `internal/server/world.go` — ~20 field accesses
- `internal/server/sv_send.go` — ~50 field accesses
- `internal/server/user.go` — ~40 field accesses
- `internal/server/user_spawn.go` — ~20 field accesses
- `internal/server/physics_loop.go` — ~10 field accesses

**Effort**: Large but mechanical. Find-and-replace with review.

**Risk**: Low. Each change is a direct field→accessor swap. Tests catch
regressions.

**Critical**: After this step, `Vars` is no longer the source of truth — the
QCVM byte array is. But sync functions still run, so `Vars` stays in sync
(belt-and-suspenders). This means bugs in sync no longer cause issues
because the accessor reads directly from QCVM.

### Step 3: Remove sync functions (breaking)

Once all Go code uses accessors (reads from QCVM bytes), the sync functions
are dead code. Remove them.

**Files touched**:
- `internal/server/server_qc_sync.go` — delete most of the file
- `internal/server/qc_trace.go` — simplify `executeQCFunction` to just
  save/restore globals + execute (no snapshots)
- `internal/server/world.go` — simplify `touchLinks` (remove pusher sync)
- `internal/server/physics.go` — simplify `Impact` and `PhysicsPusher`
- `internal/server/edict.go` — remove `EntVars` from `Edict`

**Effort**: Medium. Mostly deletion.

**Risk**: Medium. Must verify no code still reads `Vars` after removal.
Tests + compilation catch this.

### Step 4: Remove `EntVars` struct (breaking)

Delete the `EntVars` struct entirely. All field access is via accessor
methods or QCVM `E*`/`SetE*` functions.

**Files touched**:
- `internal/server/types_entities.go` — delete `EntVars`
- `internal/server/savegame.go` — rewrite to use QCVM bytes instead of
  struct copy (save/load entity byte data directly)
- `internal/server/edict.go` — remove `Vars` field from `Edict`

**Effort**: Medium. Savegame needs rework (currently does bulk `*ent.Vars`
copy — needs to copy QCVM byte range instead).

**Risk**: Medium. Savegame is the trickiest part.

### Step 5: Simplify callback dispatch (cleanup)

Now that there's no sync, callback dispatch becomes trivial — exactly
matching C:

```go
func (s *Server) Impact(e1, e2 *Edict) {
    if e1.Touch(s) != 0 && e1.Solid(s) != SolidNot {
        s.QCVM.SetGlobal("self", e1.Num)
        s.QCVM.SetGlobal("other", e2.Num)
        s.setQCTimeGlobal(s.Time)
        s.executeQCFunction(int(e1.Touch(s)))
    }
    // ... same for e2
}

func (s *Server) touchLinks(ent *Edict) {
    // collect candidates
    // for each candidate:
    //   set self/other globals
    //   executeQCFunction(touch)
    // no sync, no snapshots, no pusher handling
}
```

**Effort**: Small. Mostly deletion of sync boilerplate.

## Performance Considerations

### Current: struct field access
```go
ent.Vars.Origin  // direct memory read, ~1ns
```

### Proposed: QCVM byte array access
```go
s.QCVM.EVector(e.Num, qc.EntFieldOrigin)
// Internally:
//   offset = e.Num * EdictSize + 28 + fieldOfs * 4
//   data = vm.Edicts[offset : offset+12]
//   math.Float32frombits(LittleEndian.Uint32(...))
//   ~5-10ns per vector read
```

**Impact**: ~5-10x slower per field access. But:
- Entity field access is NOT the bottleneck (rendering and BSP traversal are)
- C also does pointer arithmetic + struct field access (similar cost)
- The sync overhead (snapshot/diff/restore ALL entities) is far more expensive
  than a few extra ns per field read
- Net performance should IMPROVE by removing sync

### Optimization: Cache EdictData slice

```go
func (e *Edict) data(s *Server) []byte {
    return s.QCVM.EdictData(e.Num)
}

// Hot loops can cache the data slice:
data := ent.Data(s)
origin := [3]float32{
    readFloat32(data, qc.EntFieldOrigin*4),
    readFloat32(data, qc.EntFieldOrigin*4+4),
    readFloat32(data, qc.EntFieldOrigin*4+8),
}
```

This reduces to ~2-3ns per field read (just byte slicing + Float32frombits).

### Optimization: Generated accessors

For ultimate performance, generate accessor code with `go generate` that
inlines the byte offset calculations. But this is premature optimization —
the current `EFloat`/`EVector` functions are already fast enough.

## What about `unsafe`?

This proposal uses **zero `unsafe`**. All field access uses:
- `math.Float32frombits` / `math.Float32bits` (already used in `vm_edict.go`)
- `encoding/binary.LittleEndian.Uint32` / `PutUint32` (alternative)
- Direct byte slice indexing (`data[offset]`)

These are 100% safe Go. The compiler may not be able to inline as aggressively
as `unsafe` pointer casts, but the performance difference is negligible for
this use case.

## Alternative: Extend EntVars with all fields

Instead of removing `EntVars`, extend it to include ALL QC extension fields
(state, speed, wait, pos1, pos2, finaldest, think1, count, delay, killtarget,
trigger_field, th_checkattack, customflags, target2/3/4, etc.).

**Pros**: Keeps typed struct access (fast, ergonomic)
**Cons**:
- EntVars becomes huge (~200+ fields)
- Must be rebuilt for each mod's progs.dat (different mods have different
  extension fields)
- Reflection-based sync still needed (just more fields to sync)
- Doesn't eliminate sync — just makes it complete
- Still has the "stale Go value overwrites QCVM" problem

**Verdict**: Rejected. It makes sync complete but doesn't eliminate it.
The fundamental fragility remains.

## Alternative: Mirror struct backed by byte array

Use `unsafe.Pointer` to overlay a Go struct on top of the byte array:
```go
vars := (*EntVars)(unsafe.Pointer(&vm.Edicts[offset+28]))
```

**Pros**: Zero-copy, zero-sync, typed access, fastest possible
**Cons**: Uses `unsafe` (user wants to avoid), struct layout must exactly
match the QCVM byte layout (endianness, padding, field order), not portable

**Verdict**: Rejected per user's request to avoid `unsafe`.

## Recommendation

**Adopt the accessor method approach** (the primary proposal). It:
- Eliminates sync entirely (the root cause of all trigger bugs)
- Matches C's shared-memory semantics (single source of truth)
- Uses zero `unsafe`
- Is incremental (Step 1 is non-breaking, Step 2 is non-breaking)
- Has acceptable performance (QCVM byte access is already used by the VM)
- Simplifies callback dispatch to match C exactly

### Migration order

1. **Step 1** (add accessors) — safe, non-breaking, can be done first
2. **Step 2** (migrate hot paths) — mechanical, test-covered
3. **Step 3** (remove sync) — deletion, test-covered
4. **Step 4** (remove EntVars) — final cleanup
5. **Step 5** (simplify dispatch) — cleanup

Each step is independently testable and committable. Steps 1-2 can be done
in parallel with other work. Steps 3-5 should be done after Step 2 is
verified.

## Estimated Effort

| Step | Effort | Risk | Can parallelize |
|------|--------|------|-----------------|
| 1: Add accessors | Medium | None | Yes |
| 2: Migrate hot paths | Large (mechanical) | Low | Yes |
| 3: Remove sync | Medium (deletion) | Medium | After 2 |
| 4: Remove EntVars | Medium | Medium | After 3 |
| 5: Simplify dispatch | Small | Low | After 4 |

Total: ~2-3 days of focused work, with each step independently shippable.
