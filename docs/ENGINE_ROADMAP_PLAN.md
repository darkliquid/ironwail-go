# ironwail-go Engineering Roadmap & Implementation Plan

This document defines the step-by-step implementation strategy for the major engine projects in **ironwail-go**, ordered strictly according to the priority sequence:

1. **Phase 1 (Item 2)**: Direct-VM Accessors & Zero-Sync QCVM (Steps 3–5) — **COMPLETED** (commit `570e806556cc614d9cec0ea6d2c967dd0cb3a241`)
2. **Phase 2 (Item 3)**: Texture Atlas Storage Buffer Upgrade (BSP2 Large Map Fix)
3. **Phase 3 (Item 5)**: Arena / Region Allocators for Level Lifetimes — **COMPLETED** (commit `ab41b31`)
4. **Phase 4 (Item 4)**: CSQC (Client-Side QuakeC) Host/Client Runtime Integration — **COMPLETED**
5. **Phase 5 (Item 6)**: Parity Closure & Sign-off on `qbj3_stickflip`
6. **Phase 6 (Item 7)**: Engine Modularisation & Go Idiom Adoption
7. **Phase 7 (Item 1)**: Browser Port via WebAssembly (`GOOS=js GOARCH=wasm`)

---

## Phase 1: Direct-VM Accessors & Zero-Sync QCVM (Steps 3–5) [COMPLETED]

**Status**: Completed (Merged in commit `570e806556cc614d9cec0ea6d2c967dd0cb3a241`)

### Objective
Complete the migration of server entity state from dual-storage (`EntVars` struct + `QCVM.Edicts []byte`) to direct-VM accessor methods, and delete the reflection-based synchronization layer.

### Target Files
- `internal/server/entity_accessors.go` (157 existing typed accessors)
- `internal/server/physics.go`, `physics_loop.go`, `physics_push.go`, `physics_step.go`, `physics_toss.go`, `world.go`
- `internal/server/server_qc_sync.go` (to be deleted)
- `internal/server/qc_trace.go` (`executeQCFunction`)
- `internal/server/savegame.go`

### Step-by-Step Execution
1. **Migrate Hot-Path Reads/Writes to Accessors (Step 2 completion)**:
   - Replace direct struct access (`ent.Vars.Origin`, `ent.Vars.Velocity`, `ent.Vars.Flags`, `ent.Vars.Solid`, `ent.Vars.Movetype`) across `physics*.go` and `world.go` with accessor calls (`ent.Origin()`, `ent.SetOrigin()`, `ent.Velocity()`, `ent.SetVelocity()`, etc.).
2. **Update Save/Load Logic (Step 4 prerequisite)**:
   - Rewrite `savegame.go` to serialize/deserialize `QCVM.Edicts` byte array slice directly rather than saving `EntVars` struct fields.
3. **Delete Reflection Sync Layer (Step 3)**:
   - Delete `internal/server/server_qc_sync.go` (`syncAllToQCVM`, `syncAllFromQCVM`, `syncEdictToQCVM`, `syncEdictFromQCVM`).
   - Remove sync calls from `executeQCFunction` in `qc_trace.go`.
4. **Delete `EntVars` Struct**:
   - Remove `EntVars` struct definition from `internal/server/` and `pkg/types`.

