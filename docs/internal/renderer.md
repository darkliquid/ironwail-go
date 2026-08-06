# Package `renderer`

## Purpose
The `renderer` package provides the graphics engine for Ironwail-Go. It abstracts
the complexities of modern GPU APIs (specifically WebGPU via the `gogpu`
library) and provides a unified interface for rendering 3D world geometry, 2D
overlays, and special effects like particles and dynamic lighting.

> **Status note (2026-07-27):** The renderer is functional but unfinished, with
> a known open bug in texture atlas overflow on BSP2 large maps (>256
> textures). Water translucency and brush entity texture animation are
> resolved. See `docs/RENDERER_LEARNING_PLAN.md` "State of the Renderer" for
> details, and `docs/diagnoses/` for investigation records. This document
> describes *what is currently in the code*, not what is "correct" or final.

## Key Types & Interfaces
- **`Renderer`** (`renderer_gogpu.go:118`): The main entry point that manages
  the GPU application window, frame loop, and all GPU-side resource caches
  (pipelines, buffers, textures, lightmaps, materials). Its ~126 flat fields
  are grouped into seven anonymous embedded domain value structs
  (`worldRendererState`, `aliasRendererState`, `spriteRendererState`,
  `particleRendererState`, `decalRendererState`, `polyBlendRendererState`,
  `overlayRendererState`) so future leaf code can take the slice of state it
  owns instead of the whole renderer.
- **`RenderContext`** (`types.go:23`): An interface passed to drawing callbacks
  that provides methods for both 2D (e.g., `DrawPic`, `DrawFill`) and 3D
  rendering operations.
- **`Backend`** (`types.go:69`): Defines the lifecycle and window management
  methods (e.g., `Run`, `Shutdown`, `OnDraw`).
- **`DrawContext`** (`renderer_gogpu.go:17`): The concrete implementation of
  `RenderContext` for the `gogpu` backend. Wraps a `*gogpu.Context` and holds
  per-frame render state. All HAL (hardware abstraction layer) render methods
  live on this type.
- **`Core`** (`core_gogpu.go:46`): Headless-capable GPU core holding the wgpu
  Instance/Adapter/Device/Queue. Used for both windowed and screenshot/headless
  rendering.
- **`WorldRenderData`** (`renderer_gogpu_world.go:57`): A passive data holder
  for GPU-side world resources. Visible face selection is done by
  `selectVisibleWorldFaces` in `renderer_gogpu_world_shared.go`, called from
  `renderWorldInternal` in `renderer_gogpu_world_render.go`.

## Core Workflow
1. **Setup**: `renderer.New()` (`renderer_gogpu_runtime.go:20`) reads video
   cvars and constructs the `Renderer`. `NewWithConfig()` (`:35`) accepts an
   explicit `Config`.
2. **Loop**: The `Renderer` delegates to `gogpu.App` for the event loop.
   `OnDraw()` (`renderer_gogpu_runtime.go:165`) registers the frame draw
   callback; `OnUpdate()` (`:215`) registers the game logic update callback.
3. **Frame** (`RenderFrame()` at `renderer_gogpu_frame.go:128`): The full
   render pipeline, executed in ordered phases:
   - **Phase 1 — Clear**: Clear the screen or offscreen scene
     render target.
   - **Phase 2 — World BSP** (`dc.renderWorld()` `renderer_gogpu_frame.go:188`
     → `renderWorldInternal()` `renderer_gogpu_world_render.go:27`): Cluster
     compute dispatch for dynamic lights, then the opaque/alpha-test/turbulent/
     sky world render pass (pass helpers in `renderer_gogpu_world_render_passes.go`).
   - **Phase 3 — Entities** (`dc.renderEntities()` `renderer_gogpu_frame.go:243`):
     Opaque brush entities, alias models, particles, sky brush entities, opaque
     liquid brush entities, external skybox overlay, then translucent water +
     translucent entities (collected at `:669`, sorted and drawn by
     `renderGoGPUSortedTranslucentFaceRendersHAL` in
     `renderer_gogpu_world_translucent.go:519`), decal marks, sprites,
     translucent particles.
   - **Phase 4 — Viewmodel** (`:249`): First-person weapon model
     (`renderViewModelHAL` `renderer_gogpu_world_alias.go:590`).
   - **Phase 4b — Scene composite** (`:254`): Blit offscreen scene render
     target to the swapchain surface, applying underwater warp if active
     (`compositeSceneRenderTarget` `renderer_gogpu_warpscale.go:472`).
   - **Phase 4c — PolyBlend** (`:268`): Screen-space color tint
     (`renderPolyBlendHAL` `polyblend_gogpu.go:224`).
   - **Phase 5 — 2D Overlay** (`:284`): HUD/menu/console drawn CPU-side
     into a single texture and blitted to the screen
     (`flush2DOverlay` `renderer_gogpu_overlay.go:32`).

