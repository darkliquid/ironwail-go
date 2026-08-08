# Fix Remaining Test Failures After EntVars Deletion

**Priority**: High — 62 failing tests across 3 packages blocking green CI
**Status**: Completed (All 45 test packages pass green — fixed in commits `86a1a27` through `f5de2c5`)
**Prerequisite**: Commit `47df06d` (EntVars struct deleted, production code builds clean)
**Estimated effort**: 1 focused session

---

## 1. Current State

The EntVars struct has been deleted. All production code builds and uses QCVM
accessor methods exclusively. 34/41 test packages pass. The remaining 62
failing tests (61 in `internal/server`, 8 in `internal/host`, 1 in
`internal/client`) all share a single root cause: **tests create bare `Edict{}`
structs and set fields via accessor methods, but without a QCVM (or with
`vm.NumEdicts` too low), the writes silently no-op**.

### Failing tests by package

| Package | Failing | Root cause |
|---------|---------|------------|
| `internal/server` | 62 | No QCVM or `NumEdicts` too low for accessor writes |
| `internal/host` | 8 | `autosave_test.go` mock server passes `nil` to accessors |
| `internal/client` | 1 | `client_parse_misc_test.go` sets entity origin via accessor without QCVM |

---

## 2. Root Cause Analysis

### 2.1 The `NumEdicts` problem

`newServerTestVM` (in `server_hooks_test.go:11`) sets `vm.NumEdicts = 1`.
Accessor methods call `s.QCVM.SetEFloat(e.Num, ...)` which internally checks
`e.Num < vm.NumEdicts` — so any write to edict number >= 1 silently fails.

**Fix**: Change `newServerTestVM` to set `vm.NumEdicts = max(s.NumEdicts, maxEdicts)`.

### 2.2 The `fieldDef` fallback problem

`parseQCVMEdictFieldValue` in `edict.go` uses `em.fieldDef(keyName)` which
scans `vm.FieldDefs`. Tests that don't set up `vm.FieldDefs` for all standard
fields cause QCVM writes to be silently skipped during entity parsing.

**Fix**: Add `defaultEntFieldOffsets` fallback in `fieldDef` so standard fields
resolve even without `FieldDefs` entries.

### 2.3 The autosave mock server problem

`autosave_test.go` uses `autosaveTestServer` (a mock), not `*server.Server`.
Accessor calls pass `nil` as the `*Server` argument, so writes silently no-op.
The tests check autosave heuristics that read `Health`, `MoveType`, `Flags`,
`Velocity`, `WaterType`, `Button0`, `ArmorType`, `ArmorValue`, `TeleportTime`.

**Fix**: The `autosaveTestServer` mock needs to store entity field values
directly (e.g., add fields to the mock struct) and the `checkAutosave` code
needs to work with the mock. Alternatively, convert autosave tests to use a
real `*server.Server` with a test QCVM.

### 2.4 The `syncEdictFromQCVM` no-op problem

`syncEdictFromQCVM` is now a no-op (accessors dual-write). Tests that relied on
it to pull QCVM mutations back into the Edict no longer work. One test
(`TestSyncEdictFromQCVM_EmptyModelClearsStaleModelIndex`) explicitly tests this
sync behavior.

**Fix**: The test premise needs updating — the sync is now a no-op, so the test
should verify QCVM data directly via `vm.EInt`/`vm.EFloat`.

### 2.5 The `savegame.go` raw QCVM data problem

`TestSaveGameStateRoundTripsGameplayState` fails because savegame now
serializes raw QCVM bytes instead of `EntVars`. Tests that set up edict state
via accessors need a QCVM to write to, and the round-trip test needs to verify
QCVM data instead of `EntVars` fields.

**Fix**: Set up `newServerTestVM` in the test, set edict fields via accessors,
and verify restored state via QCVM reads.

---

## 3. Execution Strategy

### Step 1: Fix `newServerTestVM` (5 min)

In `server_hooks_test.go:11`, change:
```go
vm.NumEdicts = 1
```
to:
```go
vm.NumEdicts = max(s.NumEdicts, maxEdicts)
```

This is the single highest-impact fix — it will resolve the majority of the 62
failing server tests in one shot.

### Step 2: Add `defaultEntFieldOffsets` fallback to `fieldDef` (5 min)

In `edict.go`, modify `fieldDef` to fall back to `defaultEntFieldOffsets` when
`vm.FieldDefs` doesn't contain a matching field:

```go
func (em *EntityManager) fieldDef(keyName string) (int, qc.EType, bool) {
    // ... existing FieldDefs scan ...
    // Fallback to default offsets for standard fields
    if ofs, ok := defaultEntFieldOffsets[normalizeFieldName(keyName)]; ok {
        // Infer type from the field name
        return ofs, inferFieldType(keyName), true
    }
    return 0, 0, false
}
```

This ensures `parseQCVMEdictFieldValue` writes to QCVM even when `FieldDefs`
isn't fully populated.

### Step 3: Fix `server_test.go` sound test (5 min)

`TestStartSoundUsesExtendedPacketForLargeEntityChannelAndSound` creates an
`Edict` and sets `Mins`/`Maxs` via accessors, but the server has no QCVM. Add
`newServerTestVM(s, 8)` before creating the edict, and set `ent.Num = entNum`.

### Step 4: Fix `world_leafs_test.go` (5 min)

`TestFindTouchedLeafsUsesBoxOnPlaneSideForNonAxialPlanes` sets `AbsMax` via
accessor without a QCVM. Add `newServerTestVM` and set `ent.Num`.

### Step 5: Fix `sv_send_test.go` entity state test (5 min)

`TestEntityStateForClient_AppliesEffectsMask` sets `Effects` via accessor
without a working QCVM. Add `newServerTestVM` and set `ent.Num`.

