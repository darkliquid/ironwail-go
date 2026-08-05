# Implementation Plan: Engine Modularisation & Go Idiom Adoption

**Priority**: #6 (Item 7 from Roadmap)  
**Status**: Completed  
**Target Milestone**: Phase 6  




---

## 1. Executive Summary & Goals

The Ironwail-Go port was initially a mechanical translation from C, producing
large monolithic packages. The goal of this plan is to transform the codebase
into a clean, modular, well-documented architecture that:

1. **Is clean and modular** — packages have focused responsibilities,
   clear boundaries, and minimal coupling
2. **Is easy to unit test in isolation** — components can be tested without
   spinning up the entire engine
3. **Is verbosely commented and documented** — every package has a `doc.go`
   explaining its role, C lineage, and how it fits in the larger engine
4. **Emphasises education** — comments explain *why* the engine works this
   way, what the C original did, and how Quake's architecture maps to Go

---

## 2. Current State Analysis

### 2.1 Monolithic Packages

**`internal/renderer/`** (root `package renderer`):
- ~80+ `.go` files in a single package
- `Renderer` struct has ~370 fields spanning world rendering, alias models,
  sprites, particles, decals, overlays, and post-processing
- No sub-package boundaries within the root renderer package
- Largest files: `world_render_gogpu.go` (~988 lines), `renderer_gogpu_frame.go`
  (~855 lines), `world_geometry_gogpu.go` (~763 lines)
- `renderWorldInternal()` is a 694-line monolith

**`internal/server/`** (`package server`):
- 75 `.go` files in a single package, no sub-packages
- Largest files: `world.go` (~995 lines), `sv_main.go` (~950 lines),
  `physics.go` (~927 lines), `sv_send.go` (~900 lines)
- Physics, collision, networking, entity management, and savegame all share
  one namespace

### 2.2 Missing Documentation

11 packages lack `doc.go` files:
- `internal/async`, `internal/compatrand`, `internal/mods`
- `internal/renderer/alias`, `internal/renderer/world`,
  `internal/renderer/world/gogpu`, `internal/renderer/surface`,
  `internal/renderer/scrap`, `internal/renderer/sky`,
  `internal/renderer/gogpu`, `internal/renderer/oit`, `internal/renderer/warpscale`

### 2.3 Test Coverage Gaps

- `internal/renderer/` root: ~80 source files, only ~25 test files
- Zero tests in: `renderer/sky`, `renderer/surface`, `renderer/oit`,
  `renderer/warpscale`

### 2.4 C Idioms Remaining

