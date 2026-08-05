# Implementation Plan 15: Multi-Map Profiling & Performance Analysis

**Priority**: High  
**Status**: Completed (`ac3ea90`)  
**Target Milestone**: Phase 15  

---

## 1. Executive Summary & Architectural Context

With the zero-sync QCVM entity migration (commit `da40ae2`), alias vertex transform optimization (commit `7b4db8e`, 12.41x speedup), and edict field lookup caching (`ac3ea90`), baseline engine performance has improved dramatically.

Empirical `pprof` heap and CPU profiling across `qbj2_zetabyt` (1,127 edicts) and `qbj3_stickflip` (1,430 edicts) led to an **85.6% reduction in total heap allocations** during map loading (from 717,542 alloc objects down to 102,986 objects).

---

## 2. Step-by-Step Implementation Sequence

### Step 15.1: Automated Profile Capture Suite (COMPLETED - commit ac3ea90)
- **Files**: `tasks/profile_maps.sh`, `mise.toml`
- **Actions**:
  - Scripted headless map benchmark runs across target maps for 1,000 frames each.
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