### Step 6: Fix `sv_send_entities_test.go` (10 min)

Four tests create `Edict{}` and set fields via accessors without QCVM. Add
`newServerTestVM` to each test and set `ent.Num` before accessor calls.

### Step 7: Fix `sv_send_clientdata_test.go` (15 min)

Seven tests create `Edict{}` with accessor-set fields. Each needs
`newServerTestVM` and `ent.Num = 1` before the accessor calls. The test that
checks telemetry output also needs the QCVM populated.

### Step 8: Fix `server_spawn_test.go` (10 min)

Tests that check spawn angles, player state, and color updates need QCVM-backed
edicts. Add `newServerTestVM` and ensure `ent.Num` is set.

### Step 9: Fix `server_pvs_test.go` (5 min)

`TestSyncEdictFromQCVM_EmptyModelClearsStaleModelIndex` — update test premise:
sync is now a no-op, so verify QCVM data directly. Other PVS tests need
`newServerTestVM` for accessor writes.

### Step 10: Fix `user_test.go` noclip/walk tests (10 min)

`TestSVClientThinkNoclip` and related tests set `VAngle`, `MoveType`, etc. via
accessors but the test server may not have `NumEdicts` high enough. Ensure
`newServerTestVM` is called with sufficient `maxEdicts` and `ent.Num` is set.

### Step 11: Fix remaining server test failures (15 min)

Run `go test ./internal/server/... -count=1` and fix any remaining failures
individually. Most should be resolved by Steps 1-2. Remaining ones will likely
be in:
- `server_hooks_test.go` (CheckClient, MoveToGoal, SpawnParms tests)
- `server_hooks_client_test.go` / `server_hooks_moveto_test.go`
- `sv_main_test.go` (spawn trigger tests)
- `physics_test.go` / `physics_runthink_test.go` (may already pass if using `newServerTestVM`)
- `frame_physics_parity_test.go` (may already pass)

### Step 12: Fix `autosave_test.go` (20 min)

The autosave tests use a mock server (`autosaveTestServer`) that isn't
`*server.Server`. Two approaches:

**Option A (recommended)**: Add a `testEdictVars` struct to `autosaveTestServer`
that stores the entity field values directly. Modify `checkAutosave` to accept
an interface or add test-specific accessor methods. This is invasive.

**Option B (simpler)**: Convert autosave tests to use a real `*server.Server`
with `newServerTestVM`. The mock server's purpose was to avoid full server
setup, but with QCVM accessors, a minimal server+VM is lightweight.

### Step 13: Fix `client_parse_misc_test.go` (5 min)

`TestParseLiveServerEntityDatagrams` sets `ent.Origin` via accessor without a
QCVM. Add `newServerTestVM` and ensure `ent.Num` is set so the accessor writes
reach the QCVM.

### Step 14: Fix host loopback/edict count tests (10 min)

`TestLocalLoopbackClientFrameAndSendCommand` and
`TestCmdEdictCountPrintsCanonicalSummary` need QCVM-backed edicts. These tests
use a real server but may not have `newServerTestVM` set up.

### Step 15: Final verification (5 min)

```bash
TMPDIR=.../.tmp CGO_ENABLED=0 go build ./...
TMPDIR=.../.tmp CGO_ENABLED=0 go test ./... -count=1 -timeout 300s
```

All tests should pass.

---

## 4. Key Patterns for Fixing Tests

### Pattern 1: Add QCVM to test server

```go
// Before
s := NewServer()
ent := &Edict{}
ent.SetOrigin(s, [3]float32{10, 20, 30})

// After
s := NewServer()
newServerTestVM(s, 8)  // sets up QCVM with edict storage
ent := &Edict{Num: 1}
ent.SetOrigin(s, [3]float32{10, 20, 30})
```

### Pattern 2: Fix NumEdicts

```go
// In newServerTestVM, change:
vm.NumEdicts = 1
// To:
vm.NumEdicts = max(s.NumEdicts, maxEdicts)
```

### Pattern 3: Read-modify-write for vector fields

```go
// Before (doesn't work — can't assign to return value)
ent.Origin(s)[0] = 42

// After
org := ent.Origin(s)
org[0] = 42
ent.SetOrigin(s, org)
```

### Pattern 4: Verify QCVM data directly (for sync tests)

```go
// Before (syncEdictFromQCVM populated ent.Vars)
s.syncEdictFromQCVM(entNum, ent)
if ent.Vars.ModelIndex != 0 { ... }

// After (accessors read from QCVM directly)
if ent.ModelIndex(s) != 0 { ... }
```

---

## 5. Files to Touch (in order)

1. `internal/server/server_hooks_test.go` — fix `newServerTestVM`
2. `internal/server/edict.go` — add `defaultEntFieldOffsets` fallback to `fieldDef`
3. `internal/server/server_test.go` — add QCVM to sound test
4. `internal/server/world_leafs_test.go` — add QCVM
5. `internal/server/sv_send_test.go` — add QCVM to entity state test
6. `internal/server/sv_send_entities_test.go` — add QCVM to 4 tests
7. `internal/server/sv_send_clientdata_test.go` — add QCVM to 7 tests
8. `internal/server/server_spawn_test.go` — add QCVM to spawn tests
9. `internal/server/server_pvs_test.go` — fix sync test premise
10. `internal/server/user_test.go` — ensure NumEdicts for noclip tests
11. `internal/server/` remaining test files — fix individual failures
12. `internal/host/autosave_test.go` — convert to real server or add mock fields
13. `internal/client/client_parse_misc_test.go` — add QCVM
14. `internal/host/` loopback and edict count tests — add QCVM
