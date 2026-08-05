# Implementation Plan 17: Sub-Package Test Isolation & Synthetic Test Scaffolding

**Priority**: High  
**Status**: Planned  
**Target Milestone**: Phase 17  

---

## 1. Executive Summary & Architectural Context

Per the project's testing conventions (AGENTS.md):
> *"Test helpers are package-local, not testutil constructors. Synthetic world models allow deterministic unit tests without requiring external pak assets."*

Following the modularisation of large packages into sub-packages (`internal/renderer/world`, `internal/renderer/alias`, `internal/server/physics`), several tests still rely on top-level integration scaffolding or shared game state.

This plan refactors unit tests across all extracted sub-packages to run in complete isolation with zero cross-package circular dependencies and fast execution times (<10ms per package).

---

## 2. Technical Strategy & Architecture

1. **Sub-package Synthetic Test Fixtures**:
   - Provide package-local test builders (`newTestLightmapAtlas()`, `newTestDecalManager()`, `newTestPhysicsServer()`).
   - Eliminate dependencies on `pak0.pak` for unit tests by providing minimal in-memory synthetic models.
2. **Deterministic Parity Assertion Helpers**:
   - Standardize `testutil.CompareStructs` and package-local parity validators across sub-packages.
3. **Zero-Allocation Test Assertions**:
   - Add benchmark regression tests (`Benchmark*`) for all critical sub-package hot paths.

---

## 3. Step-by-Step Implementation Sequence

### Step 17.1: Isolated Tests for `internal/renderer/lightmap`
- **Files**: `internal/renderer/lightmap/lightmap_test.go`
- **Actions**: Add unit tests verifying lightmap page allocation, sample packing, and intensity scaling without GPU context dependencies.

### Step 17.2: Isolated Tests for `internal/renderer/decal`
- **Files**: `internal/renderer/decal/decal_test.go`
- **Actions**: Add unit tests verifying mark polygon clipping against synthetic coplanar BSP planes.

### Step 17.3: Isolated Tests for `internal/server/edict`
- **Files**: `internal/server/edict/edict_test.go`
- **Actions**: Add unit tests verifying edict pool recycling, field offsets, and entvars zero-sync field accessors.

---

## 4. Verification & Testing Strategy

```bash
TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/renderer/... ./internal/server/... -count=1
```