- Some functions return integer status codes instead of `error`
- Package-level global variables exist in `server.go` and `host.go`
- `QuakeGo` source (`pkg/qgo/quakego`) is intentionally exempt (mechanical
  port, see `AGENTS.md` gotcha #2)

---

## 3. Implementation Phases

### Phase 1: Documentation Foundation (No Code Changes)

**Goal:** Ensure every package has a `doc.go` that explains what it does,
where the C source came from, and how it fits in the engine.

**Actions:**

1. **Add `doc.go` to all 11 missing packages** listed in §2.2

2. **Enhance existing `doc.go` files** to include:
   - `# Package Purpose` — what this package does in 2-3 sentences
   - `# Original C lineage` — which C source files this mirrors
   - `# Architecture context` — how this package relates to others
   - `# Key types` — the main structs/interfaces with brief descriptions
   - `# Testing notes` — how to test this package in isolation

3. **Add educational package-level comments** to the largest/most complex
   files:
   - `world_render_gogpu.go` — explain the 5-phase world render pipeline
   - `renderer_gogpu_frame.go` — explain the frame loop and phase ordering
   - `physics.go` — explain Quake's physics movetypes and hull tracing
   - `sv_send.go` — explain the delta encoding protocol

4. **Update `docs/LEARNING_GUIDE.md`** with a revised package map that
   reflects the modular structure after all phases complete

**Verification:** `mise run build` succeeds, all `doc.go` files present.

---

### Phase 2: Split `internal/server/` into Sub-Packages

**Goal:** Break the 75-file monolith into focused sub-packages with clear
boundaries and testable interfaces.

**Target Structure:**

```
internal/server/
├── doc.go                      # Package overview, C lineage
├── types.go                    # Shared types: MoveType, SolidType, TraceResult
├── server.go                   # Server struct, SpawnServer, frame loop
├── sv_main.go                  # Init, Shutdown, model cache
├── sv_client.go                # Client connection, signon
├── physics/
│   ├── doc.go                  # Physics movetypes, C lineage (sv_phys.c)
│   ├── physics.go              # SV_Physics(), per-movetype dispatch
│   ├── walk.go                # SV_WalkMove, SV_FlyMove
│   ├── push.go                # SV_PushMove, SV_PushRotate
│   ├── movement.go             # Monster AI movement (NewChaseDir, etc.)
│   └── physics_test.go         # Unit tests (use synthetic BSP)
├── collision/
│   ├── doc.go                  # Hull tracing, C lineage (world.c)
│   ├── trace.go                # SV_Trace, recursiveHullCheck
│   ├── world.go                # Area grid, touch links, SV_LinkEdict
│   └── collision_test.go       # Unit tests (use synthetic BSP)
├── netcode/
│   ├── doc.go                  # Delta encoding, C lineage (sv_send.c)
│   ├── send.go                 # WriteEntityUpdate, WriteClientData
│   ├── stats.go                # Stat tracking, player stats
│   └── netcode_test.go         # Protocol round-trip tests
└── savegame/
    ├── doc.go                  # Save/load format, C lineage (sv_save.c)
    ├── savegame.go             # Save/LoadGame
    └── savegame_test.go        # Round-trip save/load tests
```

**Actions:**

1. **Create `server/physics/` sub-package:**
   - Move `physics.go`, `movement.go` to `server/physics/`
   - Extract `SV_WalkMove`, `SV_FlyMove`, `SV_PushMove` into separate files
   - Define `PhysicsEngine` interface for the server to call
   - Tests use `synthetic_bsp_helper.go` for deterministic unit tests
   - Add educational comments explaining each movetype and its C equivalent

2. **Create `server/collision/` sub-package:**
   - Move `world.go` (collision parts) to `server/collision/`
   - Extract `SV_Trace`, `SV_LinkEdict`, `SV_TouchLinks` 
   - Define `CollisionWorld` interface
   - Tests use synthetic BSP for deterministic tracing

3. **Create `server/netcode/` sub-package:**
   - Move `sv_send.go` to `server/netcode/`
   - Extract entity delta encoding and clientdata serialization
   - Add educational comments explaining the protocol flags (U_ORIGIN, SU_*)
   - Tests verify protocol round-trips (encode → decode → compare)

4. **Create `server/savegame/` sub-package:**
   - Move `savegame.go` to `server/savegame/`
   - Tests verify save/load round-trips

**Constraints:**
- No import cycles — sub-packages import `server` for shared types,
  not the reverse
- Parity: physics and netcode behavior must not change
- `pkg/qgo/quakego` is exempt from any changes

**Verification:** `mise run test` passes, `mise run parity-all` shows no
regressions.

---

### Phase 3: Split `internal/renderer/` Root Package

**Goal:** Break the 80-file monolith into focused sub-packages with
clear separation of concerns.

**Target Structure:**

```
internal/renderer/
├── doc.go                      # Package overview
├── types.go                    # RenderContext, Backend, Config interfaces
├── renderer_gogpu.go           # Renderer struct (slimmed), DrawContext
├── renderer_gogpu_runtime.go   # NewWithConfig, Run, Shutdown
├── renderer_gogpu_frame.go      # RenderFrame orchestrator (slimmed)
├── camera.go                   # Camera math
├── screen.go                   # Canvas/FOV/refdef
├── texture.go                  # Palette conversion
│
├── worldrender/                # World geometry + BSP rendering
│   ├── doc.go                  # BSP render pipeline, C lineage
│   ├── render.go               # renderWorldInternal (refactored)
│   ├── geometry.go             # Vertex/face/index extraction
│   ├── resources.go            # Texture atlas, materials, lightmap
│   ├── upload.go               # UploadWorld, GPU resource creation
│   ├── shared.go               # Visibility, alpha settings
│   ├── shaders.go              # WGSL shader sources
│   └── *_test.go
│
├── entities/                   # Alias models, sprites, brush entities
│   ├── doc.go                  # Entity render pipeline, C lineage
│   ├── alias.go                # Alias model rendering (CPU vertex path)
│   ├── sprite.go               # Sprite rendering
│   ├── brush.go                # Brush entity rendering
│   └── *_test.go
│
├── overlay/                    # 2D HUD, menu, console compositing
│   ├── doc.go                  # 2D overlay pipeline
│   ├── overlay2d.go            # Overlay2D struct, DrawPic, DrawFill
│   ├── composite.go            # Scene composite, polyblend
│   └── *_test.go
│
├── effects/                    # Particles, decals, polyblend
│   ├── doc.go                  # Effect systems, C lineage
│   ├── particle.go             # Particle rendering
│   ├── decal.go                # Decal rendering
│   └── *_test.go
│
├── pipeline/                   # Render pass management, depth, scene target
│   ├── doc.go                  # Pipeline state management
│   ├── depth.go                # Depth texture management
│   ├── scenetarget.go          # Scene render target + compositing
│   └── *_test.go
│
└── (existing sub-packages remain: alias/, world/, world/gogpu/, 
     surface/, scrap/, sky/, gogpu/, oit/, warpscale/)
```

**Actions:**

1. **Refactor `renderWorldInternal()` (694 lines → ~5 focused functions):**
   - `prepareWorldRender()` — depth texture, render target, uniform setup
   - `renderSkyPass()` — sky face collection and draw
   - `renderOpaquePass()` — opaque world face batching and draw
   - `renderTranslucentPass()` — translucent liquid face collection
   - `submitWorldRender()` — encoder finish, queue submit, timing

2. **Split `Renderer` struct (~370 fields → grouped sub-structs):**
   ```go
   type Renderer struct {
       mu      sync.RWMutex
       app     *gogpu.App
       config  Config
       camera  CameraState
       
       worldRender   *worldRenderResources    // ~80 fields
       aliasRender   *aliasRenderResources     // ~30 fields
       spriteRender  *spriteRenderResources    // ~15 fields
       effectRender  *effectRenderResources    // ~20 fields
       overlayRender *overlayRenderResources   // ~15 fields
       pipelineState *pipelineResources        // ~25 fields
   }
   ```
   Each sub-struct is defined in its sub-package, making the resources
   testable in isolation.

3. **Move files to sub-packages** following the target structure above.
   Each sub-package has a `doc.go` explaining its role and C lineage.

4. **Add educational comments** to every exported function:
   - What it does (1-2 sentences)
   - Why it's needed (architectural context)
   - C lineage reference (e.g., `// Where in C: R_DrawAliasModel in r_alias.c`)
   - How to test it in isolation

**Constraints:**
- No functional changes — the rendered output must be identical
- Sub-packages import `renderer` for the `Renderer` struct, not the
  reverse (avoids cycles)
- Existing sub-packages (`alias/`, `world/`, etc.) remain unchanged
- Sprite bind group separation (from Wave 2 work) must be preserved

**Verification:** `mise run test` passes, visual parity with e1m1 and
qbj2 maps.

---

### Phase 4: Go Idiom Adoption

**Goal:** Replace C-style idioms with idiomatic Go patterns where it
improves clarity without breaking parity.

**Actions:**

1. **Replace integer error codes with `error` types:**
   - Define sentinel errors: `ErrInvalidMap`, `ErrClientDisconnected`,
     `ErrServerFull`, etc.
   - Replace `return 0` / `return 1` patterns with `return nil` /
     `return err`
   - **Exempt:** `pkg/qgo/quakego` (mechanical port)

2. **Replace package-level globals with struct fields:**
   - Move `server.go` globals into the `Server` struct
   - Move `host.go` globals into the `Host` struct
   - **Exempt:** `pkg/qgo/quake` engine stubs (need package-level
     access for QCVM)

3. **Add `context.Context` to long-running operations:**
   - Asset loading (texture upload, BSP parsing)
   - Audio streaming
   - NOT for the frame loop (Quake's frame is synchronous)

4. **Enforce package encapsulation:**
   - Unexport internal helper functions (lowercase first letter)
   - Only export what other packages need to call
   - Move test helpers into `*_test.go` files in the same package

**Verification:** `mise run lint` passes, `mise run test` passes.

---

### Phase 5: Test Coverage Expansion

**Goal:** Add unit tests for all sub-packages, with emphasis on
testability in isolation.

**Actions:**

1. **Add tests for zero-coverage packages:**
   - `renderer/sky/` — sky texture loading, face classification
   - `renderer/surface/` — lightmap allocator, surface texture
   - `renderer/oit/` — OIT (if used)
   - `renderer/warpscale/` — water warp math, scene target

2. **Add isolation tests for new sub-packages:**
   - `server/physics/` — test each movetype with synthetic BSP
   - `server/collision/` — test trace with synthetic BSP
   - `server/netcode/` — protocol encode/decode round-trips
   - `server/savegame/` — save/load round-trips

3. **Add table-driven tests** for complex functions:
   - `SV_FlyMove` — multiple bump scenarios
   - `WriteEntityUpdate` — all flag combinations
   - `BuildModelGeometry` — various face configurations

4. **Add integration test helpers:**
   - `TestRenderer` — minimal renderer for shader/pipeline tests
     (no window, no swapchain, headless GPU context)
   - `TestServer` — minimal server for physics tests (synthetic BSP)

**Verification:** `mise run test` passes, `mise run race` passes,
test coverage percentage increases.

---

### Phase 6: Documentation Polish

**Goal:** Final pass to ensure the codebase reads like an educational
Quake engine textbook.

**Actions:**

1. **Add `# Architecture overview` section to root `doc.go`:**
   - ASCII diagram of the package dependency graph
   - Data flow: BSP → geometry → atlas → GPU → render pass → present
   - Control flow: Host → Server → QC → Client → Renderer

2. **Update `docs/LEARNING_GUIDE.md`:**
   - Revised package map reflecting new sub-package structure
   - New "How to test a subsystem in isolation" section
   - New "Understanding the render pipeline" walkthrough
   - Cross-references to the C source files

3. **Add `docs/CROSS_REFERENCE.md`:**
   - Table mapping every Go package to its C source file(s)
   - Table mapping every Go function to its C equivalent
   - Notes on where the port intentionally diverges from C

4. **Review all package `doc.go` files for:**
   - Clear purpose statement
   - C lineage reference
   - Architecture context (how it relates to other packages)
   - Testing notes
   - Educational commentary

**Verification:** `mise run build` succeeds, all docs present and
accurate.

---

## 4. Edge Cases & Constraints

- **QuakeGo Exemption:** `pkg/qgo/quakego` is intentionally exempt from
  Go-idiomatic rewrites per `AGENTS.md` gotcha #2
- **Parity Oracle:** All refactoring must preserve behavioral parity
  with C Ironwail/Quake. Run `mise run parity-all` after each phase
- **No build tags:** Zero `.go` files contain `//go:build` directives.
  The gogpu renderer is always compiled. Do not introduce build tags
- **CGO off:** `CGO_ENABLED=0` always. Never introduce CGO dependencies
- **File length limit:** All `.go` files must stay under 1000 lines
  (enforced by `internal/testutil/file_length_test.go`)
- **1000-line check:** After splitting, verify no new file exceeds 1000
  lines: `find . -name "*.go" -exec wc -l {} \; | sort -rn | head -20`

---

## 5. Testing & Verification Plan

1. **After each phase:**
   ```bash
   mise run test      # all tests pass
   mise run build     # go generate + go build
   mise run lint      # golangci-lint + govulncheck
   ```

2. **After Phase 2 (server split):**
   ```bash
   mise run parity-all  # visual parity with C Ironwail
   ```

3. **After Phase 3 (renderer split):**
   ```bash
   # Test specific maps known to exercise all render paths
   mise run run -- +map e1m1        # standard Quake
   mise run run -- -game qbj2 +map qbj2_zetabyt  # large mod map
   mise run run -- +playdemo demo1.dem  # demo playback
   ```

4. **After Phase 5 (test expansion):**
   ```bash
   mise run race  # race detector
   ```

---

## 6. Recommended Execution Order

1. **Phase 1** (Documentation) — no risk, immediate educational value
2. **Phase 2** (Server split) — highest complexity, do while context is fresh
3. **Phase 3** (Renderer split) — largest scope, depends on Phase 2 patterns
4. **Phase 4** (Go idioms) — incremental, can be done alongside other work
5. **Phase 5** (Test coverage) — after packages are split and testable
6. **Phase 6** (Documentation polish) — final pass after structure stabilizes

Each phase is independently committable. Start with Phase 1 (zero risk)
and stop after any phase if priorities change.
