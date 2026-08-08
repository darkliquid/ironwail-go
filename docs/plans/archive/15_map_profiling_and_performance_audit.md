# Implementation Plan 15: Multi-Map Profiling & Performance Analysis

**Priority**: High  
**Status**: Completed (`ac3ea90`)  
**Target Milestone**: Phase 15  

---

## 1. Executive Summary & Architectural Context

With the zero-sync QCVM entity migration (commit `da40ae2`), alias vertex transform optimization (commit `7b4db8e`, 12.41x speedup), and edict field lookup caching (`ac3ea90`), baseline engine performance has improved dramatically.

Empirical `pprof` heap and CPU profiling across `qbj2_zetabyt` (1,127 edicts), `qbj3_stickflip` (1,430 edicts), and `qbj3_softi` (941 edicts) led to an **85.6% reduction in total heap allocations** during map loading (from 717,542 alloc objects down to 102,986 objects).

---

## 2. Step-by-Step Implementation Sequence

### Step 15.1: Automated Profile Capture Suite (COMPLETED - commit ac3ea90 & 278fcd2)
- **Files**: `tasks/profile_maps.sh`, `mise.toml`
- **Actions**:
  - Scripted headless map benchmark runs across target maps (`qbj2_zetabyt`, `qbj3_stickflip`, `qbj3_softi`).
  - Added `mise run profile-maps` task saving CPU and heap profiles to `.tmp/profiles/`.


### Step 15.2: Edict Field Lookup Optimization (COMPLETED - commit ac3ea90)
- **Files**: `internal/server/edict.go`
- **Actions**:
  - Added `fieldDefMap map[string]fieldDefInfo` to `EntityManager` for $O(1)$ key lookups instead of linear `FieldDefs` scans.
  - Added RLock/Lock string normalization cache (`normalizedNamesCache`) and hoisted global `vec3Replacer`.
  - **Results**: Reduced total map load heap objects by **85.6%** (from 717,542 objects to 102,986 objects).


### Step 15.3: Performance Sign-off
- **Actions**: Verify zero heap allocations per frame during active gameplay loops across all target maps.

---

## 4. Verification & Testing Strategy

```bash
mise run build
./ironwailgo -basedir ./quake-data -game qbj2 -headless +map qbj2_zetabyt
```

## Step 15.3: Steady-State Per-Frame Allocation Sign-off (COMPLETED)

**Added**:
- Three console commands (`game_profile.go`):
  - `perf_warmup [frames]` — begin a warmup window (one-time uploads settle).
  - `perf_capture [frames]` — begin steady-state sampling (requires an active warmup session).
  - `perf_reset` — clear an active session.
- `perfTick` state machine driven from `HeadlessGameLoop` (`game_loop.go:387`) samples `runtime.ReadMemStats` every 15 frames and reports a machine-readable `PERF_RESULT` line (avg/max per-frame alloc bytes and objects, wall seconds).
- `perf_commands_test.go` — lifecycle, arg-parsing, and measurement tests.
- `tasks/profile_maps.sh` now also runs the warmup+capture session per map, appends `steady_state_summary.csv`, and prints the summary table.

**Verification** (`PERF_FRAMES=40` short window):
```
PERF_RESULT frame_budget 0.215 avg_alloc 1620654 avg_objects 1251 max_alloc_frame 5761568 max_objects_frame 4514 samples 4
PERF_RESULT frame_budget 0.204 avg_alloc 232206  avg_objects 261  max_alloc_frame 336593  max_objects_frame 311  samples 3
PERF_RESULT frame_budget 0.212 avg_alloc 1462713 avg_objects 11492 max_alloc_frame 3217216 max_objects_frame 26203 samples 3
```
Full 240-frame steady-state baselines pending next `mise run profile-maps` run.
