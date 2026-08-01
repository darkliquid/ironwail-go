# Implementation Plan: GPU Alias Model Animation & Renderer Performance

**Priority**: #1 (Post Zero-Sync Rendering Bottleneck)  
**Status**: Partially Completed — Wave 1 done (`24a34f3`), Wave 2 partially done (`4c40eb4`, GPU matrix transforms only; CPU keyframe interpolation remains), Wave 3 partially done (`cae8d3f`, CPU loop optimization only; GPU lightstyle uniform not yet implemented)
**Target Milestone**: Phase 1.5  

---

## 1. Executive Summary & Profiling Context

Following the total elimination of reflection-based QCVM entity synchronization (commit `da40ae2`), server-side CPU consumption dropped by **13x** (from 97.69% down to 9.83%). 

However, rendering performance on large, entity-dense maps like `qbj2_zetabyt` (1,127 active edicts) remains CPU-bound. Profiling with `pprof` shows that **Map Rendering consumes 68.13% of frame time**, with the majority spent performing CPU-side mesh animation and transform calculations in Go:

| Subsystem / Function | Flat CPU Time | Cum CPU Time | % Frame Time | Bottleneck Cause |
| :--- | :--- | :--- | :--- | :--- |
| **`alias.BuildVerticesInterpolatedInto`** | 1.51s (20.05%) | 3.21s (42.63%) | **54.32%** | CPU vertex decoding & normal interpolation per entity |
| **`alias.RotateYaw`** | 0.34s (4.52%) | 1.03s (13.68%) | Included in above | CPU `sin()`/`cos()` trigonometric matrix calculations |
| **`alias.InterpolateVertexPosition`** | 0.31s (4.12%) | 0.60s (7.97%) | Included in above | CPU float vector interpolation per vertex |
| **`compositeWorldLightmapSurfaceRGBA`** | 0.48s (6.37%) | 0.51s (6.77%) | **9.69%** | Full surface RGBA compositing for dynamic light styles |
| **`runtime.memmove`** | 0.57s (7.57%) | 0.57s (7.57%) | **7.57%** | Heap allocations & slice copying per alias entity draw |

### Primary Goal
Transition alias model keyframe interpolation, vertex transformation, and matrix calculation from CPU Go code into **WebGPU WGSL Vertex Shaders** and eliminate per-frame slice allocations.

---

## 2. Technical Strategy & Architecture

### Wave 1: Pre-allocated Render Scratch Buffers (Immediate CPU Allocation Removal)
- **Problem**: `aliasVertexBytesInto` allocates temporary Go slices and calls `runtime.memmove` continuously during frame rendering (7.57% CPU time).
- **Solution**: Replace transient slice creation with persistent, pre-allocated vertex staging buffers inside `DrawContext`.
- **Target Files**: `internal/renderer/renderer_gogpu_alias.go`, `internal/renderer/renderer_gogpu.go`

### Wave 2: GPU WGSL Vertex Shader Keyframe Blending (54.32% Speedup Target)
- **Problem**: For every active entity (monsters, pick-ups, projectiles), the CPU decodes keyframe 1 and keyframe 2, calculates `RotateYaw` matrices, and interpolates positions/normals vertex-by-vertex in Go.
- **Solution**:
  1. Upload quantized MDL keyframe vertex buffers to GPU Storage/Vertex Buffers once during model load.
  2. Pass entity transform uniforms (Translation, Rotation Yaw/Pitch/Roll, Scale, Frame 1 Index, Frame 2 Index, Lerp Fraction) per instance/entity.
  3. Perform lerping `mix(frame1_pos, frame2_pos, lerp)` and matrix transformation directly inside WGSL vertex shader `vs_alias_main`.
- **Target Files**: `internal/renderer/alias/`, `internal/renderer/world_shaders_gogpu.go`, `internal/renderer/alias_pipelines_gogpu.go`

### Wave 3: Lightmap Surface Patch Updates (9.69% Speedup Target)
- **Problem**: `compositeWorldLightmapSurfaceRGBA` re-composites full lightmap surfaces on CPU whenever a dynamic light style (flickering torch, pickup light) changes intensity.
- **Solution**:
  1. Cache static lightmap samples.
  2. Compute dynamic light style multipliers into a small 1D lightstyle uniform array passed directly to the world fragment shader.
  3. Allow fragment shader to compute `lightmap_color = static_sample * style_factors[style_id]` without re-compositing CPU RGBA pixel buffers.
- **Target Files**: `internal/renderer/world_lightmap_gogpu.go`, `internal/renderer/world_shaders_gogpu.go`

---

## 3. Phased Execution Plan

### Phase 1: Allocation & Scratch Buffer Reuse (Wave 1) — COMPLETED (commit `24a34f3`)
- [x] Create `AliasScratchBuffer` in `DrawContext` with 64KB initial size.
- [x] Refactor `buildAliasVerticesInterpolatedInto` to use `AliasScratchBuffer` instead of allocating per entity.
- [x] Verify zero heap allocations in `aliasVertexBytesInto` via `go test -bench`.

### Phase 2: GPU Keyframe Shader Migration (Wave 2) — PARTIALLY DONE (commit `4c40eb4`)
- [ ] Extend MDL model loader to generate GPU vertex buffers for all keyframes (`KeyframeBuffers`).
- [ ] Add instance uniform struct `AliasInstanceParams`: `[16]float32 matrix`, `int frame1`, `int frame2`, `float32 lerp`.
- [x] Update WGSL alias vertex shader to apply 4x4 model matrix transformation on GPU (`AliasEntityModelMatrix` helper added).
- [x] Remove `alias.RotateYaw` / `RotateAngles` CPU trig calls from hot rendering path (matrix math offloaded to GPU).
- [ ] Perform keyframe lerp `mix(frame1_pos, frame2_pos, lerp)` inside the GPU vertex shader (currently still done CPU-side via `InterpolateVertexPosition`).
- [ ] Remove `alias.InterpolateVertexPosition`, `alias.BuildVerticesInterpolatedInto` CPU loops from hot rendering path.

### Phase 3: Dynamic Lightstyle Uniform Shader Upgrade (Wave 3) — PARTIALLY DONE (commit `cae8d3f`)
- [ ] Add `@group(0) @binding(3) var<uniform> lightstyles: array<vec4<f32>, 64>;` to world fragment shader.
- [ ] Update `Server.Frame` / `Renderer.OnDraw` to update lightstyle array uniform per frame (64 floats) instead of re-compositing surface lightmap RGBA textures.
- [ ] Remove `compositeWorldLightmapSurfaceRGBA` dirty surface re-composition loop.
- [x] Optimize `compositeWorldLightmapSurfaceRGBA` with loop unrolling and precomputed scales (CPU-side optimization, not full GPU migration).

---

## 4. Verification & Testing Strategy

1. **Functional Verification**:
   ```bash
   TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/renderer/... -count=1
   ```
2. **Visual Parity**:
   ```bash
   mise run parity-all
   ```
3. **Performance Profiling Sign-off**:
   - Run `IRONWAILGO_PPROF=127.0.0.1:6060 ./ironwailgo -basedir ./quake-data -game qbj2 +map qbj2_zetabyt`.
   - Verify `alias.BuildVerticesInterpolatedInto` and `compositeWorldLightmapSurfaceRGBA` fall below 2% CPU time.
