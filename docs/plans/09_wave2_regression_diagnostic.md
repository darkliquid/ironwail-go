# Wave 2 Regression Diagnostic Log & Telemetry Plan

## Symptom

Alias models (enemies, weapons, items) render as rapidly flickering, deformed
geometry that swaps between different models in the scene. The effect
intensifies with more entities. The exact same symptom occurred in the
original Wave 2 attempt (commit `4c40eb4`, reverted `e6360a2`).

## Failed Fixes (Chronological)

All fixes below were applied sequentially. Each was verified to compile and
pass tests, but **none resolved the cycling/flickering**.

| # | Fix | File(s) | Result |
|---|-----|---------|--------|
| 1 | Removed placeholder instance bind group that used uniform buffer for storage binding (usage mismatch `0x48` vs `0x80`) | `world_gogpu_alias.go` | No change |
| 2 | Added lazy creation of `instanceBindGroup` for cached models loaded before renderer init | `world_gogpu_alias.go` | No change |
| 3 | Fixed instance uniform buffer growth releasing buffers still referenced by per-model bind groups (deferred-release) | `world_gogpu_shadow_resources.go` | No change (crash gone, but cycling remained) |
| 4 | Changed `ensureAliasInstanceUniformBufferLocked` to never grow (clamp with warning) | `world_gogpu_shadow_resources.go` | Buffer overflow (16640 > 16384), models still invisible |
| 5 | Reverted to growth + recreation of all per-model bind groups on grow | `world_gogpu_shadow_resources.go` | No change |
| 6 | Changed vertex format from `VertexFormatFloat32` to `VertexFormatUint32` for vertex index attribute | `world_gogpu_alias.go`, `shaders.go` | No change |
| 7 | Fixed WGSL uniform alignment: changed `vec3<f32>` to `vec4<f32>` in `AliasInstance` struct, updated Go packing to 16-byte boundaries | `shaders.go`, `world_gogpu_alias_uniforms.go` | No change |
| 8 | Fixed `numVerts` parameter: was passing `len(poses)` (number of poses) instead of `AliasHeader.NumVerts` (vertices per pose) | `world_gogpu_alias.go` | No change |
| 9 | Regenerated WGSL normals LUT array (was filled with repeating blocks of 15 instead of 162 unique entries) | `shaders.go` | No change |
| 10 | Removed `@interpolate(flat)` from `vertexIndex` in `VertexInput` | `shaders.go` | No change |
| 11 | Merged instance params into scene uniform (group 0), eliminated dynamic offset on group 2 | `shaders.go`, `world_gogpu_alias.go`, `world_gogpu_alias_uniforms.go` | No change |

## Architecture Summary (Current State)

- **Group 0**: `AliasUniforms` (192 bytes, dynamic offset) — VP matrix, fog, alpha + instance params (frame1, frame2, blend, scale, origin, angles, numVerts)
- **Group 1**: Per-skin bind group — sampler + base texture + fullbright texture
- **Group 2**: Per-model bind group — read-only storage buffer (pose data), NO dynamic offset
- **Vertex buffer**: Per-model, 12-byte stride (uint32 vertexIndex + vec2 texCoord)
- **Pose buffer**: Per-model, `numPoses * numVerts * 4` bytes, stored as `array<u32>`
- **Pipeline**: `aliasRefVertexStride = 12`, `VertexFormatUint32` at location 0, `VertexFormatFloat32x2` at location 1

## What We Know Works

- The original CPU vertex path (before Wave 2) renders correctly
- `queue.WriteBuffer` + `queue.Submit` are serialized (no GPU race)
- Dynamic offsets on group 0 work correctly (proven by the CPU path)
- Sprite separation (own bind group layout) works correctly

## What We Don't Know

1. Whether the GPU is reading the correct instance data per draw
2. Whether the pose storage buffer contains correct data
3. Whether the vertex ref buffer (uint32 indices + texcoords) is correct
4. Whether the WGSL shader's arithmetic (decode, lerp, rotate) matches the CPU path
5. Whether the normals array is being indexed correctly
6. Whether the pose buffer indexing (`poseData[f1 * nv + vIdx]`) is correct

