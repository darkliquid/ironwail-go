# Implementation Plan: Arena / Region Allocators for Level Lifetimes

**Priority**: #3 (Item 5 from Roadmap)  
**Status**: Planned  
**Target Milestone**: Phase 3  

---

## 1. Executive Summary & Architectural Context

Original Quake managed memory for maps using the **Hunk**: a bump-allocated memory block where BSP nodes, vertices, lightmaps, textures, and models were loaded linearly. When changing levels, the engine did not free objects individually; it wiped the Hunk offset back to 0 in a single operation.

In `ironwail-go`, level assets are currently allocated on the standard Go GC heap via `make()` and `new()`. Changing levels relies on the Go garbage collector to sweep tens of thousands of objects. During rapid level changes or map resets, GC sweep cycles introduce frame stutters.

The goal of this project is to implement a Go bump-allocated **Arena / Region allocator** package (`internal/engine/arena`) for map-lifetime assets (BSP geometry, MDL models, lightmaps) that can be reset in a single operation when changing maps, eliminating level-transition GC overhead.

---

## 2. Existing Code Analysis & Current State

- **BSP Loader**: `internal/bsp/` allocates vertices, edges, faces, nodes, and leaves as standard Go slices.
- **Model Loader**: `internal/model/` allocates alias model skins, triangles, and animation frames on the heap.
- **Renderer Upload**: `internal/renderer/world_upload_gogpu.go` builds vertex arrays on the heap before uploading to GPU buffers.
- **Session Lifecyles**: `internal/host/session.go` (`Host.LoadMap`) orchestrates unmounting the old map and mounting the new map.

---

## 3. Step-by-Step Implementation Sequence

### Step 3.1: Package Scaffolding (`internal/engine/arena`)
- **Files**: `internal/engine/arena/arena.go`, `arena_test.go`
- **Actions**:
  - Implement a region allocator backed by pre-allocated `[]byte` chunks (e.g. 4 MB chunks).
  - Expose typed allocation method: `Alloc[T any](a *Arena, count int) []T`.
  - Expose reset method: `a.Reset()`, which resets chunk offsets without freeing backing memory.

### Step 3.2: Integrate into BSP Loader
- **Files**: `internal/bsp/bsp.go`, `lump.go`
- **Actions**:
  - Update `bsp.Load` to accept `*arena.Arena` parameter.
  - Allocate BSP vertex, leaf, plane, and node slices out of the level arena.

### Step 3.3: Integrate into Model Loader
- **Files**: `internal/model/alias.go`, `sprite.go`
- **Actions**:
  - Update MDL animation frame and vertex loading to allocate from `*arena.Arena`.

### Step 3.4: Integrate into Renderer World Upload
- **Files**: `internal/renderer/world_upload_gogpu.go`
- **Actions**:
  - Use level arena for transient CPU vertex packing buffers during map upload.

### Step 3.5: Wire Level Unload Hook in Host Session
- **Files**: `internal/host/session.go`
- **Actions**:
  - Call `levelArena.Reset()` in `Host.LoadMap` before parsing the new map.

---

## 4. Edge Cases & C Parity Oracles

- **Memory Alignment**: Go structs must be properly aligned (4-byte alignment for `float32`, 8-byte for pointers/int64). The arena allocator must round allocation offsets to 8-byte boundaries.
- **C Parity Oracle**: `ironwail/Quake/common.c` (`Hunk_AllocName`, `Hunk_FreeToLowMark`).

---

## 5. Testing & Verification Plan

1. **Arena Package Unit Tests**:
   ```bash
   TMPDIR=/home/darkliquid/Projects/ironwail-go/.tmp CGO_ENABLED=0 go test ./internal/engine/arena/... -count=1
   ```
2. **Allocation Profiling**:
   - Run console commands `profile_cpu_start`, `map e1m1`, `map e1m2`, `profile_dump_allocs` to verify that level transition heap allocations drop significantly.
