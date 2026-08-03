# qbj2_zetabyt World Geometry Rendering Diagnostic Plan

## Symptom

World geometry textures render as black/missing on `qbj2_zetabyt`. Other
maps in the same mod (e.g. `qbj2_zetabyt2`) work fine. The sky and brush
entity textures render correctly. No errors or warnings in logs.

## Map Characteristics (from bspdiag)

- BSP version: 29 (standard, not BSP2)
- 175 textures, 4487 texinfo entries, 33247 faces, 289 models
- Atlas: 4 layers (64 MB), 2 lightmap pages (8 MB)
- NON-POW2 texture: `dopefish` 112x80 (index 119)
- External skybox: `sky13_` (loads correctly)
- 1127 edicts (exceeds standard limit of 600)

## Known Working State

- `qbj2_zetabyt` worked before the Wave 2/Wave 3 changes
- The map was specifically mentioned in the profiling data (Plan 09)
- It was the test map for the buffer overflow bug (commit `59b14be`)

## Potential Causes

### 1. Materials buffer WriteBuffer overflow
- `worldBaseMaterials` has 175 entries × 32 bytes = 5600 bytes
- The buffer is created with exactly `matBufSize = 5600`
- `updateWorldMaterialsBuffer` writes `len(animatedMaterials) * 32` bytes
- If `animatedMaterials` has more entries than `worldBaseMaterials`
  (e.g. due to animation chains), the write could overflow
- **Status**: Unlikely — `animateWorldMaterials` copies `baseMaterials`
  and only modifies entries in-place

### 2. Vertex buffer overflow (too many vertices)
- 33247 faces × avg ~4 vertices = ~130K vertices × 48 bytes = ~6 MB
- The vertex buffer is sized dynamically — should handle any size
- **Status**: Need to verify the buffer was created and written

### 3. Index buffer overflow (too many indices)
- 33247 faces × 3 indices = ~100K indices × 4 bytes = ~400 KB
- Same as vertex buffer — dynamically sized
- **Status**: Need to verify

### 4. Texture atlas layer count exceeds GPU limits
- 4 atlas layers = 4 × 2048×2048 = 64 MB
- Each layer is a separate texture view in the atlas
- The shader indexes layers via `mat.layer`
- **Status**: Need to verify layer indices are within [0, 3]

### 5. NON-POW2 texture (`dopefish` 112x80) causes atlas packing failure
- The atlas assumes power-of-2 dimensions for UV packing
- A 112x80 texture might cause incorrect UV bounds or atlas overflow
- **Status**: Need to check if `dopefish` appears in the atlas at all
  and whether its `WorldMaterialData.AtlasBounds` are correct

### 6. Lightmap page count mismatch
- 2 lightmap pages are estimated
- If the actual page count differs from what the shader expects,
  lightmap sampling could read garbage
- **Status**: Unlikely to cause black textures (lightmaps affect
  brightness, not texture presence)

### 7. World vertex buffer WriteBuffer failure (silent)
- `queue.WriteBuffer` for the vertex buffer might fail silently
- The world render pass would then read uninitialized/zeroed vertex data
- **Status**: Need to add error checking/logging

### 8. World uniform buffer bind group not set correctly
- The world bind group (`r.worldBindGroup`) includes both the uniform
  buffer and the materials buffer
- If the bind group creation fails or the materials buffer is nil,
  the shader reads garbage from `materials[]`
- **Status**: Need to verify `r.worldBindGroup` is non-nil

### 9. Deferred-release queue draining world resources
- The deferred-release queue (from Wave 1) might be releasing world
  buffers prematurely if they were grown during this map's upload
- **Status**: Need to check if any world buffers are being released
  during the first few frames

## Diagnostic Findings (Phase 1-5 executed)

### Confirmed Working
- All GPU resources created: vertex buffer (7MB), index buffer (1MB), materials buffer (5.6KB), texture atlas (4 layers, 64MB), lightmap array (2 pages)
- 29198 of 30065 faces have lightmap data (style 0, values[0]=1.0)
- 12000+ visible faces per frame via BSP visibility culling
- 2 opaque batches, 114195 drawn indices per frame
- Render pass begins, draws, ends, and submits successfully (no errors)
- All bind groups non-nil: worldBindGroup, uniformBindGroup, worldPipeline, worldTextures, materialsBuffer
- Materials buffer written successfully on initial upload and per-frame update
- Texture atlas inserts all succeed (no overflow)
- Only 1 of 175 textures is missing (texture 116, 0x0, handled by fallback)
- Brush entity (door) textures render correctly

