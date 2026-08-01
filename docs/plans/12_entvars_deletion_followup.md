# Follow-Up Plan: Complete EntVars Deletion (Phase 3 Wave 9)

**Priority**: High (architectural cleanup, removes ~400 lines of sync code)
**Prerequisite**: Commit `5efdd1a` (stable baseline — build clean, all tests pass)
**Estimated effort**: 1-2 focused sessions

---

## 1. Current State

The codebase is in a stable state with:

- **Phase 1 fix applied**: `syncQCVMGlobals()` extracted from `syncQCVMState()`.
  Runtime paths (physics_loop, user, rules, user_spawn) use
  `syncQCVMGlobals()` instead of `syncQCVMState()`. This stops the per-frame
  clobbering of QC bytecode mutations that caused the trigger/door/button
  regression.

- **Phase 2 applied**: `touchLinks`, `areaTriggerEdicts`, `LinkEdict`,
  `clipToLinks`, `hullForEntity` in `world.go` migrated to accessor methods.

- **Waves 3-7 applied**: `movement.go`, `sv_client.go`, `sv_stats.go`,
  `svdbg.go`, `qc_trace.go`, and corresponding test files migrated to
  accessor methods.

- **EntVars still exists** as a dual-write mirror. All accessor methods
  (entity_accessors.go, entity_accessors_vec.go) dual-write to both QCVM
  bytes and EntVars struct. The sync layer (server_qc_sync.go) still
  copies between them at init/callback boundaries.

- **Zero-fallback still present**: 62 accessor reader methods have the
  `if v != 0 || e.Vars == nil` pattern that falls back to EntVars when
  QCVM returns zero. This was removed once but caused 15 test failures
  (tests create bare `Edict{Vars: &EntVars{...}}` without QCVM).

---

## 2. What Remains

### 2.1 Production code still using `ent.Vars.*` directly

These files were NOT migrated (batch regex approach broke complex patterns):

| File | Sites | Issue |
|------|-------|-------|
| `server.go` | ~54 | QC builtin implementations with complex write patterns (`+=`, array elements, redundant `vm.SetEVector` calls) |
| `user.go` | ~69 | `SV_ClientThink`, `airMove`, `waterMove` etc. with `ctx.player.Vars.Velocity[0] += ...` patterns |
| `user_spawn.go` | ~33 | `initClientSpawnFallback` with bulk `ent.Vars.* = ...` assignments |
| `server_runtime.go` | ~5 | `SetClientName`, `SetClientColor` with `Vars.NetName`/`Vars.Team` writes |
| `sv_main.go` | ~2 | World entity setup with `Vars: &EntVars{}` |
| `savegame.go` | ~12 | Serializes `EntVars` struct directly, string capture/apply via reflection |
| `savegame_text.go` | ~1 | `Edict{Vars: &EntVars{}, Scale: 16}` |
| `edict.go` | ~19 | `parseEdictFieldValue` reflection path, `ED_Free` Vars clearing, `buildEntVarsFieldIndex` |
| `server_qc_sync.go` | ~14 | `syncEntVarsToQC`, `syncEntVarsFromQC`, `qcSyncCacheForVM`, `newCheckClient` |
| `debug_telemetry.go` | ~6 | `EntitySnapshot` and `entityClassname` EntVars fallback |
| `entity_accessors.go` | ~74 | Dual-write `e.Vars.X = v` in setters, zero-fallback in readers |
| `entity_accessors_vec.go` | ~71 | Same dual-write and zero-fallback patterns |
| `types_entities.go` | 1 | `Vars *EntVars` field on `Edict` struct, `EntVars` struct definition (lines 131-549) |
| Host files | ~61 | `autosave.go`, `commands_gameplay*.go`, `server_browser.go`, `commands_map.go`, `commands_gameplay_save.go` |

**Total**: ~340 production Vars references remaining.

### 2.2 Test code still using `ent.Vars.*`

~125 test Vars references across 15 test files. Most were migrated in the
earlier session but reverted due to EntVars deletion instability.

### 2.3 Missing accessor setters

These setters need to be added to `entity_accessors.go`:
- `SetCurrentAmmo`
- `SetAmmoShells`
- `SetAmmoNails`
- `SetAmmoRockets`
- `SetAmmoCells`
- `SetArmorType`
- `SetArmorValue`
- `SetSpawnFlags`
- `SetDmgInflictor`
- `SetMap`

---

## 3. Execution Strategy

