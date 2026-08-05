# Implementation Plan 20: Zero-Allocation Steady-State Frames

**Priority**: Medium
**Status**: Planned
**Target Milestone**: Phase 20

---

## 1. Executive Summary & Architectural Context

Phase 15 (multi-map profiling) built the measurement infrastructure:
`perf_warmup` / `perf_capture` / `perf_reset` console commands
(`internal/game/game_profile.go`) report per-frame allocation bytes and
object counts from a warm, post-map-load window. The first full 240-frame
baseline exposed a dominant steady-state churn source: `Server.PushMove`
(`internal/server/physics.go`) allocated two `NumEdicts`-sized slices per
pusher per frame, and the per-edict `sv_debug_push` varargs call sites
boxed their arguments into `[]any` even when the cvar was disabled. Both
were eliminated in the Phase 15 follow-up (commit `0afc2db`, reusable
`Server.pushMoveMoved`/`pushMoveFrom` buffers plus the
`SvDebugPushEnabled()` gate).

Post-fix 240-frame steady-state measurements (mean per frame):

| Map | alloc bytes | objects |
| --- | --- | --- |
| qbj2_zetabyt | ~10.2 MB | ~15.0k |
| qbj3_stickflip | ~6.9 MB | ~11.8k |
| qbj3_softi | ~8.9 MB | ~17.2k |

Zero-alloc framing is a *target*, not a hard parity requirement: map load,
signon, and one-time cache population legitimately allocate. The goal is
that the sustained gameplay window (server physics, client message parse,
renderer) reaches bounded, attributable allocations, measured against an
objective gate, with no single site dominating.

Open questions for the plan owner: whether the gate should be a hard CI
failure or a report-only trend signal, and how aggressively to pursue
GC-pressure zero (vs. "no allocation in the hot loop").

---

## 2. Technical Strategy & Architecture

1. **Attribution-first**: the current `heap`/`allocs` pprof captures are
   whole-run and dominated by map load (bsp `ReadLump`,
   `os.readFileContents`, `ApplyLitFile`, arena setup). Per-frame churn
   must instead be attributed with two heap profiles sampled on either side
   of the capture window and diffed with `go tool pprof -diff_base`.
2. **Lower-cost sampling**: `runtime.ReadMemStats` is a full STW snapshot.
   `runtime/metrics` exposes cumulative allocation counters
   (`/gc/heap/allocs:bytes`, `/gc/heap/allocs:objects`) that can be read
   without STW; switch `perfTick` sampling to those, allowing denser frames
   coverage with zero stop-the-world distortion.
3. **Hot-path audit by subsystem**, each verified with `testing.AllocsPerRun`
   benchmarks before and after:
   - QCVM accessors (`EFloat`, `EdictData`, `String`, `SetFloat`) and
     float32-to-interface boxing (`convT64` showed up in CPU profiles).
   - Client parser paths (`parseEntityUpdate` and friends) that decode with
     reflection (`encoding/binary` decoder). Replace with manual big-endian
     readers for the hot message types.
   - Collision scratch lists: `findTouchedLeafs` / `touchLinks` collect
     per-entity leaf/entity lists; reuse buffer slices on
     `CollisionSystem` like the PushMove fix did on `Server`.
4. **Regression gates**: per-subsystem `Benchmark*` with
   `testing.AllocsPerRun`, plus an end-to-end gate task that reruns
   `perf_capture` and fails if the mean exceeds the recorded baseline.

---

## 3. Step-by-Step Implementation Sequence

### Step 20.1: Heap-Diff Attribution in the Perf Harness
- **Files**: `internal/game/game_profile.go`, `tasks/profile_maps.sh`
- **Actions**:
  - `perf_capture` dumps a baseline heap profile
    (`-diff_base` operand) immediately after the start `runtime.GC()`, and
    `finishPerfCapture` dumps an end heap profile before the final reset.
  - `profile_maps.sh` runs
    `go tool pprof -diff_base=_start.pprof _end.pprof -sample_index=alloc_objects -top`
    and appends the top sites to a per-map `_steadystate_delta.txt`.
  - Switch `perfTick` from `runtime.ReadMemStats` to `runtime/metrics`
    (`/gc/heap/allocs:bytes`, `/gc/heap/allocs:objects`) and densify the
    sample interval from 15 frames to 5 (no STW cost).

### Step 20.2: QCVM Accessor and Conversion Fast Paths
- **Files**: `internal/qc/` (executor and field accessors)
- **Actions**:
  - Profile-record per-frame entry counts for `EFloat`, `EdictData`,
    `String`, and float setter paths using `pprof -diff_base` from 20.1.
  - Add typed, non-boxing hot paths (dedicated offset-based byte readers
    that return `float32`/`int32` without building interfaces).
  - Eliminate `convT64`-style boxing in per-frame QC call sites.

### Step 20.3: Reflection-Free Client Message Decoding
- **Files**: `internal/client/` parse path (`parseEntityUpdate` and the
  `SizeBuf` consumers)
- **Actions**:
  - Replace `encoding/binary` reflection decoding with manual
    `binary.BigEndian` readers for entity updates, temp entities, and
    stats messages.
  - Add `BenchmarkParseEntityUpdate` with `testing.AllocsPerRun`; target
    zero allocations on the hot line.

### Step 20.4: Collision Scratch Buffer Reuse
- **Files**: `internal/server/collision/` (`findTouchedLeafs`, hull checks)
- **Actions**: Reuse `System`-owned leaf/entity list buffers and trace
  scratch structures across calls, mirroring the PushMove pattern from
  `0afc2db`. Re-run `mise run profile-maps` after the change to confirm the
  per-frame delta attribution shrinks accordingly.

### Step 20.5: Zero-Alloc Gate Task
- **Files**: `tasks/`, `mise.toml`
- **Actions**:
  - Record the post-20.2..20.4 baseline table as the gate reference.
  - Add `mise run perf-gate` that runs `perf_capture` on the three
    benchmark maps and fails if mean bytes or objects exceed the reference
    by more than a tolerance (default 20%), or if any single site accounts
    for >10% of the window delta.
  - Add the hot-path `Benchmark*` functions to the regular test suite so
    `mise run test` catches per-frame allocation regressions.

---

## 4. Verification & Testing Strategy

```bash
mise run build
QUAKE_DIR=./quake-data mise run profile-maps   # regenerates steady_state_summary.csv + delta profiles
mise run perf-gate                              # fails on regression beyond tolerance
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./... -count=1
```

Pass criteria for the milestone:

1. Every map's 240-frame window mean drops below 1 MB/frame, and object
   counts drop below 5k/frame.
2. `go tool pprof -diff_base` between capture window endpoints shows no
   single site above 10% of the window delta (no dominating per-frame
   allocator).
3. `Benchmark*` zero-alloc assertions for the QC, client parse, and
   collision hot paths hold across the suite.
4. `TestProjectFilesUnderLineCeiling` and full `go test ./...` stay green
   (any new hot-loop file split follows the 16/17 modularisation pattern).
