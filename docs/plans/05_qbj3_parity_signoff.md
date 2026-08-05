# Implementation Plan: Parity Closure & Sign-off on `qbj3_stickflip`

**Priority**: #5 (Item 6 from Roadmap)  
**Status**: Completed (GPU Memory Leak & Audio Exit Deadlock fixed; qbj3 viewpoints added to harness)  
**Target Milestone**: Phase 5  



---

## 1. Executive Summary & Architectural Context

`qbj3_stickflip` (from the Quake Brutalist Jam 3 map pack) is the project's primary community-map stress case. It contains 85,936 raw faces, 77,001 built faces, 168,142 triangles, 22,195 leaves, 750 models, 106 textures, and 1,295 lit-water faces.

While `ironwail-go` successfully mounts `qbj3`, loads its QuakeC code, and renders the map at high frame rates, visual comparisons against reference C Ironwail show two primary visual parity gaps:
1. **Lighting / Contrast Delta**: GoGPU renders the upper-center ceiling/light regions darker and with lower contrast than C Ironwail.
2. **Decal / Brush Depth Z-Fighting**: Potential depth bias flickering on coplanar brush surfaces and decal marks under certain camera angles.

The goal of this project is to investigate and resolve these rendering deltas, expand the test viewpoints in `testdata/parity/viewpoints.json`, and achieve official visual parity sign-off for `qbj3_stickflip`.

---

## 2. Existing Code Analysis & Current State

- **Parity Guide**: `docs/PARITY.md` records current `qbj3_stickflip` status and evidence workflows.
- **Lighting Shader**: `internal/renderer/world_shaders_gogpu.go` (world fragment shader, lightmap sampling, overbright multiplier `lightmap * 2.0`).
- **Decal Pipeline**: `internal/renderer/world_gogpu_decal.go` (decal mark generation and depth bias).
- **Visual Harness**: `tools/parity_screenshots/main.go` and `mise run parity-compare`.

---

## 3. Step-by-Step Implementation Sequence

### Step 5.1: Lighting & Contrast Math Audit
- **Files**: `internal/renderer/world_shaders_gogpu.go`, `world_lightmap_gogpu.go`
- **Actions**:
  - Compare GoGPU fragment shader lightmap sampling and overbright factor (`lightmap * 2.0`) against C Ironwail `gl_rmain.c` / `r_world.c`.
  - Audit gamma correction and exposure curves in `warpscale_gogpu.go` (scene composite shader) to match C Ironwail's default gamma response.

### Step 5.2: Decal & Brush Depth Bias Tuning
- **Files**: `internal/renderer/world_gogpu_decal.go`, `world_gogpu_brush_render.go`
- **Actions**:
  - Audit depth-stencil pipeline descriptor settings in `world_pipelines_gogpu.go`.
  - Adjust slope-scaled depth bias and constant depth bias on decal render pipelines to prevent z-fighting on coplanar world faces.

### Step 5.3: Expand Viewpoints Harness
- **Files**: `testdata/parity/viewpoints.json`
- **Actions**:
  - Launch C Ironwail on `qbj3_stickflip`, navigate to reported contrast delta and decal locations, and record `viewpos` coordinates (`x, y, z, pitch, yaw, roll`).
  - Add `qbj3-overview`, `qbj3-decal-depth`, `qbj3-texture-depth`, and `qbj3-moving-brush` viewpoints to `viewpoints.json`.

---

## 4. Edge Cases & C Parity Oracles

- **Environment Config**: Use `scr_viewsize 130`, `r_drawviewmodel 0`, `crosshair 0`, `fov 90` during parity captures.
- **C Parity Oracle**: C Ironwail binary `./ironwail -basedir ${QUAKE_DIR} +map qbj3_stickflip`.

---

## 5. Testing & Verification Plan

1. **Parity Harness Run**:
   ```bash
   export QUAKE_BASEDIR=/path/to/quake-data
   export IRONWAIL_BIN=./ironwail
   mise run parity-ref
   mise run parity-go
   mise run parity-compare
   ```
2. **Visual Inspection**:
   - Inspect `testdata/parity/diff/` and `testdata/parity/overlay/` to verify that pixel deltas fall within acceptable thresholds (< 5% luma delta).