**DO NOT use batch regex/sed for the production files.** The earlier
session proved this approach repeatedly breaks complex write patterns
(`+=`, array element writes, nil-check blocks, `vm.SetEVector` redundancy).
Instead, migrate each file function-by-function with compilation checks.

### Step 1: Add missing accessor setters (5 min)

Add the 10 missing `Set*` methods to `entity_accessors.go`, following
the existing dual-write pattern.

### Step 2: Migrate `server.go` QC builtins (30 min)

This is the largest file (~54 sites). Key functions to migrate:
- `FindRadius` — `ent.Vars.Chain` write
- `CheckClient` — `selfEnt.Vars.Origin`/`ViewOfs` reads, `Vars.Health`/`Flags`
- `SV_Aim` — `ent.Vars.Origin`/`Team`/`TakeDamage` reads, `trace.Entity.Vars.*`
- `SV_CheckBottom` — `e.Vars.Origin`/`Mins`/`Maxs`/`Flags` reads
- `SV_MoveToGoal` — `e.Vars.Origin`/`Mins`/`Maxs`/`Flags`/`GroundEntity` reads
- `SetOrigin` builtin — remove redundant `vm.SetEVector` calls, use accessors
- `SetSize` builtin — same pattern
- `SetModel` builtin — same pattern, complex with `modelBounds`
- `MakeStatic` — `ent.Vars.Origin`/`Angles`/`ModelIndex`/`Frame`/`Colormap`/`Skin`/`Effects`
- `MoveToGoal` hook — `ent.Vars.GoalEntity` read
- `ChangeYaw` hook — nil check removal

**Pattern for each**: read `ent.Vars.X` → `ent.X(s)`, write `ent.Vars.X = v` →
`ent.SetX(s, v)`, array element `ent.Vars.Origin[0]` → `org := ent.Origin(s); org[0]`,
remove `|| ent.Vars == nil` checks, remove redundant `vm.SetEVector` calls
that duplicate what the accessor setter already does.

**Compile after each function.**

### Step 3: Migrate `user.go` (30 min)

Key: `SV_ClientThink`, `airMove`, `waterMove`, `noclipMove`, `waterJump`,
`dropPunchAngle`, `userFriction`, `accelerate`, `airAccelerate`,
`ReadClientMove`, client command handlers.

**Pattern for velocity writes**: `ctx.player.Vars.Velocity[0] += val` →
```go
vel := ctx.player.Velocity(s)
vel[0] += val
ctx.player.SetVelocity(s, vel)
ctx.velocity = vel
```

### Step 4: Migrate `user_spawn.go` (20 min)

`initClientSpawnFallback` has ~20 bulk `ent.Vars.* = ...` assignments.
Convert to `ent.SetX(s, v)` calls.

### Step 5: Migrate `server_runtime.go`, `sv_main.go`, `rules.go` (10 min)

Small files, straightforward replacements.

### Step 6: Migrate host files (30 min)

Each host function needs `srv, _ := subs.Server.(*server.Server)` declared.
Functions that may run with mock servers (autosave, viewframe, setpos)
must not return early when `srv == nil` — the accessor methods handle
`srv == nil` by falling back to `EntVars` (which still exists at this point).

### Step 7: Remove zero-fallback from accessor readers (10 min)

Remove the `if v != 0 || e.Vars == nil` pattern from all 62 reader methods.
**Before doing this, fix the 15 tests that break** (see step 8).

Key test fixes needed:
- `newServerTestVM`: set `vm.NumEdicts = max(s.NumEdicts, 1)`
- `fieldDef` in `edict.go`: add fallback to `defaultEntFieldOffsets` when
  `vm.FieldDefs` doesn't contain a field
- `syncEdictFromQCVM`: also clear QCVM `ModelIndex` when `Model` is empty
- Tests that create `Edict{Vars: &EntVars{...}}` need `Num` field set
  and `syncEdictToQCVM` called to push data to QCVM
- `TestImpactDoesNotClobberExistingPusherStateFromStaleQCVM`: remove
  the QCVM-clearing lines (test premise no longer applies)
- `WriteClientData` tests: add `Num: 1` and `s.syncEdictToQCVM(1, ent)`

### Step 8: Fix `debug_telemetry.go` (5 min)

Replace EntVars fallback paths with QCVM direct access (already done once,
reverted). The `EntitySnapshot` and `entityClassname` functions use `vm`
directly — just remove the `else if ent.Vars != nil` branch.

### Step 9: Rewrite `savegame.go` (30 min)

