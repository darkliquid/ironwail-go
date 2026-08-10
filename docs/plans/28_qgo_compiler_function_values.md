# Implementation Plan 28: QGo Compiler Function-Value Wiring (the closure sentinel)

**Priority**: #1 (unblocks plan 22 playable no-assets demo + plan 25 qcmod mod dev kit)
**Status**: COMPLETED (2026-08-10)
**Prerequisite**: `c990111f` landed (entity fields + dependency builtins registered from
imported packages); `qgo` test suite green; full `go test ./...` green except the two
pre-existing `internal/host` savegame failures fixed in `e2b13a00` (now zero).
**Estimated effort**: 2-3 focused sessions
**Tag**: `qgo-compiler`

---

## 1. Executive Summary — what "the sentinel" actually is

The no-assets demo (plan 22) boots to a spawned synthetic world but the local client
handshake fails inside QC with `invalid function number: 13178 (fn=ClientConnect stmt=3598)`.
Before `c990111f` it failed with `OPAddress pointer out of bounds: 4378853404` (garbage
edict/field — that was the `num_entityfields=0` bug, now fixed). The remaining failure is
**function-value cells**: every `*types.Func` referenced as a value is lowered to a
placeholder `VReg`, and codegen emits that VReg's raw numeric value as the `OP_CALL`
operand (`st.A`), which the VM interprets as a **global offset** (`Globals[st.A]` → function
index). Those cells are never populated correctly — or collide with the parameter region.

The root architectural flaw: qgo's virtual-register scheme (comment in
`cmd/qgo/compiler/ir.go:11-15`):
> VReg values below `vregBase` (`0x1000`) are treated as **direct global offsets** by
> codegen; `>= vregBase` are virtual (mapped via `vregMap`, locals only).

But the **real global/data region starts at offset 82** (`OFSMsgEntity+1`, `globals.go:62`),
which **overlaps the QCVM parameter region** `parm1..parm16` at `OFSParmStart=43..91`
(`internal/qc/types.go:86-107`). My earlier attempt to give function objects real
sub-`vregBase` offsets (82+) collided with `parm` slots — e.g. `fieldOfs` consts landing on
`parm13..16` and polluting a function cell. This plan fixes the layout coherently.

## 2. Ground-truth facts (verified this session)

| Item | Value | Where |
| --- | --- | --- |
| Virtual base | `vregBase = 0x1000` (VRegs `>=` are virtual) | `ir.go:11-15` |
| `VRegInvalid` | `0xFFFFFFFF` | `ir.go:9` |
| System globals end | `OFSMsgEntity = 81` → free globals start at **82** | `types.go:86`, `globals.go` `nextOfs = OFSMsgEntity+1` |
| QCVM params | `parm1..parm16` at `OFSParmStart=43`..91 | `types.go:107` |
| Return slot | `OFSReturn = 1` (3 slots) | `types.go:61` |
| `OP_CALL` operand | `vm.callFunction(GFunction(st.A))` reads `Globals[st.A]` | `exec.go:165` |
| Call emission | `OPCall*` `A: resolveVReg(funcVReg)`; `funcVReg = resolveObject(funcObj)` | `lowering_calls_fields.go:110-124` |
| `resolveObject` | returns placeholder `allocVReg()` (virtual) for any object | `lowering_helpers.go:65-73` |
| Codegen `resolveVReg` | `vregMap` (locals) → raw `uint16(v)` fallback | `codegen.go:256-265` |
| Engine builtins | `strcat=115`, `ftos=26` | `builtins.go:108,173` |
| Entity fields now | `EntityFields=201`, `NumFieldDefs=157`, `NumFunctions=2164` after `c990111f` | compiled progs header |

## 3. The three defects to fix

### D1 — Function-object cells are uninitialized / wrong-indexed
`resolveObject(*types.Func)` fabricates a virtual VReg with no backing value. `OP_CALL`
then reads `Globals[thatVirtualNumberOfst.A]` — a wild offset (e.g. `0x1000+` OOB) or, when
lucky, a colliding real slot holding a float. Need: a real EvFunction global cell whose value
is the **function table index**, written after all functions (incl. dependency builtins) are
numbered.

