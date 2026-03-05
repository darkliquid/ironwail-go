# Ironwail-Go Module-by-Module Port Plan

## Goal
Complete the pure Go port of Ironwail systematically, one module at a time. This plan replaces the previous phased approach with a strict dependency-ordered, bottom-up execution strategy.

## Key Directives
- **Pure Go**: No `cgo`.
- **Third-Party Libraries**: Do NOT port C libraries like `lodepng.c` or `miniz.c`. Use Go standard library equivalents (`image/png`, `archive/zip`, etc.).
- **Executable Acceptance**: Every module must have automated `go test` coverage.
- **Large File Breakdown**: Monolithic C files (e.g., `gl_model.c` at 5k lines) must be logically broken down into smaller Go files (e.g., `model_alias.go`, `model_bsp.go`, `model_sprite.go`).

## Task Dependency Graph

```text
[Wave 1]
  (A) Testing Infrastructure (testutil) -> Required by ALL downstream tasks
  (B) Common & Mathlib (common) -> Required by ALL downstream tasks
       |
       v
[Wave 2]
  (C) Image loading (image)
  (D) BSP loading (bsp/model)
       |
       v
[Wave 3]
  (E) Server (sv_main, world, physics)
       |
       v
[Wave 4]
  (F) Host (host, commands)
  (G) Network (net_main, udp, datagram)
       |
       v
[Wave 5]
  (H) Client (cl_main, cl_parse, input, demo)
  (I) Audio (snd_spatial, mix)
       |
       v
[Wave 6]
  (J) Renderer (WebGPU: core, surface, model, particle, screen)
```

## Task Definitions & Skills

### Wave 1: Foundation

**Task A: Testing Infrastructure**
- [x] **Description**: Create `internal/testutil/assets.go` to locate and load `pak0.pak`. Implement hex-dump/structure comparison helpers.
- **Category**: `unspecified-high`
- **Skills**: None needed.
- **QA**: `go test ./internal/testutil` passes (skips gracefully if pak is missing).

**Task B: Common & Mathlib**
- [x] **Description**: Complete foundational math and string manipulation in `internal/common` missing from `common.c` and `mathlib.c`.
- **Category**: `quick`
- **Skills**: None needed.
- **QA**: `go test ./internal/common` passes.

### Wave 2: Data Formats

**Task C: Image & WAD Loading**
- [x] **Description**: Port `wad.c` to `internal/image/wad.go` and `gl_texmgr.c` texture parsing to `internal/image`. Use `image/png` instead of `lodepng.c`.
- **Category**: `unspecified-high`
- **Skills**: None needed.
- **QA**: `go test ./internal/image` passes texture loads.

**Task D1: BSP Tree Loading**
- [x] **Description**: Port BSP tree loading from `gl_model.c` to `internal/bsp/tree.go`.
- **Category**: `deep`
- **Skills**: None needed.
- **QA**: `go test ./internal/bsp` successfully loads BSP entities and geometry from `pak0.pak`.

**Task D2: Alias Model Loading**
- [x] **Description**: Port Alias model loading from `gl_model.c` to `internal/model/alias.go`.
- **Skills**: None needed.
- **QA**: `go test ./internal/model` successfully loads Alias models from `pak0.pak`.

**Task D3: Sprite Loading**
- [x] **Description**: Port Sprite loading from `gl_model.c` to `internal/model/sprite.go`.
- **Skills**: None needed.
- **QA**: `go test ./internal/model` successfully loads Sprites from `pak0.pak`.

### Wave 3: Server

**Task E1: Server Main & World**
- [x] **Description**: Port `sv_main.c` and `world.c` to `internal/server/sv_main.go` and `internal/server/world.go`.
- **Category**: `deep`
- **Skills**: None needed.
- **QA**: `go test ./internal/server` can initialize a headless server instance and load a map (`start`).

**Task E2: Server Physics**
- [x] **Description**: Port `sv_phys.c` to `internal/server/physics.go`.
- **Category**: `deep`
- **Skills**: None needed.
- **QA**: `go test ./internal/server` passes physics tests.

**Task E3: Server Movement**
- [x] **Description**: Port `sv_move.c` to `internal/server/movement.go`.
- **Category**: `deep`
- **Skills**: None needed.
- **QA**: `go test ./internal/server` passes movement tests.

**Task E4: Server User**
- [x] **Description**: Port `sv_user.c` to `internal/server/user.go`.
- **Category**: `deep`
- **Skills**: None needed.
- **QA**: `go test ./internal/server` passes user command tests.
### Wave 4: Orchestration