Replace `EntVars` serialization with raw QCVM byte serialization:
- `SavedEdictState.Vars EntVars` → `SavedEdictState.RawQCVMData []byte`
- `captureSavedEdictStrings` → `captureSavedEdictStringsQCVM(entNum, vm)`
  using `defaultEntFieldOffsets` and `stringEntFieldNames` to find string
  fields in QCVM
- `applySavedEdictStrings` → `applySavedEdictStringsQCVM(entNum, strings, vm)`
  writing to QCVM via `vm.SetEInt`
- Remove `reflect` import

### Step 10: Simplify `edict.go` (20 min)

- Remove `buildEntVarsFieldIndex` function
- Remove `entVarsFieldIndex` variable
- Replace `parseEdictFieldValue` EntVars reflection path with QCVM-only
  (already has `parseQCVMEdictFieldValue` for QCVM writes)
- Remove `ED_Free` Vars clearing block
- Remove `reflect` import

### Step 11: Simplify `server_qc_sync.go` (10 min)

- Make `syncEdictToQCVM` and `syncEdictFromQCVM` no-ops (accessors dual-write)
- Make `syncSpawnedEdictsFromQCVM` only relink (no data copy)
- Remove `syncEntVarsToQC`, `syncEntVarsFromQC`, `qcSyncCacheForVM`,
  `entFieldBinding`, `entFieldKind`, `buildQCFieldOffsets`
- Fix `newCheckClient` to use accessors instead of `Vars`
- Remove `reflect` import

### Step 12: Delete EntVars struct (5 min)

- Remove `Vars *EntVars` field from `Edict` in `types_entities.go`
- Delete `EntVars` struct definition (lines 131-549)
- Remove `&EntVars{}` from all `Edict` struct literals
- Compile and fix remaining errors

### Step 13: Migrate remaining test files (20 min)

~125 test Vars references. Use accessor setters. Tests that create bare
`Edict` structs without a QCVM need `newServerTestVM` + `Num` field +
`syncEdictToQCVM`.

### Step 14: Final verification (5 min)

```bash
TMPDIR=.../.tmp CGO_ENABLED=0 go build ./...
TMPDIR=.../.tmp CGO_ENABLED=0 go test ./... -count=1 -timeout 300s
```

---

## 4. Key Lessons from Prior Attempts

1. **Batch regex/sed breaks complex write patterns**: `+=` operations,
   array element writes, and nil-check blocks get mangled. Always
   compile after each function migration.

2. **`newServerTestVM` must set `NumEdicts` correctly**: The default
   `vm.NumEdicts = 1` causes accessor writes to slot 1 to silently fail
   (bounds check in `EdictData`). Fix: `vm.NumEdicts = max(s.NumEdicts, 1)`.

3. **`fieldDef` needs `defaultEntFieldOffsets` fallback**: Tests that
   don't set up `vm.FieldDefs` for all standard fields cause
   `parseQCVMEdictFieldValue` to silently skip writing to QCVM.

4. **Host package needs `srv` type assertion**: `subs.Server` is an
   interface, not `*server.Server`. Each host function needs
   `srv, _ := subs.Server.(*server.Server)` before calling accessor methods.

5. **Mock server compatibility**: Some tests use mock servers that aren't
   `*server.Server`. The accessor methods handle `srv == nil` by falling
   back to EntVars (while it still exists). Don't add `if srv == nil { return }`
   early returns in autosave/viewframe functions.

6. **`ensureQCVMEdictStorage` must preserve data on growth**: Use `copy`
   not `make` when growing the Edicts byte slice.

---

## 5. Files to Touch (in order)

1. `entity_accessors.go` — add missing setters, remove zero-fallback
2. `entity_accessors_vec.go` — remove zero-fallback
3. `server.go` — migrate QC builtins (function-by-function)
4. `user.go` — migrate client movement
5. `user_spawn.go` — migrate client spawn
6. `server_runtime.go` — migrate client management
7. `sv_main.go` — migrate world setup
8. `rules.go` — migrate level change
9. `host/*.go` — migrate host commands (add srv declarations)
10. `debug_telemetry.go` — remove EntVars fallback
11. `savegame.go` — rewrite to use QCVM bytes
12. `savegame_text.go` — remove `&EntVars{}`
13. `edict.go` — remove reflection, simplify parse
14. `server_qc_sync.go` — simplify to no-ops, remove dead code
15. `types_entities.go` — delete EntVars struct, remove Vars field
16. `server_hooks_test.go` — fix `newServerTestVM`, `NumEdicts`
17. All remaining test files — migrate to accessors
