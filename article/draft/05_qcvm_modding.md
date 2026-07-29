# Chapter 5: The Modding System and the QuakeC VM

Quake's moddability is one of its most enduring legacies. The game logic —
how the shotgun works, how monsters think, how doors open, how triggers
fire — is not in the engine's C code. It is compiled into a `progs.dat`
bytecode file that the engine loads at map start and interprets via a
virtual machine. This chapter explains that VM, the Go port's challenges
with it, and the independent QuakeGo side project.

---

## What QuakeC is

QuakeC is a compiled domain-specific language. It compiles to bytecode that
runs on the QuakeC Virtual Machine (QCVM), a simple stack-based interpreter
embedded in the engine. The language has:

- **One numeric type**: `float` (32-bit). All numeric values — positions,
  velocities, health, flags, even booleans — are `float32`.
- **Strings**: indices into a string table, not heap-allocated Go strings.
- **Vectors**: `vec3` (three consecutive `float32` values in the globals
  array or entity fields).
- **Entities**: integer indices into the edict array.
- **Functions**: indices into a function table. Functions can be QC bytecode
  or builtins (native engine functions dispatched by negative index).

This design enabled a thriving modding community: modders could write
entirely new gameplay without access to the engine source. The engine
provides **builtins** — native functions like `traceline`, `spawn`,
`sound`, `setorigin`, `makevectors` — that QC code calls to interact with
the engine. The engine calls into QC at specific dispatch points:
`StartFrame`, `PlayerPreThink`, `PlayerPostThink`, `touch`, `think`,
`use`, `blocked`, and the client lifecycle functions (`PutClientInServer`,
`ClientConnect`, etc.). [#QCDocs](#qcdocs) [#Hickman](#hickman)

---

## The VM architecture

The Go QCVM lives in `internal/qc/`. The key types are:

### `DProgs` — the `progs.dat` header

```go
type DProgs struct {
    Version       int32 // Must be 6
    CRC           int32
    Statements    int32 // Offset to bytecode statements
    NumStatements int32
    GlobalDefs    int32 // Offset to global definitions
    NumGlobalDefs int32
    FieldDefs     int32 // Offset to field definitions (reflection)
    NumFieldDefs  int32
    Functions     int32 // Offset to function table
    NumFunctions  int32
    Strings       int32 // Offset to string table
    NumStrings    int32
    Globals       int32 // Offset to global variables
    NumGlobals    int32
    EntityFields  int32 // Number of entity fields
}
```

This is the on-disk format, parsed by `LoadProgs`. The C counterpart is
`dprograms_t` in `progs.h`.

### `DFunction` — a function definition

```go
type DFunction struct {
    FirstStatement int32  // Negative for builtins
    ParmStart      int32  // Offset of first parameter
    Locals         int32  // Number of local variables
    Profile        int32  // Profiling counter
    Name           int32  // String table index
    File           int32  // Source file string index
    NumParms       int32
    ParmSize       [MaxParms]byte
}
```

When `FirstStatement` is negative, the function is a builtin: the negative
value indexes into the `Builtins` array. When positive, it is a bytecode
function starting at that statement index.

### `DStatement` — a single bytecode instruction

Each statement is an opcode plus up to three operands (typically offsets
into the globals or entity fields). The opcode categories are:
- Arithmetic: `OPAddF`, `OPSubF`, `OPMulF`, `OPDivF`, etc.
- Comparison: `OPEqF`, `OPNeF`, `OPLE`, `OPLT`, `OPGE`, `OPGT`.
- Control flow: `OPIF`, `OPIFNot`, `OPGoto`, `OPReturn`, `OPDone`.
- Function calls: `OPCall0`–`OPCall8`.
- Memory: `OPLoad*`, `OPStore*`, `OPAddress`.
- Entity state: `OPState`.

### The interpreter loop

`ExecuteProgram` (`exec.go:62`) is the entry point. It dispatches builtins
directly for negative `FirstStatement`, or enters the bytecode loop for
positive. The loop (`:97`) reads `Statements[XStatement]`, switches on
the opcode, and increments `XStatement` at the bottom. The comment at
`:85` notes a subtle difference from C's pre-increment convention.

The runaway-loop limit is `0x1000000` (`exec.go:34`):

```go
const runawayLoopLimit = 0x1000000
```

If the statement count exceeds this, the VM aborts with `"runaway loop
error"`. This is a parity constant — changing it would break mods that
rely on the exact limit. [#QCDocs](#qcdocs)

### Profile counters

Each function has a `Profile` counter. The `profile` console command prints
the top 10 functions by statement count and resets the counters. This is
the engine's built-in QC profiler. [#README](#readme)

---

## Bit-perfect parity concerns

The QCVM must bit-match C's behavior for demo compatibility. Several tests
in `exec_test.go` guard invariants that would break demos or mods:

- **IEEE divide-by-zero**: `1/+0→+Inf`, `-1/+0→-Inf`, `0/+0→NaN`. QC
  programs sometimes divide by zero intentionally as an early-exit
  pattern. The Go `float32` division produces the same IEEE results as C.
  (`TestExecuteProgramDivByZeroBehaviorMatrixMatchesC`)
- **`mod` builtin**: 9 cases of integer/float modulo including sign
  combinations and zero divisors. The Go `%` operator uses truncated
  division and must match C exactly. (`TestModBuiltinBehaviorMatrixMatchesC`)
- **`random()` builtin**: the RNG sequence must be byte-for-byte identical
  for demo playback. Both the fixed and legacy formulas are tested with
  hardcoded expected sequences. (`TestRandomBuiltinMatchesCompatSequence`)
- **Runaway loop limit**: asserted to be exactly `0x1000000`.
  (`TestExecuteProgramRunawayLoopLimitConstantMatchesC`)

[#QCDocs](#qcdocs)

---

## The C shared-memory model

This is the critical architectural fact established in Chapter 1. In C, an
`edict_t` struct contains engine fields *and* an embedded `entvars_t v`
field. The QC bytecode's `OP_LOAD_*` / `OP_STORE_*` instructions read and
write `&ed->v + field_offset` — the **exact same memory** the C engine
code accesses via `ed->v.field`. The macros are pure pointer arithmetic:

```c
#define EDICT_TO_PROG(e)  (int)((byte *)e - (byte *)qcvm->edicts)
#define PROG_TO_EDICT(e)  ((edict_t *)((byte *)qcvm->edicts + e))
#define NEXT_EDICT(e)     ((edict_t *)((byte *)e + qcvm->edict_size))
```

**No sync.** When QC sets `self.nextthink`, the engine sees it
immediately. When the engine sets `ent->v.velocity`, QC sees it
immediately. All entity fields — standard and extension — are accessible
by both C and QC through the same memory. [#QCVM](#qcvm)

---

## The Go dual-storage problem

Go forbids pointer arithmetic and has a garbage collector. The engine
cannot share a raw `byte *` array with the QCVM and have Go structs point
into it. Instead, the Go port has **two separate storage representations**:

```
s.Edicts []*Edict          (Go structs)
  └── Edict.Vars *EntVars  (78 typed fields — Go's source of truth)

s.QCVM.Edicts []byte       (flat byte array, ~105+ fields)
  └── [entNum*EdictSize + 28 + fieldOfs*4]
```

- **Go physics, networking, and area grid** use `EntVars` (the typed
  struct). `ent.Vars.Origin`, `ent.Vars.Solid`, `ent.Vars.Velocity`, etc.
- **QC bytecode** reads/writes `QCVM.Edicts` (the flat byte array) via
  `vm.EFloat(entNum, fieldOfs)`, `vm.EVector(...)`, `vm.SetEFloat(...)`.
- These are **different storage**. The engine must copy data between them
  before and after every QC callback.

### What is synced

The per-edict sync (`syncEdictToQCVM` / `syncEdictFromQCVM` in
`server_qc_sync.go`) uses reflection to copy fields between `EntVars` and
the QCVM byte array. Only 78 of ~105+ QCVM fields are bound to `EntVars`
struct fields and thus synced. Extension fields (`state`, `speed`, `wait`,
`pos1`, `pos2`, `finaldest`, `think1`, `count`, `delay`, `killtarget`,
`trigger_field`, `th_checkattack`, `customflags`, `target2/3/4`) exist
**only in QCVM bytes** — Go physics and networking never read them through
the sync layer (though some are accessible via the direct-VM accessor
methods). [#QCVM](#qcvm)

### The sync layer

`syncAllToQCVM()` (`sync_all.go:47`) copies all `EntVars` fields → QCVM
bytes for every active entity before a QC callback.
`syncAllFromQCVM()` (`sync_all.go:14`) copies all QCVM bytes → `EntVars`
for every active entity after a QC callback. Both are called from
`executeQCFunction` (`qc_trace.go:69`), the **single sync point** for all
QC dispatch.

### The cost: reflection and GC pressure

The sync functions use `reflect.ValueOf(vars).Elem()` and iterate
`entFieldBinding` slices to copy each field. This is O(numEdicts ×
numFields) per QC callback, and it allocates nothing per call (the
bindings are cached), but the reflection overhead is real. The parity
doc's CPU profile of the qbj3 `qbj3_stickflip` map found that QC/server
edict sync paths — `syncEntVarsFromQC`, `syncEntVarsToQC`,
`captureNonPusherQCVMEdictSnapshots`, `syncMutatedNonPushersFromQCVM`,
`SetEFloat` — dominate the profile alongside the QC execution itself.
[#Parity](#parity)

---

## The bug chain: selective sync and the qbj2 lift

The original sync architecture was **selective**: it classified entities
as pushers (`MOVETYPE_PUSH`) or non-pushers and synced them differently
at each of five dispatch points (`touchLinks`, `Impact`, `PhysicsPusher`
think, `executeQCFunction`, `executeQCFunctionLeavingGlobals`). Each
dispatch point captured snapshots, executed QC, and selectively synced
back. This was fragile — "forgot to sync this path" was an entire bug
class.

The qbj2 mod's `start` map exposed this via a lift trigger stack:

1. Player touches `trigger_multiple` → `multi_touch` → `multi_trigger`
   → `SUB_UseTargets`.
2. Finds `func_button` → `button_use` → `button_fire` → `SUB_CalcMove`
   (button starts moving).
3. Button arrives → `button_wait` → `SUB_UseTargets` (with `delay=.5`).
4. Spawns `DelayedUse` entity (`MOVETYPE_NONE`) with `think=DelayThink`.
5. 0.5s later, `DelayedUse` think fires via `RunThink` →
   `executeQCFunction`.
6. `DelayThink` → `SUB_UseTargets` → finds `func_train` → `train_use`
   → `train_next` → `SUB_CalcMove` sets train velocity/nextthink in QCVM.
7. **BUG**: `executeQCFunction` synced non-pushers back but NOT pushers.
8. Train's Go-side velocity/nextthink remain 0 → `PhysicsPusher` never
   moves it. The lift doesn't work.

[#QCVM](#qcvm)

### The fix: unified sync

Commit `fe9e43c` replaced the selective sync with
`syncAllToQCVM`/`syncAllFromQCVM` — all entities, unconditionally, at a
single dispatch point. The fragile pusher/non-pusher classification,
`capturePusherSnapshots`, `syncPushersToQCVM`,
`syncMutatedPushersFromQCVM`, and related functions were deleted (~170
lines of dead code). Callers now just set `self`/`other`/`time` globals
and call `executeQCFunction`. [#QCVM](#qcvm)

### The accessor infrastructure

The same commit added 157 typed accessor methods to `Edict` in
`internal/server/entity_accessors.go`. These read/write the QCVM byte
array directly via `s.QCVM.EFloat(e.Num, qc.EntFieldModelIndex)`,
bypassing `EntVars` entirely. The doc comment is explicit:

> Entity field accessors provide typed read/write access to QCVM entity
> data via the byte array, eliminating the need for the EntVars sync layer.
> These methods read/write directly to `s.QCVM.Edicts[]` — the single
> source of truth — matching C Ironwail's shared-memory architecture.

~27 call sites in `server.go` have migrated to direct-VM access. But the
physics and movement hot paths still use `ent.Vars.*` — the full migration
to accessors is the remaining work (steps 3–5 of the migration plan).

### Hook isolation

A separate bug: a package-level `serverBuiltinHooks` global caused all VMs
to share hooks. A CSQC VM and a server VM would cross-contaminate each
other's callbacks. Fixed by moving hooks to per-VM storage. Tested by
`TestVMServerHooksIsolation`. [#QCDocs](#qcdocs)

---

## The long-term goal: eliminate sync entirely

The migration plan has five steps:

| Step | What | Status |
| --- | --- | --- |
| 1 | Add accessor methods to `Edict`, cache extension field offsets | **Done** (157 accessors) |
| 2 | Migrate hot-path code to accessors | **Partial** — `server.go` has ~27 direct-VM sites, physics/movement still use `EntVars` |
| 3 | Remove sync functions (delete `server_qc_sync.go`, simplify `qc_trace.go`) | **Partially done** — old selective sync removed, sync-all replaces it |
| 4 | Remove `EntVars` struct (rewrite `savegame.go` for QCVM bytes) | **Not done** |
| 5 | Simplify callback dispatch (match C exactly — no sync, just set globals and execute) | **Not done** — sync-all still runs at every callback |

When steps 3–5 complete, `EntVars`, `syncAllToQCVM`,
`syncAllFromQCVM`, and `server_qc_sync.go` will be deleted entirely. The
`executeQCFunction` wrapper will simplify to just save/restore
`self`/`other`/`time` globals and execute — matching C's zero-sync model
exactly. The accessor infrastructure is in place; the remaining work is
migrating the hot paths. [#QCVM](#qcvm)

---

## CSQC: client-side QuakeC

The QC package also supports CSQC (Client-Side QuakeC), a specialized
wrapper for client-side logic: custom HUD rendering, client-side effects,
and input handling. The `CSQC` type in `internal/qc/` loads a separate
`csprogs.dat` and provides hooks for `CallDrawHud`, `CallDrawOverlay`,
and client event dispatch.

CSQC runtime integration is currently **deferred** — the repo has CSQC
wrapper infrastructure, but host/client runtime wiring for a full CSQC
gameplay path is outside the current parity milestone. [#Parity](#parity)
The tests in `csqc_test.go` verify construction, loading, precache
registry behavior, and global sync, but no e2e CSQC gameplay path is
wired.

---

## QuakeGo: the side project (not used in the engine)

**Critical clarification:** `pkg/qgo` (the `qgo` compiler and `QuakeGo`
gameplay source) is an **independent side project** to explore porting the
QuakeC *language* to a Go-dialect variant. It is **not wired into the
engine**. The engine runs the **original** `progs.dat` bytecode compiled
from the original QuakeC sources — there are **no tests or e2e runs of the
game using a QuakeGo-compiled `progs.dat`**.

### What QuakeGo is

`qgo` is a compiler in `cmd/qgo/` that takes a Go package and emits
Quake `progs.dat` bytecode for the QCVM. QuakeGo is the Go subset and
runtime surface used by that compiler:

- `pkg/qgo/quake` — core QCVM-facing types (`Entity`, `Vec3`, `Func`) and
  engine builtin stubs (`pkg/qgo/quake/engine/`).
- `pkg/qgo/quakego` — translated gameplay package proving the model works
  against real Quake game logic.

The mental model from the QGo guide: *"QuakeGo is not 'full Go running on
Quake.' It is a deliberately narrow Go subset that maps cleanly onto
QuakeC VM concepts."* [#QGoGuide](#qgoguide) The supported types are
`float32`, `string`, `bool`, `quake.Vec3`, `*quake.Entity`, and function
values. Struct fields tagged for qgo map to entity fields. Methods are
lowered to QCVM-compatible functions. Engine calls are expressed as imports
from `quake/engine`.

### Why it is a separate module

`pkg/qgo/quake` and `pkg/qgo/quakego` are **separate Go modules** with
their own `go.mod` files. The root module does not require or replace them.
They cannot be imported by the engine. This is intentional: `pkg/qgo/quakego`
is QuakeGo source (a Go dialect compiled to QCVM `progs.dat` bytecode by
`cmd/qgo`), not regular Go library code. From the repo root, gopls/LSP may
report `BrokenImport` errors for these packages — these are expected.
[#AGENTS](#agents)

### The mechanical-port convention

`pkg/qgo/quakego` intentionally mirrors original QuakeC/progs source
structure. The QGo guide is explicit: *"Avoid cosmetic Go-idiom rewrites
(tagged switches, merged var decls) there — they drift the port from
`progs.src` and make resync harder."* `.golangci.yml` suppresses `unused`,
`SA4017`, `QF1003`, and `S1021` for that package for the same reason.
[#AGENTS](#agents) This is a *resync* concern specific to the QuakeGo side
project, not the engine — the engine itself uses the original QC bytecode.

### How to use it

```bash
go build -o qgo ./cmd/qgo
cd pkg/qgo/quakego
../../../qgo
```

Or via mise: `mise run build-progs`. The `qgo` CLI also has a
`source-order` utility for deterministic function/file ordering. But the
output `progs.dat` is never loaded by the engine — the engine loads the
original `progs.dat` from the Quake data directory.

---

## References

<a name="qcdocs"></a>[QCDocs] `docs/internal/qc.md`, ironwail-go repository.

<a name="qcvm"></a>[QCVM] `docs/QCVM_ENTITY_SYNC.md`, ironwail-go repository.

<a name="parity"></a>[Parity] `docs/PARITY.md`, ironwail-go repository.

<a name="readme"></a>[README] `README.md`, ironwail-go repository.

<a name="agents"></a>[AGENTS] `AGENTS.md`, ironwail-go repository.

<a name="qgoguide"></a>[QGoGuide] `docs/QGO_QUAKEGO_GUIDE.md`, ironwail-go
repository.

<a name="hickman"></a>[Hickman] Zachary Hickman, *"Quake Engine Analysis,"*
Northeastern University. Local copy: `article/analysisfinal.pdf`.