## Subpackages
| Subpackage | Purpose |
| --- | --- |
| `alias/` | Alias (MDL) model mesh interpolation, frame desc, pose blending. |
| `gogpu/` | gogpu input backend adapter — bridges gogpu window input to `input.Backend`. |
| `particle/` | Particle CPU math/state shared helpers. |
| `pipeline/` | World render-pipeline + layout constructors. |
| `lightmap/` | CPU lightmap atlas compositing, dirty tracking, page stacking. |
| `decal/` | Decal mark lifetime, quad geometry, atlas seeding. |
| `oit/` | Order-independent transparency shared helpers (config currently disabled). |
| `scrap/` | Skyline bin-packing scrap allocator for small 2D UI textures. |
| `sky/` | External skybox cubemap face loading (PNG/TGA/JPG). |
| `surface/` | Surface texture animation chains (`SURF_DRAWTURB` linked lists). |
| `warpscale/` | Shared waterwarp FOV scale math. |
| `world/` | Pure-Go world logic: BSP types, lightmap samples, liquid alpha, fog, sky cvars, transforms, BSP texture-table helpers. |
| `world/gogpu/` | gogpu-specific world helpers: brush entity draw building, translucent face planning, vertex/alias byte packing (`buffer.go`, `aliasbytes.go`), buffer/texture/resources creation (`resources.go`), sprite/decal draw params, WGSL shaders for alias/sprite/decal. |

## Integration
- **Host**: The `host` package initializes the renderer and sets up the main callbacks.
- **Client**: The client logic uses the `RenderContext` to draw the game world and the HUD.
- **Image**: The package uses `internal/image` for loading and processing Quake-format images (LMP, PCX).
- **Cvars**: Rendering behavior is heavily influenced by cvars like `r_gamma`, `vid_width`, `r_dynamic`, `r_wateralpha`, `r_telealpha`. Cvars are registered in `internal/game/game_init.go`.

## Learning Tips
- **Start with the learning plan**: `docs/RENDERER_LEARNING_PLAN.md` is a
  stage-by-stage curriculum that takes a reader from "what is a GPU?" through
  the full frame pipeline, with external citations to scratchapixel and
  webgpufundamentals.
- **Vertex layout**: Read `docs/VERTEX_LAYOUT.md` first — it documents the
  48-byte `WorldVertex` contract that flows from Go struct → byte packer → WGSL
  `@vertex` input.
- **Shaders are the most readable entry point**: All WGSL shaders are
  self-contained Go string constants in:
  - `renderer_gogpu_world_shaders.go` (world vertex/fragment + uniforms)
  - `renderer_gogpu_world_compute_shaders.go` (cluster dynamic light compute)
  - `world/gogpu/shaders.go` (alias, sprite, decal)
  - `polyblend_gogpu.go`, `renderer_gogpu_warpscale.go`,
    `overlay_composite_gogpu.go`, `renderer_gogpu_particle.go`
- **Multi-pass ordering**: The opaque/translucent ordering is subtle and the
  subject of ongoing bug investigations. Read `docs/diagnoses/` for context
  before changing render pass boundaries.
- **Dynamic Lighting**: Look at `internal/renderer/dynamic_light_pool.go` for
  the implementation of Quake's iconic dynamic lights, and
  `renderer_gogpu_world_cluster_compute.go` for the cluster-forward compute
  pipeline.