### Confirmed Broken
- World geometry renders as completely black, even with debug shader mode 4 (sample actual textures)
- Debug shader mode 1 (materialID as color) also produces black (expected for small IDs, but mode 4 confirms texture sampling fails)
- Pre-existing bug: reproduced on pre-Wave 1 code (commit e6360a2)

### Key Insight
The render pass IS executing (draw calls issued, submit succeeds), but the GPU
produces black output. Even the debug shader that bypasses lighting and directly
samples textures produces black. This suggests the texture binding or atlas
itself isn't being read correctly by the shader for the world render pass —
even though brush entities use similar bindings successfully.

### Remaining Suspects
1. **Scene render target compositing**: If `sceneTargetActive` is true, the world renders to an offscreen target. If compositing fails or the target is wrong dimensions, the output would be black. Need to check `shouldUseSceneRenderTarget` for this map.
2. **Depth attachment mismatch**: The world render target's depth attachment might have wrong dimensions for this map's camera position, causing all fragments to fail depth test.
3. **Vertex position data corruption**: The vertex positions might be wrong (e.g., NaN or infinity), causing all triangles to be clipped. Need to verify vertex data matches expected world coordinates.
4. **Index buffer data corruption**: The batched indices might point to wrong vertices, producing degenerate triangles. Need to verify index data.
5. **Uniform buffer VP matrix**: The view-projection matrix might be wrong for this map's spawn position, clipping all geometry. Need to verify the camera position and VP matrix.

## Deep Investigation Findings (Phase A/C/D executed)

### Phase A: VP Matrix and Camera — OK
- Camera origin: `(-672, -2048, 2359)`, angles `(0, 45, 0)` — valid spawn position
- VP matrix: no NaN/Inf, m00=0.637, m05=0, m10=0, m15=1923 — looks valid
- Surface dimensions: 825×952 (stable after window settles)

### Phase C: Scene Render Target — OK (but both maps use it)
- `sceneTargetActive=true` for both `qbj2_zetabyt` AND working map `qbj2_amperz`
- `sceneRenderTarget != nil`, `sceneRenderActive = true`
- `compositeSceneRenderTarget` returns true (no "failed to composite" warning)
- Both maps use translucent liquid faces, triggering the scene render target path

### Phase D: Depth & Bind Groups — OK
- `worldDepthTextureView != nil`, dimensions match surface (825×952)
- All bind groups non-nil: worldBindGroup, uniformBindGroup, worldPipeline,
  worldTextures, worldLightmapArray, whiteLightmapBindGroup, whiteTextureBindGroup
- All GPU resources present and correctly sized

### Worldspawn Entity — Has Fog
- `fog_color = "0.0980392 0.0980392 0.0980392"` (dark gray, ~25/255)
- `fog_density = "0.012"`
- Working map `qbj2_amperz` has NO fog settings
- Fog density converted: `0.012 * 0.01595 * 0.01595 = 3.666e-08` (very small)
- At distance 1000: `exp2(-3.666e-8 * 1e6) = 0.975` — 97.5% lit, NOT black
- Debug shader mode 4 (no fog) also produces black — fog is NOT the cause

### Conclusion
All CPU-side data and GPU bindings are correct. The render pass executes
successfully (12000+ faces, 2 batches, successful submit). Both the normal
shader and debug shaders produce black output. Brush entities render
correctly. The issue is a pre-existing bug that requires GPU-level
debugging (Phase G: Renderdoc/wgpu validation) to diagnose further.

### Key Differentiator
`qbj2_zetabyt` is the only map tested that has both:
1. Fog settings in worldspawn (fog_color + fog_density)
2. 1127 edicts (exceeds standard limit of 600)

The fog settings alone shouldn't cause black output (verified math).
The high edict count might cause memory pressure or buffer size issues
that weren't caught by the per-resource size checks.

### Broader Scope: Not Just qbj2_zetabyt

The bug affects maps with **4+ atlas layers**, not just qbj2_zetabyt:

| Map | Atlas Layers | Atlas Height | Status |
|-----|-------------|---------------|--------|
| e1m1 (id1) | 1 | 2050 | ✅ Works |
| qbj2_alexunder | 1 | 2050 | ✅ Works |
| start (qbj2) | 3 | 6150 | ✅ Works |
| qbj2_zetabyt | 4 | 8200 | ❌ Black |
| qbj2_amperz | 5 | 10250 | ❌ Black |

The threshold is between 3 and 4 atlas layers. Maps with ≤3 layers
render correctly; maps with ≥4 layers render as black void.