### D2 — Global/param region collision at offsets 43..91
Real globals start at 82, but QCVM `parm1..parm16` occupy 43..91. A function-valued global
or any sub-`vregBase` constant in 82..91 collides with parm slots during calls. Need a
coherent layout: either (a) bump free-global start past 91 (e.g. `OFSParmStart+16*3 = 91`),
or (b) treat function cells as a distinct high region (see §4 design).
**Defense-in-depth**: regardless of the chosen base, `resolveObject`/`allocGlobalOfs` and
`GlobalAllocator` must **reject ever returning an offset inside 43..91** (the system/param
window). The safe base is the *primary* guard; an explicit bounds assertion is the *backstop*
so a future layout change cannot silently hand out a param slot.

### D3 — Function-typed `var` declarations and Func-field stores
`var X func()` (e.g. `boss_missile1`, `plat_center_touch`) and `Self.Think = player_stand1`
require the RHS function reference → index conversion. `lowerFieldStore` currently emits
`STOREP(val, ptr)` where `val` may be a virtual VReg or a float const → the field gets a
garbage value (observed `fieldOfs=0x41F00000` ≈ float 30.0). Need: function-value
conversion in both `var`-init and field-store paths.

### Known landmines (prior art — do not re-walk)
Last session's failed attempt taught three hard lessons; this plan is designed around them:

- **L1 — EvFunction cells via a `funcVRegs` codegen side-map double-mapped offsets.** The
  earlier approach allocated a `IRGlobal{Type: EvFunction}` via `resolveObject`, then tried
  to teach `CG.resolveVReg` a VReg→global map. It collided with parameter slots and was
  reverted. **Do not introduce a second mapping layer** — the sub-`vregBase` VReg IS the
  offset (option A), one source of truth.
- **L2 — `globalByName[shortName]` is ambiguous across packages.** Two packages can both
  define a function with the same Go name; keying resolve-lookups by short name silently
  returned the wrong cell (e.g. `quake.Heal` vs a target `Heal`). Key cells by
  `pkgpath.Name` at resolve time; emit the short QCName only for the function *record*.
- **L3 — `var X func()` collides with `resolveObject`'s fallback cell.** Plain
  func-typed vars were turned into `EvFunction` globals with an empty `FuncInit`, then the
  codegen fill-pass errored ("no FuncInit"). They start as **null (index 0)** and are set at
  runtime by assignments; never treat them as resolve targets.

## 4. Proposed design (option A — coherent, minimal-churn)

**A. Pre-allocate global offsets during lowering** and store them on `IRGlobal.Offset`
so `resolveObject`/lowering can return sub-`vregBase` VRegs immediately:

1. Add `Lowerer.globalAlloc *GlobalAllocator`-like cursor (`nextGlobalOfs` starting at a
   **safe base**) + `globalByName map[string]uint16`, mirroring `GlobalAllocator`'s layout.
   - **Safe base**: `OFSMsgEntity+1` is wrong (parm overlap). Use
     `OFSParmStart + 16*3` (= `91`) as the new free-global start, and make `GlobalAllocator`
     start from the same constant so codegen reuses the same offsets (named-cache
     dedupes identical names).
   - **One source of truth, not duplication**: define the base in one place
     (e.g. a compiler-shared const) imported by both the lowerer's cursor and
     `NewGlobalAllocator`. Never re-derive it independently in two spots —
     divergent copies are how the 43..91 collision entered the earlier attempt.
2. `lowerGenDecl` VAR case: assign `g.Offset = allocGlobalOfs(name, slots)`;
   `resolveObject(*types.Var)` returns `VReg(ofs)`.
3. `resolveObject(*types.Func)`:
   - if the function is registered (target func or builtin in `prog.Functions`): create an
     `IRGlobal{Type: EvFunction, FuncInit: name, Offset: allocGlobalOfs(name,1)}`,
     return `VReg(ofs)`;
   - **if unknown** (quake-package intrinsic like `Sprintf`/`MakeVec3`): do **not** create a
     cell — handled by the intrinsics in D4; any leftover reference becomes a clear
     compile error (or a 0 cell, see risk R3).
4. Codegen: after `generateFunc` pass, iterate `prog.Globals` with `FuncInit != ""`,
   resolve `functionIndexByName(FuncInit)`, `globals.SetInt(g.Offset, idx)`. Keep the
   "leave 0" fallback (already implemented and safe) for unresolved intrinsics.