## Telemetry & Diagnostic Plan

### Phase 1: CPU-Side Data Validation

Add temporary debug logging to dump the actual bytes being uploaded to GPU
buffers, then verify they match what the CPU path would produce.

**1a. Pose buffer dump**
- After `queue.WriteBuffer(poseBuf, 0, poseData)`, log first 5 vertices of
  pose 0 as raw bytes and decoded floats
- Verify: `DecodeVertex(pose[0][v], scale, scaleOrigin)` matches the bytes

**1b. Instance uniform dump**
- After packing `dc.aliasBulkUniformData`, log the instance params for draw 0
  and draw 1 (frame1, frame2, blend, scale, origin, angles, numVerts)
- Verify: these match the `gpuAliasDraw` fields

**1c. Vertex ref buffer dump**
- After `queue.WriteBuffer(vtxBuf, 0, vtxData)`, log first 3 ref entries
  (vertexIndex, texCoord) as packed bytes and decoded values
- Verify: `refs[0].VertexIndex` matches the packed uint32

### Phase 2: GPU Readback Validation

Use a GPU buffer readback (copy buffer to a readback buffer, map it) to
verify the GPU sees the same data we uploaded.

**2a. Pose buffer readback**
- Create a readback buffer, copy pose buffer to it after upload, map and
  read first few u32s
- This verifies the storage buffer isn't being corrupted during upload

**2b. Instance uniform readback**
- After `queue.WriteBuffer(uniformBuffer, ...)`, copy to readback buffer,
  map and read the instance params at offset 0 and offset 256

### Phase 3: Shader Output Validation

**3a. Render a single model in isolation**
- Modify the render path to only draw the first alias entity (skip all others)
- This eliminates multi-draw interference
- If the single model renders correctly, the issue is multi-draw data sharing
- If the single model is still deformed, the issue is in the shader/buffer data

**3b. Hardcode identity transform in the shader**
- Replace all instance transform math with identity (no rotation, no translation,
  scale=1.0, blend=0, frame1=frame2=0)
- If models render correctly as static T-pose at origin, the transform math
  is wrong
- If models are still deformed, the vertex decode or pose buffer is wrong

**3c. Hardcode known-good vertex positions**
- Instead of decoding TriVertX from the storage buffer, hardcode a test
  triangle (e.g., a single triangle at fixed positions)
- If the test triangle renders correctly, the pipeline/bind group setup is
  fine and the issue is in the pose data or decode
- If the test triangle doesn't render, the pipeline/bind group setup is broken

### Phase 4: A/B Comparison with CPU Path

**4a. Dual render**
- Run both the CPU path (build vertices, upload to scratch buffer) and the
  GPU path (storage buffer + shader decode) for the same model
- Render CPU path in red, GPU path in green
- Compare: if they overlap perfectly, the paths are equivalent
- If they differ, the difference reveals the bug

**4b. Vertex-by-vertex comparison**
- For a single model, compute the CPU-interpolated vertex positions
- Compare against what the GPU shader SHOULD produce (same inputs, same math)
- Log any discrepancies

### Phase 5: Pipeline State Validation

**5a. Verify pipeline bind group count**
- The pipeline layout has 3 bind groups (0, 1, 2). Verify the pipeline was
  created with 3 bind group layouts, not 2 (leftover from pre-Wave 2)

**5b. Verify vertex buffer stride matches pipeline declaration**
- Pipeline declares `aliasRefVertexStride = 12`. Verify the actual buffer
  was created with matching data

**5c. Verify MinBindingSize doesn't exceed actual binding size**
- Group 0: `MinBindingSize = aliasSceneUniformBufferSize = 192`. The bind
  group entry `Size` must be >= 192. Verify.
- Group 2: `MinBindingSize = 4`. The bind group entry `Size` must be >= 4.
  Verify.

## Recommended Execution Order

1. **3a** (single model isolation) — quickest test to narrow the problem
2. **3b** (hardcoded identity transform) — isolates shader math vs data
3. **1a-1c** (data dumps) — verify uploaded data is correct
4. **5a-5c** (pipeline state validation) — verify GPU setup is correct
5. **4a-4b** (A/B comparison) — definitive test against known-good path
