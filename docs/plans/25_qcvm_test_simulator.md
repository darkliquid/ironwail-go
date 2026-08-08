# Implementation Plan 25: QuakeGo/QCVM Test Simulator & Standalone Mod Dev Kit

**Priority**: #1 (Developer Experience for QuakeC/QuakeGo authors)
**Status**: PLANNED
**Prerequisite**: `pkg/qgo/quake`, `pkg/qgo/quakego` (separate modules —
see AGENTS.md gotcha #2), `internal/qc` VM, `internal/server` edict/QC bridges.
**Estimated effort**: 5-8 focused sessions

---

## 1. Executive Summary & Architectural Context

Writing QuakeC/QuakeGo today means: edit source → compile `progs.dat` with
`cmd/qgo` → boot the *entire engine* (`mise run run`/`-headless`) → play/fight
to reach the code path → die/iterate. That loop is slow and opaque: there is no
way to assert a mod's behavior in isolation, set breakpoints on `think`/
`touch`/`use`, inspect edict fields at a halt, or step QC statements.

ironwail-go is unusually well positioned to fix this because the QC runtime is
**pure Go**:

- `pkg/qgo/quake` defines the QCVM-facing types (`Entity`, `Vec3`, `Func`) and
  the **engine builtin stubs** (`pkg/qgo/quake/engine/`) — a test can run
  QuakeGo *functions directly in Go*, no bytecode, with Go-native assert +
  `go test` tooling.
- `internal/qc` is a complete VM over `progs.dat` with a statement interpreter
  (`exec.go`), edict byte storage (`vm_edict.go`), trace hooks
  (`TraceCallFunc`), and a compat RNG.
- `internal/server` bridges QC to the engine through typed accessors
  (`internal/server/types/entity_accessors*.go`) and telemetry
  (`internal/server/debug`), so a "simulator" can reuse the server-side
  containment logic without a full game loop.
- `internal/server/synthetic_bsp_helper.go` gives an asset-free world for
  physics-adjacent mod tests.

Goal: **a standalone dev kit** — a `qcmod test` command, a `go test`-style
runner for QuakeGo "test entities/functions", an in-VM debugger
(breakpoints, watch fields, step statements), and an interactive headerless
REPL/inspector (`qcmod sim`) usable from the terminal or embedded in the
wasm walkthrough (plan 22).

## 2. Architectural Goals

1. **Two execution modes, one test API**:
   - *In-Go*: run QuakeGo functions natively (`pkg/qgo/quake/engine` stubs +
     recording harness) — fastest, Go-assertable, no bytecode. Best for
     logic-only mods (subscripts, helper funcs, pure AI vignettes).
   - *In-VM*: run real `progs.dat` in `internal/qc` with the server bridges —
     authoritative semantics incl. edict memory, builtins, timers. Best for
     whole-feature assertions (doors, triggers, weapons).
2. **Debug support in both modes**: breakpoint on (function | statement |
   field-write), step-into/over/out, expression evaluation against edict/global
   state, watch on `nextthink`/`velocity`/`health`, call-stack printout,
   non-halting "pause" via telemetry.
3. **No engine boot**: the simulator never needs a renderer, input, audio, or a
   real map. Physics is optional (synthetic world gate).
4. **Deterministic**: fixed seed + `host_frametime`, replayable scripts; timers
   driven by the simulator clock, never wall-clock.

## 3. Step-by-Step Implementation Sequence

### Phase A — In-Go test runner (fastest win)

**Step 25.1: `cmd/qcmod` skeleton + `qcmod test`**
- **Files**: `cmd/qcmod/` (new), `pkg/qgo/quake/engine` (extend stubs),
  `mise.toml` tasks (`qcmod-test`, `qcmod-sim`).
- **Actions**: `qcmod test ./mod/...` imports QuakeGo packages as **Go test
  packages** (like `go test ./pkg/qgo/quakego`), exposes `engine` builtin
  stubs that record calls (SetViewKick, sound, traceline snapshots) into a
  `engine.Recorder`; test code writes normal Go assertions:
  ```go
  func TestDoorChain(t *testing.T) {
      world := sim.NewWorld(minimalProgs)
      door := world.Spawn("func_door", origin(0,0,0), .Speed=100)
      world.Step(0.1) // 3 engine frames
      if door.Field("nextthink") == 0 { t.Fatal("door never scheduled") }
      world.Fire(door, "touch", player)
      if !door.Moved() { ... }
  }
  ```
- **Verify**: run a real QuakeGo function (e.g. `door_fire` from quakego) and
  assert its `SUB_CalcMove` mutation of `nextthink`; `go test` cycle under 1s.

**Step 25.2: `sim.World` (edict store + clock + builtin recording)**
- **Files**: `pkg/qgo/quake/sim/` (new separate test-facing module or live under
  `cmd/qcmod/internal/sim`), docs in `docs/QGO_SIMULATOR.md`.
- **Actions**: `World` = edict registry (reuses `pkg/qgo/quake` Entity/Enums),
  a deterministic clock, engine-stub recorder, spawn/fire/step helpers, and a
  `Field(name)` accessor that reads **both** the Go struct and (when a
  progs.dat is loaded) the VM bytes — mirroring the server's dual-write
  semantics so In-Go tests don't fork from In-VM semantics.
- **Verify**: `Field("nextthink")` equal between In-Go and In-VM for the same
  sequence (bootstraps the "no-fork" goal).

### Phase B — In-VM runner + server bridges

**Step 25.3: `qcmod test --vm` (bytecode mode)**
- **Files**: `cmd/qcmod/vmrunner.go`, reuse `internal/qc` VM + 
  `internal/server/qc` hooks (imports: both are in the main module; careful
  with the separate `pkg/qgo/*` modules — use a thin API boundary that takes
  the compiled progs bytes).
- **Actions**: load `progs.dat` into `internal/qc`, run mod "test scenes"
  (spawn entities via `ED_ParseEdict`, drive `RunThink`/`touchLinks`-equivalent
  through the server's physics System with the synthetic collision world),
  assert via the same `sim.World` API. Statement stepping comes free from
  `VM` (see Step 25.5).
- **Verify**: identical outcomes to In-Go mode on a shared scene suite
  (parity between the two modes is itself a test).

**Step 25.4: builtins completeness + recording**
- **Files**: `internal/qc/builtins_*.go` audit; `cmd/qcmod` recorder.
- **Actions**: for each engine builtin a mod can call (`sound`, `traceline`,
  `find`, `checkclient`, `aim`, `random`, `SetOrigin`…), ensure the simulator
  either executes the real C-cited semantics (via `internal/server`) or records
  a deterministic stub; no builtin may silently no-op. Table driven by the C
  reference list in `pr_cmds.c`.
- **Verify**: a "builtins coverage" test iterates every builtin index used by
  `pkg/qgo/quakego` and asserts recorder/semantics exist.

### Phase C — Debugger & inspection

**Step 25.5: VM statement debugger (`internal/qc` extension, gated)**
- **Files**: `internal/qc/debugger.go` (new, no behavior change when off),
  `cmd/qcmod/debug.go`.
- **Actions**: in-VM breakpoints on `(function, statement index)` and
  field-write (`edict.field ← value`), watch expressions evaluated on edict/
  global state, step-into/over/out via the existing `Depth`/`Statements`
  machinery, `printStackTrace()` reusing `TraceCallFunc` nesting. All gated
  behind a `Debugger` struct that is nil in the normal engine path (zero cost
  when unused).
- **Verify**: a session script: break `door_fire` → step 3 statements → assert
  `self.velocity` written → continue → count statement hits exactly.

**Step 25.6: `qcmod sim` interactive REPL (headless, TTY or pipe)**
- **Files**: `cmd/qcmod/sim.go`.
- **Actions**: load a mod, spawn a scene, then accept commands in a loop:
  `step`, `frames N`, `break <fn>`, `watch <edict>.<field>`, `edicts`,
  `inspect <n>`, `trace on/off`, `set <edict>.<field> <val>` (via accessors),
  `save/load` snapshot, `script <file>` (batch). Non-Go REPL keeps parity with
  the wasm walkthrough inspector API (plan 22) so the same JSON snapshot
  functions power both the REPL and the browser panel.
- **Verify**: REPL drives the door chain scene and prints the same ordered
  hop list as H2 (plan 24) — one command output, two consumers.

### Phase D — DX polish

**Step 25.7: watchers, snapshots, and mod templates**
- **Files**: `cmd/qcmod/templates/` (door/trigger/weapon scenes),
  `docs/QGO_SIMULATOR.md` (full guide: write-a-test → run → debug loop).
- **Actions**: watch fires a message on field change; snapshots are JSON
  edict-table dumps (reuse H3 record format) loadable into the walkthrough;
  templates make a new mod's first test a 5-minute task.

**Step 25.8: wasm-walkthrough integration point**
- **Files**: `web/walkthrough/**`, `internal/qc/debugger.go` export surface.
- **Actions**: expose `window.ironwailInspector.qcBreak(fn)`, `qcStep()`,
  `qcEval(expr)` via the same JSON bridge; the walkthrough's QuakeC layer
  (plan 22.5) becomes a live debugging demo, not a static panel.

## 4. Verification & Testing Strategy
1. **Mode-parity suite**: shared scene-suite runs In-Go AND In-VM; asserts the
   two modes agree (nextthink/velocity/health sequence identical).
2. **Builtins coverage** test (25.4) green.
3. **Debugger determinism**: breakpoint session script with exact hit counts.
4. **REPL goldens**: batch scripts produce golden output (no time
   dependencies).
5. **Engine untouched**: full `mise run test` stays green; the debugger gating
   benchmark shows 0 allocs/overhead when `Debugger == nil` (plan 20-style
   `SetGlobalTypedNoAlloc` precedent in `internal/qc/setglobal_bench_test.go`).

## 5. Risks & Mitigations

| Risk | Mitigation |
| --- | --- |
| Two modes (In-Go vs In-VM) fork semantically | mode-parity suite (4.1) is a first-class test; `Field()` reads both stores |
| `internal/qc` debugger slows the engine hot path | `Debugger == nil` gate + benchmark; zero-cost when off |
| QuakeGo separate-module import cycle (AGENTS.md gotcha #2) | cmd/qcmod stays a root-module cmd; it consumes `pkg/qgo/quakego` only via compiled progs bytes, never via import |
| Builtins that genuinely need a full server (checkclient PVS) | reuse `internal/server` hooks instead of stubbing; document which builtins are server-mode only |
| REPL/wasm API divergence | single `sim.Snapshot()` JSON producer used by both (25.6 + 22.6) |

## 6. Out of Scope
- Full QuakeC language server / IDE integration (future: LSP over the same
  debugger protocol).
- Reproducible CI for mods against *every* engine version (pin progs +
  a snapshot of `internal/qc` semantics instead).
