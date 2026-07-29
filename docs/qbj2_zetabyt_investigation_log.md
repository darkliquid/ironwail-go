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

---

### Attempt 6: Skybox Upload Reorder + Lock Release + Frame Watchdog (Partial Success)

* **Hypothesis**: The hang was caused by (A) skybox upload attempting to use
  `worldSkyExternalBindGroupLayout` before `UploadWorld` created it, (B) GPU
  operations blocking while holding `r.mu.Lock()`, and (C) redundant
  per-frame `SetExternalSkybox` calls.
* **Changes Made**:
  - Reordered `applyRuntimeRendererState` so `uploadDeferredRuntimeWorld()`
    runs before `applyRuntimeRendererSkybox()` (Phase A).
  - Rewrote `UploadPendingExternalSkybox` to release `r.mu` during
    `queue.WriteTexture` GPU operations — snapshot data under lock, unlock,
    upload, re-lock to store results (Phase C).
  - Uploaded all 6 skybox faces in a single call instead of one-per-frame
    (Phase B).
  - Guarded `SetExternalSkybox` against per-frame re-entry via
    `lastSkyboxNameKey` (Phase D).
  - Added frame stall watchdog in `OnDraw` callback (Phase E).
* **Result / Fallout**:
  - **Unresolved**: `qbj2_zetabyt` still hung after these changes. The
    skybox upload path was a red herring — the async goroutine completed
    fine, but `UploadPendingExternalSkybox` was never the blocking call.

---

### Attempt 7: Diagnose via Per-Phase Logging (Root Cause Found)

* **Method**: Added strategic `slog.Info` calls at every phase of the `OnDraw`
  callback, `RenderFrame`, `renderEntities` per-phase loop,
  `ensureBrushModelGeometry`, and `ensureBrushModelLightmaps` to pinpoint the
  exact blocking call.
* **Findings**:
  - `OnDraw` enters and calls `applyRuntimeRendererState` →
    `uploadDeferredRuntimeWorld` → `UploadWorld` completes successfully
    (geometry, textures, lightmaps, pipelines all created).
  - `applyRuntimeRendererSkybox` calls `SetExternalSkybox` (spawns async
    goroutine) — the goroutine completes and logs "ready for GPU upload".
  - `drawRuntimeRendererFrame` → `RenderFrame` → `renderWorld` completes
    and submits world render commands via `queue.Submit`.
  - `renderEntities` begins, enters phase 0 (`gogpuEntityPhaseOpaqueBrush`)
    → `renderOpaqueBrushEntitiesHAL` → processes entity 0 (submodel 27).
  - `ensureBrushModelGeometry(27)` completes (12 faces, 52 vertices).
  - `ensureBrushModelLightmaps(27, geom)` calls
    `uploadWorldLightmapArray` → `queue.WriteTexture`.
  - **HANG**: `queue.WriteTexture` never returns. The Vulkan queue is still
    processing the world render pass's `queue.Submit` from `renderWorld`.
    `queue.WriteTexture` is a synchronous operation that blocks until the
    queue drains. Since this runs on the render thread (gogpu's locked draw
    thread), the entire event loop freezes — no further `OnDraw` or
    `OnUpdate` callbacks fire.

* **Root Cause**: Lazy lightmap uploads for brush entity submodels were
  happening **during the first render frame**, after `renderWorld` had
  already submitted GPU commands. The `queue.WriteTexture` call in
  `uploadWorldLightmapArray` blocked waiting for the queue to drain,
  deadlocking the render thread. This only affected maps with many BSP
  submodels (like `qbj2_zetabyt` with 750 models) where the first entity
  processed triggered a lightmap upload. Standard maps with few/no brush
  entities didn't trigger this path.

---

### Attempt 8: Pre-load Brush Entity Resources in UploadWorld (Hang Fixed, Regressions Remain)

* **Hypothesis**: If all brush entity geometry and lightmaps are pre-loaded
  during `UploadWorld` (before any frame rendering), no GPU operations will
  occur during `renderEntities`, eliminating the `queue.WriteTexture` stall.
* **Changes Made**:
  - Added `preloadBrushModelResources(tree)` method that iterates all BSP
    submodels (index 1..N), calling `ensureBrushModelGeometry` and
    `ensureBrushModelLightmaps` for each.
  - Called `preloadBrushModelResources(tree)` at the end of `UploadWorld`
    in `world_upload_gogpu.go`.
* **Result**:
  - **Hang Fixed**: `qbj2_zetabyt` now loads and renders without freezing.
    Multiple `OnDraw` callbacks complete successfully. The external skybox
    renders. The process stays running.
  - **Regression 1 — `qbj2_start` texture swirling/warping**: Some faces in
    `qbj2_start` now show swirling/warping texture artifacts. This is likely
    because pre-loading all submodel lightmaps changes the lightmap page
    indexing or UV mapping for shared surfaces, similar to the Attempt 3
    grid-flattening regression.
  - **Regression 2 — `qbj2_zetabyt` severe performance**: The map is
    incredibly slow — audio stutters constantly and the level is barely
    rendered. Pre-loading 750 submodels' geometry and lightmaps during
    `UploadWorld` creates massive GPU memory pressure and CPU-side
    allocation overhead, likely exhausting Vulkan memory or creating too
    many small texture allocations that fragment GPU memory.
  - **Action Needed**: The pre-load approach is correct in principle but
    needs optimization: (1) only pre-load submodels that are actually
    referenced by active brush entities (not all 750 models), (2) batch
    lightmap uploads into fewer, larger textures, (3) investigate the
    texture swirling regression in `qbj2_start` which may be a separate
    lightmap page conflict issue.