5. `CG.resolveVReg`: for a VReg that is `< vregBase` and `>= globalBase`, treat as the
   pre-allocated offset (it already returns `uint16(v)` raw — now correct because
   lowering produced real offsets). Remove the `funcVRegs` indirection approach from the
   earlier exploration (it double-mapped).

**B. Func-field stores and func-var inits** (D3):
- `lowerFieldStore`: when `fieldType == EvFunction`, ensure `val` is a sub-`vregBase`
  function cell (already guaranteed by A.3); `STOREP` for `EvFunction` must use
  `OPStorePFNC` (verify `opcodeForStoreP` has the case — it does per `opcodes.go`);
  no float-conversion leak.
- `var X func()` with an initializer (`var X = SomeFunc`): the VAR-init path must store
  the function index (A.3 gives the cell VReg). With no initializer, cell = 0 (null), which
  is correct.

**C. Stable test oracle**: `cmd/qgo/testdata/builtincall` (a tiny module that calls a
`//go:qgo:builtin` function and a target function through a Func-field/var) round-trips
through `LoadProgs` + `ExecuteProgram` with no crash and correct results.

## 5. Step-by-step sequence

1. **Step 28.1 — layout cleanup**: introduce a shared constant for the free-global base
   (`globalBase = OFSParmStart + 16*3`) used by both `GlobalAllocator` and the lowerer
   (one source of truth — no independent copies). Update `NewGlobalAllocator`'s `nextOfs`,
   and add a **bounds guard**: `AllocGlobal`/`AllocAnon`/`allocGlobalOfs` must reject
   offsets inside 43..91 (system/param window) as a backstop. Run existing qgo tests (must
   stay green — globals shift but are self-consistent). **Gate**: `go test ./cmd/qgo/...`.
2. **Step 28.2 — lowering-time offsets**: add `nextGlobalOfs`/`globalByName` to
   `Lowerer` (keyed by `pkgpath.Name`, L2), assign offsets in `lowerGenDecl` + a new
   `funcGlobalCell()` helper, honoring the same bounds guard. Wire `resolveObject` for
   `*types.Var` and `*types.Func`. **Gate**: builtincall test (C) red → green.
3. **Step 28.3 — codegen fill**: post-pass writes function indices into EvFunction cells
   (extend the existing `FuncInit` loop already in `codegen.go`); keep 0-fallback for
   unknown names. **Gate**: `go test ./cmd/qgo/... ./internal/qc/...`.
4. **Step 28.4 — Func-field/var stores**: add `OPStorePFNC` coverage in
   `lowerFieldStore` + func-var init; regression test for `Self.Think = fn` pattern.
5. **Step 28.5 — quake intrinsics**: implement `quake.Sprintf` (strcat/ftos expansion,
   `strcat=115`, `ftos=26`) and `quake.MakeVec3` (3-slot vector build) in
   `lowerCallExpr` before the generic path. These were stubbed/attempted last session;
   land them now that cells are correct. **Gate**: quakego compiles; real map boots to
   `Client Active`.
6. **Step 28.6 — no-assets demo client**: the synthetic-map boot should now reach
   `Client Active`. Verify `./ironwailgo -basedir <empty> -headless` → `client active`.
   If the synthetic world's minimal entities still trip a path, add the missing
   `info_player_start` handling (likely none — the map tests prove spawn).
7. **Step 28.7 — docs**: update plan 22 status (client handshake unblocked),
   plan 25 boundary note (sentinel resolved), and this plan → COMPLETED.

## 6. Verification & testing strategy

- **Gate tests** (ranked):
  1. `cmd/qgo/testdata/builtincall` round-trip (new) — call a builtin + a target func via
     a Func field/var, execute in VM, assert result.
  2. Existing `go test ./cmd/qgo/... ./internal/qc/...` stays green.
  3. Full quakego compile produces `EntityFields=201` + correct `NumFunctions` and
     **runs**: real map boot reaches `Client Active` (regression).
  4. `TestLoadRuntimeProgramsCompilesProgsWithNoAssets` (in-memory progs) green.
  5. No-assets demo: `ironwailgo -basedir <empty> -headless` → `client active`.
  6. **Bounds guard test** (new): assert `GlobalAllocator` (and the lowering cursor) never
     hand out an offset in 43..91, and that no emitted `OP_CALL` operand resolves into the
     param region — a small compile-time/round-trip assertion that catches the D2 class
     permanently.
