# Implementation Plan 15: Multi-Map Profiling & Performance Analysis

**Priority**: High  
**Status**: Planned  
**Target Milestone**: Phase 15  

---

## 1. Executive Summary & Architectural Context

With the zero-sync QCVM entity migration (commit `da40ae2`) and alias vertex transform optimization (commit `7b4db8e`, 12.41x speedup), baseline engine performance has improved dramatically. However, performance on large, complex community maps with high face counts, dense lightmap atlases, and thousands of active edicts (e.g. `qbj2_zetabyt`, `qbj3_stickflip`, `ad_teotw`) requires empirical CPU, GPU, and heap profiling to identify remaining bottlenecks.

This plan details the systematic profiling, diagnostic capture, and optimization strategy across multiple problematic maps.

---

## 2. Benchmark & Profiling Methodology

1. **Target Maps**:
   - `qbj2_zetabyt` (1,127 active edicts, entity-dense physics & trace stress case)
   - `qbj3_stickflip` (85,936 faces, 106 textures, lit-water & atlas stress case)
   - `ad_v1_5` / `ad_teotw` (Arcane Dimensions, high BSP leaf & PVS tree complexity)
2. **Profiling Tools**:
   - Go `pprof` CPU & heap profiles captured via built-in console commands (`profile_cpu_start`, `profile_dump_heap`).
   - WebGPU GPU timing queries via `gogpu` diagnostic metrics.
   - Server trace telemetry (`sv_debug_telemetry 1`).

---

## 3. Step-by-Step Implementation Sequence

### Step 15.1: Automated Profile Capture Suite
- **Files**: `tasks/profile_maps.sh` (new script), `mise.toml`
- **Actions**:
  - Script headless and rendered benchmark runs across target maps for 1,000 frames each.
  - Automatically save CPU (`cpu.pprof`), heap (`heap.pprof`), and alloc (`alloc.pprof`) profiles into `.tmp/profiles/`.

### Step 15.2: Bottleneck Diagnosis & Optimization Execution
- **Target Subsystems**:
  - **BSP PVS Leaf Tracing**: Audit `bsp.LeafForPoint` and `bsp.ClusterPVS` for allocation or cache misses.
  - **Dynamic Light Clusters**: Optimize `accumulateDynamicLights` cluster assignment on CPU.
  - **Audio Channel Mixing**: Optimize Oto / Web Audio channel mixing loop for 64+ simultaneous sound channels.

### Step 15.3: Performance Sign-off
- **Actions**: Verify zero heap allocations per frame during active gameplay loops across all target maps.

---

## 4. Verification & Testing Strategy

```bash
mise run build
./ironwailgo -basedir ./quake-data -game qbj2 -headless +map qbj2_zetabyt
```