### Testing Strategy
- **Unit & Integration**: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/qc/... ./internal/server/...`
- **Physics Parity**: Run `TestPhysicsWalkJump`, `TestPhysicsPusher`, `TestPhysicsStep` to verify zero movement regression.

---

## Phase 2: Texture Atlas Storage Buffer Upgrade (BSP2 Large Map Fix)

### Objective
Replace the hardcoded 256-entry uniform buffer for texture materials with a WebGPU storage buffer, allowing maps with >254 textures (e.g. `qbj2_start`) to render without buffer overflow.

### Target Files
- `internal/renderer/world_shaders_gogpu.go`
- `internal/renderer/world_material_gogpu.go`
- `internal/renderer/world_resources_gogpu.go`
- `internal/renderer/world_pipelines_gogpu.go`

### Step-by-Step Execution
1. **WGSL Shader Declaration**:
   - Change `@group(0) @binding(1) var<uniform> materials: array<MaterialData, 256>;` to `@group(0) @binding(1) var<storage, read> materials: array<MaterialData>;`.
2. **Buffer Creation & Bind Group Layout**:
   - In `world_material_gogpu.go`, change `BufferBindingTypeUniform` to `BufferBindingTypeReadOnlyStorage`.
   - Add `wgpu.BufferUsageStorage` usage flag to the materials GPU buffer.
3. **Dynamic Sizing**:
   - Allocate `baseMaterials` slice to `textureCount + 2` dynamically, removing the 256-entry clamp.
4. **Diagnostic Cleanup**:
   - Update `diag_atlas.go` warnings to reflect dynamic storage buffer capacity.

### Testing Strategy
- **Renderer Unit Tests**: `TMPDIR=.../.tmp CGO_ENABLED=0 go test ./internal/renderer/...`
- **BSP Inspection**: `bspdiag info <quake_dir> maps/qbj2_start.bsp` to verify map face and material counts load cleanly.

---

## Phase 3: Arena / Region Allocators for Level Lifetimes [COMPLETED]

**Status**: Completed (Merged in commit `ab41b31`)

### Objective
Eliminate garbage collection pressure during level changes and gameplay by introducing a bump-allocated byte arena / region pool for map assets (BSP geometry, models, lightmaps).

### Target Files
- `internal/engine/arena/` (new package)
- `internal/bsp/`
- `internal/model/`
- `internal/renderer/world_upload_gogpu.go`
- `internal/host/session.go`

### Step-by-Step Execution
1. **Create Arena Package**:
   - Build `internal/engine/arena` providing a bump-allocated `Arena` type with `Alloc[T](n int) []T` and `Reset()`.
2. **Integrate into Asset Loaders**:
   - Pass `*arena.Arena` into BSP leaf/vertex parsing and MDL animation frame parsing.
3. **Wire Level Unload Hook**:
   - Call `arena.Reset()` in `Host.LoadMap` (`session.go`) upon unmounting the previous level.

### Testing Strategy
- **Allocation Profiling**: Run `profile_cpu_start`, `map e1m1`, `map e1m2`, `profile_dump_allocs` in console to verify heap allocations drop to near-zero across map transitions.

---

## Phase 4: CSQC (Client-Side QuakeC) Host/Client Runtime Integration [COMPLETED]

**Status**: Completed

### Objective
Complete the host and client runtime execution wiring for Client-Side QuakeC (`csprogs.dat`), enabling CSQC-powered mod HUDs and predicted client entities.

### Target Files
- `internal/client/csqc_runtime.go` (new file)
- `internal/client/client.go`
- `internal/renderer/renderer_gogpu_overlay.go`
- `internal/host/host.go`

### Step-by-Step Execution
1. **CSQC VM Mount**:
   - Load `csprogs.dat` via `internal/qc/csqc.go` during client signon if present in the active mod VFS.
2. **Overlay Draw Hooks**:
   - Call `csqc.CallDrawHud()` and `CallDrawOverlay()` inside `flush2DOverlay()` (`renderer_gogpu_overlay.go`).
3. **Input & Snapshot Event Dispatch**:
   - Route mouse/keyboard events through `csqc.CallInputEvent()` before falling back to default key handling.

### Testing Strategy
- **Unit Tests**: Add `TestCSQCHudDrawLifecycle` in `internal/client/`.

---

## Phase 5: Parity Closure & Sign-off on `qbj3_stickflip`

### Objective
Achieve official visual and behavioral parity sign-off on the `qbj3_stickflip` community map pack.

### Target Files
- `internal/renderer/world_render_gogpu.go`
- `internal/renderer/world_gogpu_decal.go`
- `internal/renderer/world_gogpu_brush_render.go`
- `testdata/parity/viewpoints.json`
- `docs/PARITY.md`

### Step-by-Step Execution
1. **Lighting & Contrast Parity**:
   - Compare GoGPU fragment shader overbright multiplier (`lightmap * 2.0`) and lightmap page math against C `gl_rmain.c` to fix the upper-ceiling contrast delta.
2. **Decal Depth & Z-Fighting**:
   - Adjust depth bias settings in `world_gogpu_decal.go` for coplanar brush/decal surfaces.
3. **Expand Parity Viewpoints**:
   - Extract `viewpos` coordinates from C Ironwail for dense outdoor and trigger-heavy scenes in `qbj3_stickflip` and save into `testdata/parity/viewpoints.json`.

### Testing Strategy
- Run `mise run parity-ref`, `mise run parity-go`, `mise run parity-compare` to verify visual diffs fall within tolerance.

---

## Phase 6: Engine Modularisation & Go Idiom Adoption

### Objective
Refactor large monolithic packages (`internal/server`, `internal/game`) into focused sub-packages, and adopt Go-idiomatic control flow and error handling.

### Target Files
- `internal/server/` → `internal/server/physics`, `internal/server/entities`
- `internal/game/`

### Step-by-Step Execution
1. **Package Decomposition**:
   - Split `internal/server` into sub-packages to tighten visibility contracts.
2. **Error Handling & Contexts**:
   - Replace C-style error codes with explicit Go `error` returns and `context.Context` cancellation for async operations.

### Testing Strategy
- Run `mise run lint` and `mise run test`.

---

## Phase 7: Browser Port via WebAssembly (`GOOS=js GOARCH=wasm`)

### Objective
Compile `ironwail-go` to WebAssembly and run it in modern browsers using native WebGPU (`navigator.gpu`).

### Target Files
- `cmd/ironwailgo/main_wasm.go` (new file)
- `internal/renderer/gogpu/wasm_surface.go` (new file)
- `internal/input/wasm_input.go` (new file)
- `index.html`, `wasm_exec.js`

### Step-by-Step Execution
1. **WASM Entry Point**:
   - Create `main_wasm.go` with `//go:build js && wasm`.
2. **WebGPU Canvas Surface**:
   - Bind GoGPU context to HTML5 `<canvas id="canvas">` via `navigator.gpu`.
3. **DOM Input & Worker Audio**:
   - Map DOM `keydown`/`mousemove`/`pointerlock` events to `internal/input.Backend`.
   - Route Oto audio stream to Web Audio API AudioWorklet.

### Testing Strategy
- Build with `GOOS=js GOARCH=wasm go build -o ironwail.wasm ./cmd/ironwailgo` and launch via `npm run dev`.
