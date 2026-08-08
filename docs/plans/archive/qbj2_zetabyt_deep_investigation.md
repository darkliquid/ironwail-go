# qbj2_zetabyt Deep Investigation Plan

## Context

Previous diagnostics (see `qbj2_zetabyt_diagnostic.md`) confirmed:
- All GPU resources created correctly (vertex/index/materials/lightmap/atlas buffers)
- 12000+ visible faces, 2 opaque batches, 114195 drawn indices per frame
- Render pass begins, draws, ends, submits — no errors
- Debug shader mode 4 (direct texture sampling) still produces black
- Brush entities (doors) render correctly
- Pre-existing bug (reproduced on pre-Wave 1 code)

The draw calls execute but produce no visible pixels. This means either:
1. The geometry is being clipped/discarded before rasterization
2. The render target the world draws to isn't the one being presented
3. The depth test is rejecting all fragments

## Investigation Phases

### Phase A: Verify VP Matrix and Camera Position

The world render uses `dc.renderer.ViewProjectionMatrix()` and
`dc.renderer.cameraState`. If the spawn position for this map produces a
degenerate VP matrix (e.g., camera inside a wall, or NaN values), all
geometry would be clipped.

**Steps:**
1. Add Info log in `renderWorldInternal` (after line 144) dumping:
   - `vpMatrix` (all 16 floats)
   - `camera.Origin` (x, y, z)
   - `camera.Angles` (x, y, z)
2. Compare with the C Ironwail reference:
   ```bash
   ./ironwail/ironwailgo -basedir ./quake-data -game qbj2 +map qbj2_zetabyt
   ```
   Check spawn position in console (`pos` or `status` command).
3. Verify the camera origin is inside the playable area (not at 0,0,0 or NaN).
4. Verify the VP matrix has no NaN/Inf values.

**Expected outcome:** If the VP matrix is correct, camera is at a valid
position. If VP has NaN or camera is at origin, the issue is in camera
setup for this specific map.

### Phase B: Verify Vertex Positions

The vertex buffer contains 147272 vertices. If the positions are wrong
(e.g., all zeros, NaN, or at huge coordinates), the geometry would be
clipped.

**Steps:**
1. Add Info log in the world geometry build path
   (`world_geometry_gogpu.go`, after the vertex build loop ~line 155)
   dumping:
   - First 3 vertex positions from `geom.Vertices`
   - Min/max X, Y, Z across all vertices
   - Count of NaN/Inf vertices
2. Compare vertex positions with `bspdiag face 0` output (which shows
   world-space vertex coordinates).
3. Verify positions match the BSP data.

**Expected outcome:** If vertex positions match BSP data, the geometry
is correct. If they're wrong, the issue is in the edge/vertex extraction
path.

### Phase C: Verify Scene Render Target

The world might render to an offscreen render target that isn't being
composited. The `sceneTargetActive` flag controls this.

**Steps:**
1. Add Info log in `RenderFrame` (after line 120) dumping:
   - `sceneTargetActive` (bool)
   - `dc.shouldUseSceneRenderTarget(state)` (bool)
   - `dc.enableSceneRenderTarget()` return value
2. If `sceneTargetActive` is true, add Info log around
   `compositeSceneRenderTarget` (line 208) dumping success/failure.
3. If `sceneTargetActive` is false (direct rendering), the world renders
   directly to the surface view — verify `dc.currentWGPURenderTargetView()`
   returns a non-nil view.

**Expected outcome:** If `sceneTargetActive` is true but compositing
fails, the issue is in the scene target path. If false and the surface
view is nil, the issue is in surface acquisition.

### Phase D: Verify Depth Attachment

The world render pass uses `worldDepthAttachmentForView(dc.renderer.worldDepthTextureView)`.
If the depth texture has wrong dimensions or is nil, depth testing could
reject all fragments.

**Steps:**
1. Add Info log in `renderWorldInternal` (after line 41) dumping:
   - `dc.renderer.worldDepthTextureView != nil`
   - `dc.renderer.worldDepthWidth`, `dc.renderer.worldDepthHeight`
   - Surface dimensions `w`, `h` from `dc.renderer.Size()`
2. Verify depth texture dimensions match surface dimensions.
3. Temporarily change the world pipeline's depth write to `false` and
   depth compare to `Always` to see if geometry appears (this would
   confirm depth test is the issue).

**Expected outcome:** If depth dimensions mismatch, recreate depth
texture. If disabling depth test makes geometry visible, the depth
attachment is corrupted.

### Phase E: Verify Index Buffer Content

The batched indices might point to wrong vertices, producing degenerate
triangles that produce no rasterized fragments.

**Steps:**
1. Add Info log in the batch building path
   (`appendGoGPUOpaqueWorldFaceBatches`) dumping:
   - First batch's `firstIndex` and `numIndices`
   - First 6 indices from the batched index buffer
   - Corresponding vertex positions for those indices
2. Verify the indices form non-degenerate triangles (vertices are not
   coincident).

**Expected outcome:** If indices are correct, triangles are valid. If
indices point to wrong vertices, the batching logic has a bug for this
map's face layout.

### Phase F: Verify Texture Bind Group in Render Pass

The world texture atlas bind group (`dc.renderer.worldTextures.bindGroup`)
is set as `batch.key.textureBindGroup`. If this is nil or points to the
wrong texture, sampling would produce black.

**Steps:**
1. Add Info log in the opaque batch loop (line 454) for the first batch:
   - `batch.key.textureBindGroup != nil`
   - `batch.key.lightmapBindGroup != nil`
   - `batch.key.fullbrightBindGroup != nil`
   - `dc.renderer.worldTextures != nil`
   - `dc.renderer.worldTextures.bindGroup != nil`
2. Verify the texture bind group matches `dc.renderer.worldTextures.bindGroup`.

**Expected outcome:** If bind groups are nil, the issue is in the face
classification or draw preparation. If they're correct, the binding is
fine and the issue is deeper in the GPU pipeline.

### Phase G: Renderdoc / GPU Capture

If all CPU-side data is correct, the issue might be a GPU driver bug or
a wgpu validation error that's silently swallowed (validation is off).

**Steps:**
1. Enable wgpu validation:
   - Set `validation=true` in the gogpu/wgpu instance creation
   - Or set environment variable `WGPU_DEBUG=1` or equivalent
2. Run the map and check for validation errors.
3. If possible, capture a frame with Renderdoc or similar GPU debugger.
4. Inspect the draw call's bound resources, vertex data, and shader
   output.

**Expected outcome:** Validation errors would reveal the exact GPU
state issue. Renderdoc would show whether vertices are being
rasterized at all.

## Recommended Execution Order

1. **Phase A** (VP matrix) — quickest, rules out camera issues
2. **Phase C** (scene render target) — rules out compositing failure
3. **Phase D** (depth attachment) — rules out depth culling
4. **Phase B** (vertex positions) — rules out corrupted geometry
5. **Phase F** (texture bind group) — rules out binding issues
6. **Phase E** (index buffer) — rules out degenerate triangles
7. **Phase G** (GPU capture) — definitive but requires external tools
