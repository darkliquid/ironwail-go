# Implementation Plan 16: Package Modularisation & Monolithic Package Split

**Priority**: High  
**Status**: Planned  
**Target Milestone**: Phase 16  

---

## 1. Executive Summary & Architectural Context

While initial modularisation phases extracted key sub-packages (`internal/renderer/world`, `internal/renderer/alias`, `internal/renderer/sky`, `internal/server/physics`, `internal/server/collision`), several core packages remain monolithic and large. In particular:
- `internal/renderer/` contains **40+ Go source files** in a single directory level.
- `internal/game/` contains **30+ files** coupling init, input, UI, audio, and render loops.
- `internal/server/` contains **25+ files** mixing edict management, network buffer packing, and QuakeC hooks.

To comply with the project's strict architecture guidelines and the 1,000-line per-file ceiling rule (`TestProjectFilesUnderLineCeiling`), this plan breaks down these large monolithic packages into clean, single-responsibility sub-packages.

---

## 2. Technical Strategy & Architecture

1. **`internal/renderer/` Sub-package Decomposition**:
   - Extract `internal/renderer/pipeline/` for WebGPU pipeline descriptors and shader compilation.
   - Extract `internal/renderer/lightmap/` for lightmap atlas generation, page allocation, and sidecar `.lit` parsing.
   - Extract `internal/renderer/decal/` for mark surface clipping and GPU depth bias management.
   - Extract `internal/renderer/particle/` for particle system rendering.
2. **`internal/game/` Sub-package Decomposition**:
   - Extract `internal/game/runtime/` for frame timing, tick coordination, and screenshot capture.
   - Extract `internal/game/ui/` for console sliding, HUD scaling, and plaque rendering.
3. **`internal/server/` Sub-package Decomposition**:
   - Extract `internal/server/edict/` for edict memory pool allocation, field indexing, and dictionary lookups.
   - Extract `internal/server/state/` for server signon state and client connection management.

---

## 3. Step-by-Step Implementation Sequence

### Step 16.1: Extract `internal/renderer/lightmap`
- **Files**: `internal/renderer/world_lightmap_gogpu.go` -> `internal/renderer/lightmap/`
- **Actions**: Create package `lightmap` containing atlas page allocation, sample compositing, and `.lit` parsing.
- **Status**: ✅ **DONE** in Plan 16b (commit `6df0832` + this plan's implementation). The pure helpers
  (`CompositePageRGBA`, `DirtyBounds`, `ExtractRegionRGBA`, `MarkDirtyLightmapPages`,
  `ClearDirtyFlags`, `RecompositeDirtySurfaces`, `StackPages`, `DefaultStyleValues`,
  `LightstyleScale`, `StylesChanged`, `AnyStyleChanged`) moved into `internal/renderer/lightmap`;
  the root keeps thin wrappers and the wgpu texture/bind-group creation. Pinned tests live in
  `lightmap/lightmap_test.go`, copied from `world_aux_test.go` L210-321.
- **Note**: `.lit` parsing was NOT moved (it lives with BSP loading); only CG compositing math moved.

### Step 16.2: Extract `internal/renderer/decal`
- **Files**: `internal/renderer/world_gogpu_decal.go` -> `internal/renderer/decal/`
- **Actions**: Create package `decal` containing mark polygon generation and GPU vertex staging.
- **Status**: ✅ **DONE** in Plan 16b. The shared math (`AtlasData`, `Smoothstep`, `PrepareDraws`,
  `BuildQuad`, `BuildBasis`, `Normalize3`, `Add3/Scale3/Cross3`) moved into `internal/renderer/decal`;
  `DecalMarkSystem` became a thin root wrapper over `decal.System`; `MarkEntity` is the minimal
  geometry interface satisfied by root `DecalMarkEntity`. The wgpu resource/HAL methods
  (`ensureDecalResourcesLocked`, `renderDecalMarksHAL`) stay in the root (Pattern B). Pinned tests in
  `decal/decal_test.go`.

### Step 16.3: Extract `internal/game/ui`
- **Files**: `internal/game/runtime_ui.go`, `game_hud.go` -> `internal/game/ui/`
- **Actions**: Create package `ui` containing UI scaling calculation and HUD layout.
- **Status**: ⏸ **DEFERRED to 16+2** (Plan 16b §8/§10). Every `(*Game)` method in `runtime_ui.go`
  reads/writes `g.*` state; a real `game/ui` package needs a `Game` facade + exported accessors.
  This release only did file-level domain grouping within `package game` (already present:
  `runtime_ui.go`, `runtime_frame.go`, `runtime_overlay.go`, `runtime_csqc.go`).

---

## 4. Verification & Testing Strategy

1. **Line Ceiling Verification**:
   ```bash
   TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/testutil -run TestProjectFilesUnderLineCeiling -count=1
   ```
2. **Full Build & Test**:
   ```bash
   mise run verify
   ```