**Task F: Host & Command System**
- [x] **Description**: Complete `host.c` to `internal/host/frame.go` and `host_cmd.c` to `internal/host/commands.go`.
- **Category**: `unspecified-high`
- **Skills**: None needed.
- **QA**: `go test ./internal/host` can register commands and execute a mock host frame.

**Task G: Network Protocols**
- [x] **Description**: Port `net_main.c`, `net_udp.c`, `net_dgrm.c` to `internal/net/*` using Go's `net` package.
- **Category**: `unspecified-high`
- **Skills**: None needed.
- **QA**: `go test ./internal/net` successfully sends and receives a mocked Quake connectionless packet.

### Wave 5: Client & Audio

**Task H: Client Logic**
- [x] **Description**: Port `cl_parse.c`, `cl_main.c`, `cl_input.c`, `cl_demo.c` to `internal/client/*`.
- **Category**: `deep`
- **Skills**: None needed.
- **QA**: `go test ./internal/client` can parse a static byte array representing a server sign-on message sequence.

**Task I: Audio Subsystem**
- [x] **Description**: Port `snd_dma.c` (spatial attenuation) and `snd_mix.c` (software audio mixing) to `internal/audio/*`. Skip C codecs.
- **Category**: `unspecified-high`
- **Skills**: None needed.
- **QA**: `go test ./internal/audio` passes mixing math assertions.

### Wave 6: Renderer

**Task J1: WebGPU Core**
- [x] **Description**: Port `gl_rmain.c` to `internal/renderer/core.go`.
- **Skills**: None needed.
- **QA**: `go test ./internal/renderer` can initialize a headless WebGPU context.

**Task J2: WebGPU Surface & Model**
- [x] **Description**: Port `r_brush.c` and `r_alias.c` to `internal/renderer/surface.go` and `internal/renderer/model.go`.
- **Category**: `deep`
- **Skills**: None needed.
- **QA**: `go test ./internal/renderer` passes surface and model rendering tests.

**Task J3: WebGPU Particles**
- [x] **Description**: Port `r_part.c` to `internal/renderer/particle.go`.
- **Category**: `deep`
- **Skills**: None needed.
- **QA**: `go test ./internal/renderer` passes particle rendering tests.

**Task J4: WebGPU Screen**
- [x] **Description**: Port `gl_screen.c` to `internal/renderer/screen.go`.
- **Category**: `deep`
- **Skills**: None needed.
- **QA**: `go test ./internal/renderer` passes screen rendering tests.
- **QA**: `go test ./internal/renderer` passes particle and screen rendering tests.
## Actionable TODO List for Caller

```typescript
// WAVE 1: Foundation
task(category="unspecified-high", load_skills=[], description="Task A: Implement testutil assets and comparison helpers", prompt="Read .sisyphus/plans/module-porting-plan.md for Task A details", run_in_background=false)
task(category="quick", load_skills=[], description="Task B: Complete internal/common and mathlib", prompt="Read .sisyphus/plans/module-porting-plan.md for Task B details", run_in_background=false)

// WAVE 2: Data Formats
task(category="unspecified-high", load_skills=[], description="Task C: Port image and WAD loading to internal/image", prompt="Read .sisyphus/plans/module-porting-plan.md for Task C details", run_in_background=false)
task(category="deep", load_skills=[], description="Task D: Break down gl_model.c and port BSP/Models", prompt="Read .sisyphus/plans/module-porting-plan.md for Task D details", run_in_background=false)

// WAVE 3: Server
task(category="deep", load_skills=[], description="Task E: Port headless server logic (main, world, physics)", prompt="Read .sisyphus/plans/module-porting-plan.md for Task E details", run_in_background=false)

// WAVE 4: Orchestration
task(category="unspecified-high", load_skills=[], description="Task F: Complete Host and Command system", prompt="Read .sisyphus/plans/module-porting-plan.md for Task F details", run_in_background=false)
task(category="unspecified-high", load_skills=[], description="Task G: Port network protocols to internal/net", prompt="Read .sisyphus/plans/module-porting-plan.md for Task G details", run_in_background=false)

// WAVE 5: Client & Audio
task(category="deep", load_skills=[], description="Task H: Port client logic, parsing, and prediction", prompt="Read .sisyphus/plans/module-porting-plan.md for Task H details", run_in_background=false)
task(category="unspecified-high", load_skills=[], description="Task I: Port spatial audio and software mixing", prompt="Read .sisyphus/plans/module-porting-plan.md for Task I details", run_in_background=false)

// WAVE 6: Renderer
task(category="deep", load_skills=[], description="Task J: Port WebGPU Renderer (core, surface, model, screen)", prompt="Read .sisyphus/plans/module-porting-plan.md for Task J details", run_in_background=false)
```