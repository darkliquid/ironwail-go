# Investigation Log: `qbj2_zetabyt` Window Spawn & Rendering Fixes

## Overview
This document records the investigative steps, hypotheses, implementations, and observed fallout while diagnosing the failure of `qbj2_zetabyt` to open a window, as well as subsequent rendering regressions.

---

## Chronological Attempt Log

### Attempt 1: OGG Vorbis Audio Decoding Optimization
* **Hypothesis**: The host initialization for `qbj2_zetabyt` hung because background music decoding (OGG Vorbis) was synchronously blocking the main thread during map load.
* **Changes Made**:
  - Optimized the PCM sample conversion loop in `internal/audio/ogg.go` by replacing `binary.LittleEndian.PutUint16` calls with direct byte index assignments (~300x decoding speedup).
* **Result / Fallout**:
  - Audio decoding performance improved drastically across all tracks.
  - **Unresolved**: The GUI window on `qbj2_zetabyt` still failed to spawn or unblock host initialization.

---

### Attempt 2: Vulkan Render Pass Buffer Synchronization
* **Hypothesis**: The WebGPU/Vulkan backend deadlocked during the initial frame submission because `queue.WriteBuffer(dynamicLightsBuffer, ...)` was being invoked inside an active `encoder.BeginRenderPass` during brush entity rendering.
* **Changes Made**:
  - Reordered `queue.WriteBuffer` calls in `internal/renderer/world_gogpu_brush_render.go` to run prior to `encoder.BeginRenderPass`.
  - Bypassed the `stagingBelt` for brush entity and external sky texture uploads.
* **Result / Fallout**:
  - Prevented Vulkan validation stalls during command buffer recording.
  - **Unresolved**: GUI window creation for `qbj2_zetabyt` remained unresponsive on interactive startup.

---

### Attempt 3: 2D Texture Atlas Grid Packing (`FlattenGrid`) & Lightmap Bypass
* **Hypothesis**: Large BSP maps (`qbj2_zetabyt`) overflowed the WebGPU 2D texture height limit (2048/4096px) when stacking atlas layers vertically (`FlattenVertical`). In addition, uploading 162 individual lightmap arrays for inline brush models was exhausting Vulkan memory.
* **Changes Made**:
  - Implemented `FlattenGrid()` in `internal/renderer/world_atlas_gogpu.go` to pack texture layers into a square 2D grid (`cols x rows`).
  - Rescaled material `AtlasBounds` in `internal/renderer/world_resources_gogpu.go`.
  - Changed `brushEntityLightmaps` in `internal/renderer/renderer_gogpu_worldstate.go` to unconditionally return `nil` for inline brush models.
* **Result / Fallout (Severe Visual Regressions)**:
  - **Spiderweb & Distortion Artifacts**: Lightmap V-offset rescaling was applied by walking `geom.Indices`. Fan-triangulated faces had duplicate vertex index references, causing shared vertices to have their `LightmapCoord[1]` multiplied by `vScale` multiple times (2x to 6x), creating distorted "spiderweb" black-and-white patterns across surfaces.
  - **Pitch-Black Surfaces**: Returning `nil` in `brushEntityLightmaps` deprived inline brush entities (doors, platforms) of their local lightmap pages, causing them to sample wrong or zeroed lightmaps and render pitch black.
  - **Texture Swirling / Warping**: The grid UV mapping math conflicted with shader texture sampling.
  - **Action Taken**: Completely reverted all grid flattening changes back to stable `main` (`0452d13`).

---

### Attempt 4: Black Fallback Lightmap Array (`[4]byte{0, 0, 0, 255}`)
* **Hypothesis**: Creating a black 1x1 fallback texture array for surfaces without lightmaps (`lightofs = -1`) would mimic C Ironwail's unlit surface behavior.
* **Changes Made**:
  - Created `World Black Lightmap Array` with `[4]byte{0, 0, 0, 255}` and assigned its view to `whiteLightmapBindGroup`.
* **Result / Fallout**:
  - In WebGPU, the fragment shader computes `lit = sampled.rgb * totalLight * 2.0`.
  - A black lightmap evaluates `totalLight` to `0.0`, causing `sampled.rgb * 0.0 = 0.0` (100% pitch black).
  - All unlit surfaces, external brush models, and fallback surfaces rendered completely black.

---

### Attempt 5: Lightmap Fallback Bypass for Inline Submodels (`brushEntityLightmaps = nil`)
* **Hypothesis**: The pprof stack trace showed the render thread stalled in `vkWaitSemaphores` / `Queue.WriteBuffer` during mid-frame lightmap uploads for inline submodels (>1MB allocations). Forcing `brushEntityLightmaps` to return `nil` so inline brush models fall back to `worldLightmapArray` was intended to eliminate on-the-fly GPU texture allocation.
* **Changes Made**:
  - Modified `brushEntityLightmaps` in `internal/renderer/renderer_gogpu_worldstate.go` to return `nil` for inline brush submodels.
* **Result / Fallout**:
  - **Rendering Regressions**: `qbj2_start` map rendered with dark/unlit brush entities because inline submodel faces have local lightmap page allocations that do not align with `worldLightmapArray` UVs.
  - **Window Spawn Still Blocked**: `qbj2_zetabyt` still failed to spawn a window on interactive startup.
  - **Action Taken**: Completely reverted commit `46e4f29` and restored `brushEntityLightmaps` to its original behavior.

---

## Current System State

1. **Restored Baseline (`main` - `51a27dd`)**:
   - Contains OGG Vorbis decoding optimization, material buffer capacity expansion (1024 slots), `queue.WriteTexture` skybox upload optimization, and detailed skybox diagnostic logging.
   - Standard maps (e.g. `start`, `e1m1`) render cleanly without any texture swirling, warping, spiderweb patterns, or black surfaces.
2. **Next Steps for `qbj2_zetabyt`**:
   - Investigate window creation and Wayland/Vulkan swapchain initialization specifically for `qbj2_zetabyt` from `51a27dd` without altering submodel lightmap logic.
