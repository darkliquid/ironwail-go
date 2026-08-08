# QCVM Entity Storage — Architecture, Current State, and Roadmap

> **Status:** LIVE — current as of commit `6a39e29f`. Accessor dual-write is
> authoritative; `syncEdictToQCVM`/`syncEdictFromQCVM` are no-op shims kept for
> call-site compatibility. Full `EntVars` removal is planned in
> `docs/plans/10_zero_sync_migration_completion.md` and
> `docs/plans/11_phase3_entvars_removal.md`.
>
> **Supersedes** the older "Entity Sync" naming: there is no per-callback sync
> layer anymore. Where older docs describe full-edict sync calls running at
> every QC callback, that is obsolete — the corresponding symbols no longer
> exist (removed in the zero-sync work).

## The Architectural Problem

### C Ironwail: Shared Memory (No Sync)

C's `qcvm->edicts` is a malloc'd array of `edict_t`. Each `edict_t` contains
engine fields **and** an `entvars_t v` struct. QC bytecode's `OP_LOAD_*` /
`OP_STORE_*` read/write `&ed->v + field_offset` — the **exact same memory**
C engine code accesses via `ed->v.field`.

- `EDICT_TO_PROG(e)` = byte offset from `qcvm->edicts` to `e`
- `PROG_TO_EDICT(n)` = pointer arithmetic back to edict
- **No sync needed.** When QC sets `self.nextthink`, the engine sees it
  immediately. When the engine sets `ent->v.velocity`, QC sees it immediately.
- All entity fields (standard + extension) accessible by both C and QC through
  the same memory.

### Go ironwail-go: Two Views of the Same Storage

```
s.Edicts []*Edict          (Go structs, thin proxies)
  └── Edict.Num → indexes into QCVM bytes (source of truth)

s.QCVM.Edicts []byte       (flat byte array — authoritative storage)
  └── [entNum*EdictSize + 28 + fieldOfs*4]
```

- **The QCVM byte array is the source of truth** — the same model as C's
  shared memory.
- QuakeC bytecode reads/writes `QCVM.Edicts` directly via `OP_STORE_*` /
  `OP_LOAD_*`.
- Go physics, networking, and area grid access the same bytes through **typed
  accessors** (`internal/server/types/entity_accessors.go`,
  `entity_accessors_vec.go`, `internal/qc/vm_edict.go`) — `Edict.Origin(sh)`,
  `Edict.SetNextThink(sh, v)`, etc. These read/write the VM byte array
  directly, bypassing any mirror.
- **There is no per-callback sync.** `executeQCFunction`
  (`internal/server/qc_trace.go:71`) only captures/restores the VM execution
  context (`self`/`other`/depth) around each QC invocation, exactly like C's
  `PR_ExecuteProgram`. QC mutations are visible to Go immediately, and Go
  writes are visible to QC immediately.

## Typed Accessors (the current bridge)

- 171 accessor methods (float32 scalars + `[3]float32` vectors + int32
  string/entity/function refs) live in:
  - `internal/server/types/entity_accessors.go`
  - `internal/server/types/entity_accessors_vec.go`
- Each getter does a `getVM(vmp)` + `EdictSize > 28` guard, then
  `vm.EFloat/EVector/EInt`; each setter mirrors via `vm.SetE*`. The underlying
  `vm.EFloat`/`SetEFloat` (`internal/qc/vm_edict.go`) decode/encode the
  little-endian float at `fieldOfs*4` with bounds checks.
- `NumForEdict` is O(1) via the cached `Edict.Num`.
- Extension-field offsets (`state`, `speed`, `wait`, `customflags`,
  `th_checkattack`, `gravity`, …) are cached at `progs.dat` load time
  (`internal/server/qc/offsets.go`).

## Callback Dispatch Points (execution-context only — no sync)

Five places call into QC. Each sets `self`/`other`/`time` globals and invokes
`executeQCFunction` (or `executeQCFunctionLeavingGlobals` for StartFrame):

| Dispatch point | File:line | Notes |
| --- | --- | --- |
| `touchLinks` (trigger area touch) | `internal/server/world.go` | sets globals, calls executeQCFunction |
| `Impact` (direct collision touch) | `internal/server/physics/leafs.go:373` | sets globals, calls executeQCFunction |
| `PhysicsPusher` think | `internal/server/physics/leafs.go:666` | sets globals, calls executeQCFunction |
| `executeQCFunction` (generic wrapper) | `internal/server/qc_trace.go:71` | capture/restore VM context only |
| `executeQCFunctionLeavingGlobals` | `internal/server/qc_trace.go` | same, called by StartFrame |

Because storage is shared, no per-edict copy happens at any of these points.

## Lifecycle: Alloc / Free / Spawn

`AllocEdict` / `FreeEdict` (`internal/server/server_runtime.go:80-127`)
maintain the two views in lockstep:

- **Alloc**: pick a free slot (500ms reuse cooldown), `clearQCVMEdictData`
  zeroes the slot's VM bytes, then `syncEdictToQCVM` (a no-op today) is called
  for call-site compatibility.
