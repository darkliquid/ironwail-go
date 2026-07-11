# Texture Atlas Diagnostic Plan for BSP2 Large Maps

## Problem Statement

Small maps (id1 start) render correctly. Large BSP2 maps (qbj2 start) render with
incorrect textures. The key differentiators are:

- qbj2 uses BSP2 format (32-bit indices, larger lumps)
- qbj2 has far more texinfos, textures, and miptex entries than id1
- id1 fits in a single atlas layer; qbj2 requires multiple atlas layers
- Pre-atlas rendering was correct but slow; the atlas system introduced the bug

## Root Cause Hypothesis (High Confidence)

**The materials uniform buffer is hardcoded to 256 entries, but `baseMaterials` is
allocated as `textureCount + 2` with no clamping.** When a map has more than 254
textures, the following cascade occurs:

1. `uploadWorldMaterialTextures` allocates `baseMaterials` with `textureCount+2` slots
   (`world_resources_gogpu.go:486`)
2. The GPU buffer is created with capacity for exactly 256 materials
   (`world_upload_gogpu.go:283`: `Size: 256 * 32`)
3. The WGSL shader declares `array<MaterialData, 256>`
   (`world_shaders_gogpu.go:127`)
4. `queue.WriteBuffer` writes all `textureCount+2` entries into the 256-slot buffer
   — **silent buffer overflow** (WebGPU may clamp, wrap, or corrupt)
5. The shader indexes `materials[input.materialID]` where `materialID` can be
   `textureCount+1` — **out-of-bounds GPU array access**, producing garbage
   `atlasBounds`/`layer` values and hence wrong textures

This explains why id1 (which fits under 256 textures) works fine while qbj2
(which likely exceeds 256) shows widespread texture corruption.

## Diagnostic Instrumentation Plan

The goal is **non-visual diagnostics** — logging, assertions, and structured
telemetry that confirm or refute the hypothesis and pinpoint any additional issues.
All instrumentation should be gated behind a debug cvar or env var so it has zero
cost in normal operation.

### Phase 1: Material Buffer Capacity Audit

**Goal:** Confirm the buffer overflow at the Go/GPU boundary.

**Instrumentation:**

1. **Add a capacity check in `uploadWorldMaterialTextures`**
   (`world_resources_gogpu.go`):
   - After allocating `baseMaterials`, log `textureCount`, `len(baseMaterials)`,
     and the hardcoded buffer capacity (256).
   - If `len(baseMaterials) > 256`, emit a **WARN** (not just Debug) with an
     explicit message: `"material count exceeds GPU buffer capacity"`.

2. **Add a write-size check in `world_upload_gogpu.go`** (line ~475):
   - Before `queue.WriteBuffer`, compare `byteLen` against `256 * 32`.
   - If `byteLen > 256*32`, log a **WARN** with `"materials buffer overflow:
     writing N bytes into M-byte buffer"`.

3. **Add the same check in `updateWorldMaterialsBuffer`**
   (`world_material_gogpu.go:59`):
   - Compare `len(animatedMaterials)` against 256 before writing.
   - Log WARN on overflow.

4. **Add the same check in the external brush path**
   (`renderer_gogpu_worldstate.go:464`):
   - Compare `len(baseMaterials)` against 256 before writing.

**Expected output for qbj2:** WARN lines showing `textureCount > 254`, confirming
the overflow. For id1, no warnings — confirming the theory.

### Phase 2: MaterialID Range Validation

**Goal:** Confirm that vertices carry materialIDs exceeding the shader array bound.

**Instrumentation:**

1. **Add a histogram of materialIDs in `BuildModelGeometry`**
   (`world_geometry_gogpu.go`):
   - After the face loop, compute `min`, `max`, and unique count of `materialID`
     values across all faces.
   - Log at Debug level: `"materialID range"`, `"min"`, `"max"`, `"unique"`,
     `"buffer_capacity"`, 256.
   - If `max >= 256`, emit WARN: `"vertex materialID exceeds shader array bound"`.