### What's Been Verified as Correct
- Atlas texture creation succeeds (no error from device.CreateTexture)
- Atlas WriteTexture succeeds (no error from queue.WriteTexture)
- Atlas pixel data is non-black (first pixel is [255,255,255,255])
- Atlas texture, view, and bind group all created successfully
- MaxTextureDimension2D = 32768 (well above 10250)
- BytesPerRow = 8192 (multiple of 256, matches image stride)
- Vertex data: correct positions, no NaN, valid materialIDs
- Materials buffer: 177 entries, correct atlas bounds and layers
- All bind groups non-nil in render pass
- Camera VP matrix: valid, no NaN
- Render pass: begins, draws 114195 indices, ends, submits — all succeed
- Debug shader mode 4 (direct texture sampling, no lighting): still black
- Scene render target: disabling it doesn't fix the issue
- Pre-existing bug: reproduced on pre-Wave 1 code

### Next Step: Phase G (GPU Validation)

All CPU-side data and bindings are verified correct. The issue must be
in the GPU execution path. Phase G (wgpu validation / Renderdoc capture)
is required to identify the exact GPU state issue.

To enable wgpu validation, check the gogpu/wgpu instance creation code
for a validation flag or environment variable. The current logs show
`validation=false`, which means validation errors are silently
swallowed.

## ROOT CAUSE FOUND (Phase G: Vulkan Validation)

### Validation Setup

1. Installed `VK_LAYER_KHRONOS_validation` from Steam runtime at
   `/home/darkliquid/.local/share/Steam/ubuntu12_64/libVkLayer_khronos_validation.so`
2. Created manifest at
   `/home/darkliquid/.local/share/vulkan/explicit_layer.d/VkLayer_khronos_validation.json`
3. Enabled validation via `GOGPU_DEBUG=1` environment variable (gogpu
   v0.48.5 only enables validation flags when this env var is set)
4. Fixed wgpu HAL logger: `gogpu.SetLogger(slog.Default())` is now
   called after `installLogging` configures the log level, and added
   "mod" subsystem for gogpu/wgpu source paths

### Validation Errors

**Critical error — staging buffer overflow:**

```
VUID-vkCmdCopyBufferToImage-pRegions-00171:
vkCmdCopyBufferToImage(): pRegions[0] is trying to copy 67174400 bytes
plus 0 offset to/from the VkBuffer which exceeds the VkBuffer total
size of 67108864 bytes.
```

The atlas texture data is 67,174,400 bytes (2048 × 4 × 8200), but the
Vulkan driver caps the staging buffer allocation at 64 MiB
(67,108,864 bytes). The `WriteTexture` implementation in gogpu/wgpu
creates a staging buffer with `Size: len(data)` but the Vulkan driver
silently truncates the allocation to 64 MiB. The copy command then
reads past the buffer end, corrupting the atlas texture.

### Why It Only Affects Maps with 4+ Atlas Layers

| Atlas Layers | Height | Data Size | Under 64 MiB? | Renders? |
|-------------|--------|-----------|---------------|----------|
| 1 | 2050 | 16,793,600 | ✅ Yes | ✅ Yes |
| 3 | 6150 | 50,380,800 | ✅ Yes | ✅ Yes |
| 4 | 8200 | 67,174,400 | ❌ No (64.06 MiB) | ❌ Black |
| 5 | 10250 | 83,968,000 | ❌ No | ❌ Black |

Max atlas height for 64 MiB: `67,108,864 / (2048 × 4)` = 8,192 pixels.
With `rowsPerLayer = 2050`, max full layers = `8192 / 2050 = 3.99` →
only 3 full layers fit under 64 MiB.

### Other Validation Errors (pre-existing, separate issues)

- `VUID-VkWriteDescriptorSet-descriptorType-00319`: descriptor type
  mismatch (SAMPLED_IMAGE vs STORAGE_IMAGE) — likely in skybox bind group
- `VUID-vkCmdDrawIndexed-viewType-07752`: ImageView type is
  VK_IMAGE_VIEW_TYPE_2D_ARRAY but shader declares texture_2d (not
  arrayed) — skybox texture view dimension mismatch
- `VUID-vkCmdBindDescriptorSets-dynamicOffsetCount-00359`: binding
  descriptor sets with 1 dynamic offset but 0 dynamic descriptors —
  likely the world uniform bind group being bound with a dynamic offset
  when the layout doesn't have HasDynamicOffset
- `VUID-vkCmdDrawIndexed-robustBufferAccess2-07825`: index buffer
  out-of-bounds — `firstIndex(114195) + indexCount(678)` exceeds
  index buffer size (456,780 bytes / 4 = 114,195 indices, so
  114195 + 678 = 114873 > 114195)

### Fix

Option 1 (chosen): Split the atlas `WriteTexture` call into multiple
sub-region copies, each under 64 MiB. This avoids the Vulkan staging
buffer size cap without changing the atlas architecture.
