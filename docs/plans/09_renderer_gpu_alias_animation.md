# Implementation Plan: GPU Alias Model Animation & Renderer Performance

**Priority**: #1 (Post Zero-Sync Rendering Bottleneck)  
**Status**: Wave 1 completed (`24a34f3`, deferred-release queue from `59b14be`). Wave 2 reverted — GPU shader attempt caused model cycling/flickering that resisted 11 fixes; reverted to CPU vertex path. Wave 3 completed — eliminated per-frame lightmap re-compositing (matches C Ironwail behavior). Sprite bind group separation retained. See Section 5 and `09_wave2_regression_diagnostic.md` for regression findings.
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

### Phase 2: GPU Keyframe Shader Migration (Wave 2) — REVERTED (second attempt)
- [ ] Extend MDL model loader to generate GPU vertex buffers for all keyframes (`KeyframeBuffers`).
- [ ] Add instance uniform struct `AliasInstanceParams`: `[16]float32 matrix`, `int frame1`, `int frame2`, `float32 lerp`.
- [x] ~~Update WGSL alias vertex shader to apply 4x4 model matrix transformation on GPU~~ (reverted — caused model cycling)
- [x] ~~Remove `alias.RotateYaw` / `RotateAngles` CPU trig calls from hot rendering path~~ (reverted — restored CPU path)
- [ ] Perform keyframe lerp `mix(frame1_pos, frame2_pos, lerp)` inside the GPU vertex shader (currently still done CPU-side via `InterpolateVertexPosition`).
- [ ] Remove `alias.InterpolateVertexPosition`, `alias.BuildVerticesInterpolatedInto` CPU loops from hot rendering path.

**Note**: A second Wave 2 attempt was made (commit `0abcfde`) using a different architecture (per-model pose storage buffers, separate instance bind group, GPU-side TriVertX decode). This also caused model cycling/flickering and was reverted. See `09_wave2_regression_diagnostic.md` for the full diagnostic log of 11 failed fixes.

### Phase 3: Dynamic Lightstyle Uniform Shader Upgrade (Wave 3) — COMPLETED
- [x] Eliminated per-frame CPU lightmap re-compositing (`setGoGPUWorldLightStyleValues` no longer calls `markDirtyLightmapPages` / `updateUploadedLightmapsLocked`). Lightmap textures are built once at level load with all styles at scale 1.0, matching C Ironwail behavior (`GL_FillSurfaceLightmap` in `r_brush.c`). Dynamic lightstyle animation is handled by the dynamic light cluster system, not by modifying lightmap textures.

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

---

## 5. Regression Findings — Wave 2 GPU Model Matrix (commit `4c40eb4`, reverted `e6360a2`)

### What was attempted

Commit `4c40eb4` offloaded the alias model matrix (RotateYaw/RotateAngles + origin + scale)
from CPU vertex building into the GPU WGSL vertex shader. This involved:

1. Adding a `modelMatrix: mat4x4<f32>` field to the `AliasUniforms` WGSL struct,
   increasing `aliasSceneUniformBufferSize` from 96 to 160 bytes.
2. Creating a new `BuildVerticesModelSpaceInto` function that outputs model-space
   vertices (no CPU rotation, translation, or scale).
3. Creating an `AliasEntityModelMatrix` helper that computes `trans * (rot * scale)`.
4. Updating the WGSL vertex shader to apply `uniforms.modelMatrix * position`.
5. Updating `appendAliasSceneUniformBytes` to write the model matrix at bytes [64:128].

### Symptoms

- **Model cycling/swapping**: All visible alias models (enemies, weapons, grenades,
  items, gibs) rapidly cycled between each other's geometry and textures. A grenade
  would briefly appear as an enemy model, a weapon would cycle to armor, etc.
- **Position offset**: Models were occasionally rendered at wrong positions during
  the cycling.
- **Invisibility**: Some models disappeared entirely on certain frames.
- **Worse with more entities**: The effect intensified as more alias entities were
  on screen, suggesting a scaling buffer or indexing issue.
- **Demo playback**: Most visible during attract-mode demo playback (`demo1.dem`)
  when multiple entities are active simultaneously.

### Investigation and fixes attempted

1. **Dead CPU vertex loop** (`d074ecc`, reverted): The pre-build loop still called
   `BuildVerticesInterpolatedInto` (full CPU transform) for every entity every frame.
   Its result was discarded but wasted CPU time. Removing it did not fix the cycling.

2. **Uniform buffer race condition** (`56bbf36`, reverted): Hypothesized that multiple
   render passes (opaque, translucent, viewmodel) sharing the same GPU uniform/vertex
   buffers were overwriting each other's data. Added accumulation logic to use different
   buffer regions per pass. Did not fix the cycling — WebGPU queue ordering prevents
   this kind of race.

3. **Buffer-use-after-free** (`c7bdcff`/`dbbd119`): `ensureAliasScratchBufferLocked` and
   `ensureAliasUniformBufferLocked` released and recreated GPU buffers when they needed
   to grow, potentially while the GPU was still reading from them. Fixed by
   pre-allocating generous minimum sizes and never releasing mid-frame. Did not fix
   the cycling.

4. **Sprite uniform size mismatch** (`2ede1d4`, kept): The sprite render path reuses
   the alias bind group layout. After `aliasSceneUniformBufferSize` increased to 160,
   `SpriteUniformBufferSize` was still 96, causing a WebGPU validation error. Fixed
   by adding a `modelMatrix` field to `SpriteUniforms` and padding to 160 bytes.
   This fix was correct and is retained.

### Root cause

The exact root cause was not definitively identified. The cycling stopped immediately
when the GPU transform commit was fully reverted (`e6360a2`), returning to the
CPU-transformed vertex path where all vertex transforms are baked into the vertex data
and the uniform only contains view-projection, camera origin, fog, and alpha.

Likely contributing factors:
- The `AliasUniforms` WGSL struct at 160 bytes with `worldUniformAlign = 256` and
  `HasDynamicOffset = true` may have caused the GPU to read the wrong 160-byte region
  of the uniform buffer, picking up a different entity's model matrix.
- The shared alias bind group layout (used by both alias entities and sprites) with
  `MinBindingSize` changing from 96 to 160 may have caused WebGPU validation or
  binding behavior differences across passes.
- The dead first loop's CPU-transformed vertex data (world-space) and the second loop's
  model-space vertex data may have been inconsistent in subtle ways (e.g., different
  vertex counts on edge cases), causing the `prepared` early-return check to behave
  differently than expected.

### Lessons for future Wave 2 attempts

1. **Do not add the model matrix to the shared per-draw uniform buffer.** Instead,
   upload keyframe vertex data as a per-model GPU storage buffer and pass
   `frame1Index`/`frame2Index`/`blend`/`scale`/`scaleOrigin` as small per-draw
   uniforms. The shader decodes TriVertX on the GPU and performs the lerp+transform
   entirely in WGSL. This eliminates the model matrix from the uniform entirely.

2. **Give sprites their own bind group layout** separate from the alias layout, so
   changes to one don't affect the other.

3. **Remove the dead pre-build loop** before starting Wave 2 work. It confuses the
   codebase and wastes CPU cycles.

4. **Test with demo playback early and often.** The cycling only manifested during
   demo playback with multiple entities — unit tests and single-entity tests did not
   reproduce it.

5. **Pre-allocate GPU buffers generously** and never release/recreate them mid-frame.
   The buffer-use-after-free fix (`dbbd119`) is retained even after the revert.
