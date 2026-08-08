# Implementation Plan: Texture Atlas Storage Buffer Upgrade (BSP2 Large Map Fix)

**Priority**: #2 (Item 3 from Roadmap)  
**Status**: Completed  
**Target Milestone**: Phase 2  

---

## 1. Executive Summary & Architectural Context

In WebGPU, drawing surfaces with individual per-texture `GL_Bind` calls is inefficient. `ironwail-go` solves this by packing world textures into a single 2D **texture atlas** and associating each face vertex with a `MaterialID` (uint32 at offset 44 in the 48-byte `WorldVertex` layout).

Currently, the fragment shader looks up texture bounds and atlas layer in a uniform buffer declared as:
```wgsl
@group(0) @binding(1) var<uniform> materials: array<MaterialData, 256>;
```

Because WebGPU uniform buffers have strict size limits (typically 16 KB or 64 KB) and fixed array declarations in WGSL, the buffer is capped at 256 entries. When loading large BSP2 maps — such as the `qbj2` mod's `start` map, which contains more than 254 textures — `animateWorldMaterials` attempts to write more than 256 material records, triggering a silent buffer overflow warning in `diag_atlas.go`.

The goal of this project is to upgrade the materials buffer from a fixed-size `uniform` buffer to a dynamically-sized `storage` buffer (`var<storage, read> materials: array<MaterialData>;`), removing the 256-texture cap completely.

---

## 2. Existing Code Analysis & Current State

- **WGSL Shader**: `internal/renderer/world_shaders_gogpu.go:11` defines `materials: array<MaterialData, 256>`.
- **Buffer Creation**: `internal/renderer/world_material_gogpu.go` creates uniform buffer of size `256 * 32` bytes.
- **Bind Group Layout**: `internal/renderer/world_pipelines_gogpu.go` sets binding 1 of group 0 to `BufferBindingTypeUniform`.
- **Diagnosis Doc**: `docs/diagnoses/qbj2_materials.md` documents the capacity failure on `qbj2_start`.

---

## 3. Step-by-Step Implementation Sequence

### Step 2.1: Update WGSL Shaders
- **Files**: `internal/renderer/world_shaders_gogpu.go`
- **Actions**:
  - Update `worldUniformsWGSL` binding 1 declaration from:
    `@group(0) @binding(1) var<uniform> materials: array<MaterialData, 256>;`
    to:
    `@group(0) @binding(1) var<storage, read> materials: array<MaterialData>;`

### Step 2.2: Update WebGPU Buffer Allocation & Usage Flags
- **Files**: `internal/renderer/world_material_gogpu.go`, `world_resources_gogpu.go`
- **Actions**:
  - Update `createMaterialsBuffer` to calculate buffer size dynamically based on `(textureCount + 2) * sizeof(MaterialData)`.
  - Set GPU buffer usage flags to `wgpu.BufferUsageStorage | wgpu.BufferUsageCopyDst`.
  - Remove hardcoded 256-entry check and warning logs in `diag_atlas.go`.

### Step 2.3: Update Bind Group Layout
- **Files**: `internal/renderer/world_pipelines_gogpu.go`
- **Actions**:
  - Update bind group layout entry for group 0, binding 1 from `wgpu.BufferBindingTypeUniform` to `wgpu.BufferBindingTypeReadOnlyStorage`.

### Step 2.4: Material Upload Logic
- **Files**: `internal/renderer/world_material_gogpu.go`
- **Actions**:
  - Update `animateWorldMaterials` to write the full dynamic `baseMaterials` slice to the GPU queue without truncation.

---

## 4. Edge Cases & C Parity Oracles

- **Vulkan / Adreno Alignment**: WebGPU storage buffers require 32-byte struct alignment for array elements. `MaterialData` is 32 bytes (`vec4<f32>` atlas bounds + `f32` layer + padding), which satisfies alignment.
- **C Parity Oracle**: C Ironwail uses per-texture binding (`gl_texmgr.c`), which has no fixed upper limit on texture count. The storage buffer matches C's unlimited texture count capacity.

---

## 5. Testing & Verification Plan

1. **Renderer Unit Tests**:
   ```bash
   TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/renderer/... -count=1
   ```
2. **BSP Diagnostic Inspection**:
   ```bash
   mise run build-bspdiag
   ./bspdiag info "${QUAKE_DIR}" maps/qbj2_start.bsp qbj2
   ```
   Verify that all textures (e.g. >254) load into the materials storage buffer without overflow.