- **Parity oracle**: the narrative chain / interleaving tests
  (`internal/server/parity_*_test.go`, H2/H4) still pass — function-value wiring must not
  change gameplay semantics.
- **No GC/hot-path regression**: qgo is a compiler (not the engine hot path); only the
  engine's progs bytes change. Run `mise run verify`.

## 7. Risks & mitigations

| Risk | Mitigation |
| --- | --- |
| R1 — changing the global base (43→91) shifts every progs offset; existing qgo golden tests may assert old offsets | Keep shift minimal: only the *free* region start moves; system globals (0..81) unchanged. Existing tests that assert `header.Offset` values must be re-checked (compile-time constants, not runtime behavior). The bounds guard (Step 28.1) makes any future param-region regression a hard failure, not a silent miscompilation. |
| R2 — function-object cells collide across packages (e.g. two packages both define `Run`) | Key cells by **full qualified name** (`pkgpath.Name`) for lookup, but emit the QCName (Go short name) for the function record; `functionIndexByName` search by QCName. Document the dedup. |
| R3 — quake intrinsics referenced outside the intrinsic lowerers (e.g. `quake.Sprintf` stashed in a var) | Leave the cell 0 (null func) + clear compile *warning*; treat as out-of-scope until a real mod needs it. |
| R4 — `STORE PFNC` semantics differ from what the VM expects for func fields | Verify against C Quake: `self.think = funcname` stores the function index into the field via `OP_STOREP_FNC`; mimic exactly; add a field-store round-trip test. |
| R5 — regressing the two pre-existing savegame tests again | They now pass (`e2b13a00`); keep `SpawnServer` synthetic gate unchanged (only `SyntheticMapName`). |

## 8. Out of scope

- Full QuakeC/LSP language server (plan 25 future).
- Wrapping function *values* across engine↔QC boundaries beyond what quakego needs
  (e.g. passing QC functions into engine callbacks).
- Performance of the compiled progs (covers correctness; optimize later if profiling asks).

## 9. Completion notes (2026-08-10)

All gates met; `go test ./...` green (59 packages, 0 failures), qgo packages race-clean.

- **Sentinel test**: `TestCompile_BuiltinCallFunctionValues` in
  `cmd/qgo/compiler/builtincall_test.go` (fixture `cmd/qgo/testdata/builtincall`)
  round-trips through the real VM: `th.Think = Target; th.Think()` stores the
  function table index into the entity field (`EInt(thEdict, think)==Target idx`),
  the indirect call executes, `Sprintf` produces `"$qc_test quux   2.5"` via
  engine `Strcat`/`Ftos`, and `var FuncVar func()` stays a null cell.
- **Global layout**: `FreeGlobalBase = OFSParmStart + 16*3 = 91` is the single
  source of truth; `systemParamWindow` guards both allocators; the 
  `GlobalAllocator.named` system table was extracted to `systemGlobalOffsets`
  so the lowerer and codegen resolve `//qgo:`-tagged vars (`self`, `other`,
  `world`, `time`, `mapname`, `parm1..16`, `v_*`, `trace_*`) to the SAME fixed
  offsets.
- **Extra bug found & fixed while landing 28.6**: `lowerGenDecl`'s `//qgo:`
  tag scan used `else if`, so a var carrying BOTH a Doc comment (e.g. the
  `// System Globals` header) and a trailing `//qgo:self` comment never had
  its trailing tag read. `Self` (and anything like it) therefore resolved to
  an uninitialized function-local instead of the QCVM `self` global, which
  made `worldspawn`'s `PrecacheModel(Self.Model)` read an empty string and
  abort the synthetic boot. Fixed by scanning Doc and Comment independently.
- **No-assets gate (28.6)**: `ironwailgo -basedir <empty> -headless` from the
  repo root now reaches `map spawn finished` + `client active` on the
  synthetic room. The synthetic worldspawn entity gained `"model" "*0"` (real
  maps always carry it).
- **Follow-up (not blocking)**: after `client active`, the synthetic demo's
  first-frame `respawn()` issues `changelevel *0` because the QC `mapname`
  global picks up the world model string instead of `"synthetic"` — a
  no-assets spawnparms quirk to chase in plan 22 if the demo should
  stay-running rather than reload. Not a func-cell regression.