2. **Add per-face materialID audit (gated by env var):**
   - When `IRONWAIL_DEBUG_MATERIAL_AUDIT=1`, dump the first N faces (e.g. 50)
     with: face index, texinfo index, miptex index, resolved textureIndex,
     materialID, and whether materialID >= 256.
   - This shows exactly which faces get out-of-range IDs.

**Expected output for qbj2:** `max` materialID well above 256. For id1, max < 256.

### Phase 3: Atlas Layer Distribution Telemetry

**Goal:** Verify atlas layer assignment is correct and within GPU limits.

**Instrumentation:**

1. **Add atlas layer summary in `uploadWorldMaterialTextures`**
   (`world_resources_gogpu.go`):
   - After the texture insertion loop, log a per-layer breakdown:
     - Number of textures placed in each layer
     - Total pixels used vs available per layer (utilization %)
     - Min/max texture dimensions inserted per layer
   - Log at Debug level: `"atlas layer distribution"`, with a structured
     per-layer summary.

2. **Add a layer-bounds cross-check:**
   - For each `baseMaterials[i]`, verify that `Layer < len(atlas.layers)`.
   - Verify that `AtlasBounds` values are within `[0, 1]`.
   - Log WARN on any violation: `"invalid atlas bounds for material"``,
     `"index"`, `"layer"`, `"bounds"`.

3. **Log the final atlas layer count vs WebGPU `MaxTextureArrayLayers`:**
   - Query the device limits (`adapter.Limits()`) for
     `MaxTextureArrayLayers`.
   - Log: `"atlas_layers"`, `"max_texture_array_layers"`.
   - WARN if atlas layers exceed the limit.

**Expected output:** Shows whether qbj2's atlas layer count is reasonable or
hits GPU limits, and whether any material has invalid bounds/layer.

### Phase 4: Animation Chain Integrity

**Goal:** Verify that animation remapping doesn't produce out-of-range indices
or mismatched atlas bounds.

**Instrumentation:**

1. **Add animation chain validation in `animateWorldMaterials`**
   (`world_material_gogpu.go`):
   - When `IRONWAIL_DEBUG_MATERIAL_AUDIT=1`, log each animated texture
     remapping: source index, target index, target atlas bounds, target layer.
   - If `targetIdx >= 256` (buffer capacity), WARN.
   - If `targetIdx >= len(baseMaterials)`, WARN (already guarded but log it).

2. **Log animation chain structure in `BuildTextureAnimations`**
   (`surface/surface.go`):
   - After building chains, log: number of animated texture groups, chain
     lengths, and the texture indices in each chain.
   - This confirms that animation chains reference valid texture indices.

**Expected output:** Confirms animation chains are correct or reveals
out-of-range remapping on qbj2.

### Phase 5: Structured First-Frame Material Dump

**Goal:** Provide a complete snapshot of the material table for offline analysis.

**Instrumentation:**

1. **Add a `dumpWorldMaterials` function** (new file or in `rendbg.go`):
   - When `IRONWAIL_DEBUG_MATERIAL_DUMP=1`, after atlas construction, write
     a CSV or JSON file with every material entry:
     ```
     index, texture_name, layer, atlasBounds[0], atlasBounds[1],
     atlasBounds[2], atlasBounds[3], is_animated, anim_target_idx
     ```
   - Also dump the atlas layer images as PNG files (one per layer) so the
     atlas packing can be visually inspected offline without running the game.
   - File path: `debug_materials_<mapname>_<timestamp>.csv` and
     `debug_atlas_layer_<N>.png`.

2. **Dump vertex materialID histogram:**
   - Write a second CSV with: materialID, face_count, triangle_count.
   - This shows which materialIDs are actually used and how much geometry
     references out-of-range IDs.

**Expected output:** A complete offline record showing the material table
and atlas layout, plus which materialIDs are used by how much geometry.

### Phase 6: Render-Time Material Sampling Audit

**Goal:** Verify that the shader actually receives correct material data at
draw time, catching any GPU-side corruption.

**Instrumentation:**

1. **Add a debug shader variant** (gated by a `r_debug_materials` cvar):
   - A fragment shader that outputs the materialID as a color (encoding
     `materialID % 256` into R, `materialID / 256` into G, `layer` into B).
   - This produces a visualization where wrong textures show up as wrong
     colors — without needing pixel comparison, the operator can see if
     materialIDs are systematically wrong.
   - Alternatively: output `mat.layer` as grayscale — if layers are
     corrupted, the image will show wrong brightness bands.

2. **Add a debug shader that outputs atlas UV coordinates:**
   - Output `atlasUV` as color (R=u, G=v, B=layer/maxLayers).
   - If the atlas mapping is wrong, the colors will be obviously incorrect.

**Expected output:** Debug visualizations that make the texture assignment
problem visible without needing reference images.

## Implementation Priority

| Priority | Phase | Effort | Confidence |
|----------|-------|--------|------------|
| P0 | Phase 1: Buffer capacity audit | Small | Confirms root cause |
| P0 | Phase 2: MaterialID range validation | Small | Confirms root cause |
| P1 | Phase 3: Atlas layer telemetry | Medium | Rules out layer bugs |
| P1 | Phase 5: Material table dump | Medium | Offline analysis |
| P2 | Phase 4: Animation chain integrity | Small | Rules out anim bugs |
| P2 | Phase 6: Debug shader variants | Large | Visual confirmation |

## Recommended Execution Order

1. Implement Phase 1 + Phase 2 (small, high confidence)
2. Run with qbj2 start — confirm the overflow WARNs fire
3. Run with id1 start — confirm no WARNs
4. Implement Phase 3 + Phase 5 for deeper analysis
5. If the buffer overflow is confirmed as root cause, the fix is:
   - Make the materials buffer size dynamic (sized to `textureCount + 2`)
   - Make the WGSL shader array size match (requires shader rebuild per map
     or use a storage buffer instead of uniform buffer)
   - Alternatively: use a `var<storage, read>` buffer instead of
     `var<uniform>` to avoid the 256-entry uniform buffer limit

## Key Files to Instrument

| File | Purpose |
|------|---------|
| `internal/renderer/world_resources_gogpu.go` | Atlas construction, material table |
| `internal/renderer/world_upload_gogpu.go` | GPU buffer creation, initial write |
| `internal/renderer/world_material_gogpu.go` | Per-frame animation, buffer update |
| `internal/renderer/world_geometry_gogpu.go` | MaterialID assignment per vertex |
| `internal/renderer/world_shaders_gogpu.go` | WGSL shader (array size 256) |
| `internal/renderer/renderer_gogpu_worldstate.go` | External brush material buffer |
| `internal/renderer/rendbg.go` | Existing telemetry infrastructure |
| `internal/renderer/surface/surface.go` | Animation chain construction |

## Why This Doesn't Need Pixel Comparisons

The diagnostic approach works because:

1. **The bug is in data, not in pixels** — a buffer overflow corrupts the
   material table, which the shader then reads. Logging the data directly
   reveals the corruption without needing to inspect rendered output.

2. **The control case is built in** — id1 start works, qbj2 start doesn't.
   Any diagnostic that fires on qbj2 but not id1 pinpoints the difference.

3. **The shader is deterministic** — given correct material data, it produces
   correct textures. If the material data is correct but textures are still
   wrong, the problem is elsewhere (atlas packing, UV math, layer binding).
   The telemetry narrows down which stage is wrong.

4. **Offline atlas dumps** — exporting the atlas layers as PNGs and the
   material table as CSV allows verifying the atlas packing without running
   the game or comparing screenshots.