- **Free**: `clearQCVMEdictData` again; the Go `Edict{Num}` is reset.
- `syncQCVMState` (`internal/server/server_qc_sync.go:211`) publishes globals
  + iterates live edicts; used **only** at map spawn/load boundaries
  (`savegame_server.go:179`, `server_net_main.go:401`), never per-frame.
- `SyncQCVMGlobals` (`server_qc_sync.go:230`) publishes `world`, `mapname`,
  `time`, `serverflags`, `coop`/`deathmatch` each frame before StartFrame —
  globals only, deliberately does **not** touch per-entity bytes (that would
  clobber QC's `OP_STORE_*` mutations).

## Bugs Found and Fixed (historical record)

The per-callback selective sync layer was the root cause of every trigger/
entity bug found in the qbj2 investigation. The selective pusher/non-pusher
snapshot/diff/restore machinery (capturePusherSnapshots,
syncPushersToQCVM, syncMutatedPushersFromQCVM,
captureNonPusherQCVMEdictSnapshots, syncMutatedNonPushersFromQCVM) is
**gone** — removed with the zero-sync migration (see
`docs/plans/archive/10_zero_sync_migration_completion.md` and
`docs/diagnoses/qbj2_trigger_regression.md` for the full write-up). Key fixed
bugs, kept as history:

| Bug | Summary | Fix |
| --- | --- | --- |
| 1: `executeQCFunction` missing pusher sync | QC-set pusher velocity/nextthink lost after a non-pusher think | eliminated by shared storage (no sync to lose) |
| 2: `Impact` missing pusher sync | touch callback mutations to pushers dropped | eliminated by shared storage |
| 3: Non-WALK client missing `LinkEdict` | stationary MOVETYPE_NONE clients never touched triggers | `StepFrame` links non-WALK client entities before `PlayerPostThink` |

## Known Parity Gaps / Open Items

| Item | Description | Status |
| --- | --- | --- |
| `EntVars` struct still exists | 77 bound fields in `internal/qc/vm.go:219` — a typed *view* kept for save/load + tests; not the runtime source of truth | roadmap: plans `archive/10`, `archive/11`, `archive/12` (remove `EntVars`) |
| `syncEdictToQCVM`/`syncEdictFromQCVM` no-ops + call sites | ~28 call sites in `server.go`/`server_net_main.go`/`server_runtime.go`/`server_user_commands.go`/`user_spawn.go` invoke no-ops (dead weight, but harmless; kept to ease the EntVars-removal migration) | plans 10-12 |
| `server_qc_sync.go` (406 lines) | most content is globals/lifecycle/check-client — not sync; some boilerplate removable | plans 10-12 |
| Pusher think gate | C reads `nextthink` once; Go now gates on the original value (D1) | FIXED `6a39e29f` + test |

**Do NOT reintroduce a sync layer.** If a bug looks like "QC changes lost",
the shared storage model means the divergence is an accessor that reads/writes
a different offset or a lifecycle gap (alloc/free/link), not missing sync.

## Debugging Tools

- `sv_debug_trigger` — trigger dispatch (ent#, classname, targetname,
  touch/use/think fn, `th_checkattack`, `customflags`, `state`, `wait`,
  `nextthink`). Files: `internal/server/debug_trigger.go`.
- `sv_debug_telemetry` — broader server telemetry for trigger/physics/QC
  (`internal/server/debug/telemetry.go`).
- `sv_debug_qc_trace` — QuakeC call tracing via `vm.TraceCallFunc`
  (`internal/server/qc_trace.go`).

## C Reference Functions

| C function | C file | Go contemporary |
| --- | --- | --- |
| `SV_TouchLinks` | `world.c` | `touchLinks` (`internal/server/world.go`) |
| `SV_AreaTriggerEdicts` | `world.c` | `areaTriggerEdicts` |
| `SV_LinkEdict` | `world.c` | `LinkEdict` |
| `SV_Physics` | `sv_phys.c:1226-1298` | `Physics` (`internal/server/physics/stepframe.go:22`) |
| `SV_Physics_Pusher` | `sv_phys.c:618-652` | `PhysicsPusher` (`internal/server/physics/leafs.go:666`) |
| `SV_PushMove` | `sv_phys.c:434-607` | `PushMove` (`internal/server/physics/leafs.go:433`) |
| `SV_Impact` | `sv_phys.c:155-179` | `Impact` (`internal/server/physics/leafs.go:373`) |
| `SV_RunThink` | `sv_phys.c:115-140` | `RunThink` (`internal/server/physics/leafs.go:329`) |
| `ED_Alloc` / `ED_Free` / `ED_ClearEdict` | `edict.c` | `AllocEdict` / `FreeEdict` / `clearQCVMEdictData` |

## Related Plans & Docs

- `docs/plans/10_zero_sync_migration_completion.md` — zero-sync migration status
- `docs/plans/11_phase3_entvars_removal.md` — `EntVars` removal roadmap
- `docs/plans/12_entvars_deletion_followup.md` — follow-up waves
- `docs/diagnoses/qbj2_trigger_regression.md` — the bug history that led here
- `docs/plans/25_qcvm_test_simulator.md` — standalone QuakeGo/QCVM test kit
  (builds on the same accessor layer)
