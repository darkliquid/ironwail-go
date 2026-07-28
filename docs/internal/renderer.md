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
- **`Renderer`** (`renderer_gogpu.go:101`): The main entry point that manages
  the GPU application window, frame loop, and all GPU-side resource caches
  (pipelines, buffers, textures, lightmaps, materials).
- **`RenderContext`** (`types.go`): An interface passed to drawing callbacks
  that provides methods for both 2D (e.g., `DrawPic`, `DrawFill`) and 3D
  rendering operations.
- **`Backend`** (`types.go`): Defines the lifecycle and window management
  methods (e.g., `Run`, `Shutdown`, `OnDraw`).
- **`DrawContext`** (`renderer_gogpu.go:16`): The concrete implementation of
  `RenderContext` for the `gogpu` backend. Wraps a `*gogpu.Context` and holds
  per-frame render state. All HAL (hardware abstraction layer) render methods
  live on this type.
- **`Core`** (`core_gogpu.go:46`): Headless-capable GPU core holding the wgpu
  Instance/Adapter/Device/Queue. Used for both windowed and screenshot/headless
  rendering.
- **`WorldRenderData`** (`world.go:57`): A passive data holder for GPU-side
  world resources. Visible face selection is done by `selectVisibleWorldFaces`
  in `world_shared.go:172`, called from `world_render_gogpu.go:333`.

## Core Workflow
1. **Setup**: `renderer.New()` (`renderer_gogpu_runtime.go:19`) reads video
   cvars and constructs the `Renderer`. `NewWithConfig()` (`:34`) accepts an
   explicit `Config`.
2. **Loop**: The `Renderer` delegates to `gogpu.App` for the event loop.
   `OnDraw()` (`renderer_gogpu_runtime.go:149`) registers the frame draw
   callback; `OnUpdate()` (`:199`) registers the game logic update callback.
3. **Frame** (`RenderFrame()` at `renderer_gogpu_frame.go:82`): The full
   render pipeline, executed in ordered phases:
   - **Phase 1 — Clear** (`:113-129`): Clear the screen or offscreen scene
     render target.
   - **Phase 2 — World BSP** (`renderWorld()` `:575` → `renderWorldInternal()`
     `world_render_gogpu.go:16`): Cluster compute dispatch for dynamic lights
     (`:99`), then the opaque/alpha-test/turbulent/sky world render pass.
   - **Phase 3 — Entities** (`renderEntities()` `:586`): Opaque brush entities,
     alias models, particles, sky brush entities, opaque liquid brush entities,
     external skybox overlay, then translucent water + translucent entities
     (collected at `:667` and `:671`, sorted and drawn by
     `renderGoGPUSortedTranslucentFaceRendersHAL` in `world_gogpu_translucent.go:601`),
     decal marks, sprites, translucent particles.
   - **Phase 4 — Viewmodel** (`:199`): First-person weapon model
     (`renderViewModelHAL` `world_gogpu_alias.go:593`).
   - **Phase 4b — Scene composite** (`:204`): Blit offscreen scene render
     target to the swapchain surface, applying underwater warp if active
     (`compositeSceneRenderTarget` `warpscale_gogpu.go:472`).
   - **Phase 4c — PolyBlend** (`:218`): Screen-space color tint
     (`renderPolyBlendHAL` `polyblend_gogpu.go:224`).
   - **Phase 5 — 2D Overlay** (`:233-234`): HUD/menu/console drawn CPU-side
     into a single texture and blitted to the screen
     (`flush2DOverlay` `renderer_gogpu_overlay.go:32`).

## Subpackages
| Subpackage | Purpose |
| --- | --- |
| `alias/` | Alias (MDL) model mesh interpolation, frame desc, pose blending. |
| `gogpu/` | gogpu input backend adapter — bridges gogpu window input to `input.Backend`. |
| `oit/` | Order-independent transparency shared helpers. |
| `scrap/` | Skyline bin-packing scrap allocator for small 2D UI textures. |
| `sky/` | External skybox cubemap face loading (PNG/TGA/JPG). |
| `surface/` | Surface texture animation chains (`SURF_DRAWTURB` linked lists). |
| `warpscale/` | Shared waterwarp FOV scale math. |
| `world/` | Pure-Go world logic: BSP types, lightmap samples, liquid alpha, fog, sky cvars, transforms. |
| `world/gogpu/` | gogpu-specific world helpers: brush entity draw building, translucent face planning, vertex packing, sprite/decal draw params, WGSL shaders for alias/sprite/decal. |

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
  - `world_shaders_gogpu.go` (world vertex/fragment + uniforms)
  - `world_compute_shaders_gogpu.go` (cluster dynamic light compute)
  - `world/gogpu/shaders.go` (alias, sprite, decal)
  - `polyblend_gogpu.go`, `warpscale_gogpu.go`, `overlay_composite_gogpu.go`,
    `particle_gogpu.go`
- **Multi-pass ordering**: The opaque/translucent ordering is subtle and the
  subject of ongoing bug investigations. Read `docs/diagnoses/` for context
  before changing render pass boundaries.
- **Dynamic Lighting**: Look at `internal/renderer/dynamic_light_pool.go` for
  the implementation of Quake's iconic dynamic lights, and
  `world_cluster_compute_gogpu.go` for the cluster-forward compute pipeline.
