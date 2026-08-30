# Transparency Diagnosis, Test Suite, and Pipeline Rebuild Design

**Date:** 2026-08-30  
**Status:** Approved  
**Author:** Pair Programming Agent & User  
**Target Subsystem:** `internal/renderer` (GoGPU WebGPU 3D Engine & Parity Tooling)

---

## 1. Executive Summary & Problem Statement

Transparency (water, liquids, glass brush entities, translucent models, particles, and sprites) in the Go WebGPU engine has suffered from persistent ordering and compositing defects, specifically:
- Submerged geometry, moving entities, and props behind or inside water rendering incorrectly (either occluded, fully opaque, or incorrectly layered).
- Transparent surfaces viewed through/behind other semi-transparent surfaces (e.g. glass in front of water) failing depth tests or overwriting preceding transparent fragments.
- Inconsistent translucency handling where world liquids utilized OIT accumulation while other translucent entities (brush entities, alias models, sprites, particles) bypassed OIT and blended directly with separate render passes.

This document establishes:
1. A rigorous, step-by-step mechanical breakdown and flowchart of the canonical C Ironwail rendering pipeline.
2. An architectural specification for the clean rebuild of the Go WebGPU rendering pipeline adhering to strict WebGPU/browser constraints.
3. A dedicated four-quadrant transparency test map (`test_transparency.bsp`) and pure-Go synthetic procedural test suite for zero-movement cross-engine validation.
4. Comprehensive multi-tier diagnostic tooling (in-engine intermediate pass PNG dumpers, interactive cvar pass isolation, and scripted RenderDoc/gfxrecon GPU trace automation).

---

## 2. Canonical C Ironwail Pipeline Mechanics

C Ironwail (`gl_rmain.c`, `r_world.c`, `r_brush.c`) executes a linear rendering sequence inside `R_RenderView()` &rarr; `R_RenderScene()` targeting `scene_fbo`:

```
+-----------------------------------------------------------------------------------+
| 1. Scene Setup: Bind scene_fbo, glClear(COLOR | DEPTH | STENCIL), Fog_EnableGFog |
+-----------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
| 2. Opaque Geometry Passes (DepthWrite = true, GLS_CULL_BACK):                     |
|    • R_DrawEntitiesOnList(false) -> Opaque brush models (world + bmodels),       |
|                                    opaque alias models, opaque sprites            |
|    • R_DrawParticles(false)                                                       |
|    • Sky_DrawSky()                                                                |
|    • R_DrawWater(false) -> Only liquid faces with alpha == 1.0 (opaque liquids)   |
+-----------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
| 3. R_BeginTranslucency() [OIT Mode]:                                              |
|    • Bind oit_fbo (Target 0: accum_tex RGBA16F, Target 1: revealage_tex R8)       |
|    • Clear accum to [0,0,0,0], revealage to [1,1,1,1]                            |
|    • Shared scene depth buffer bound in read-only mode (GLS_NO_ZWRITE)            |
|    • Stencil test enabled (write stencil = 2)                                     |
+-----------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
| 4. Translucent Geometry Accumulation (All translucent types write to OIT MRT):   |
|    • R_DrawWater(true) -> Translucent liquid faces (alpha < 1.0)                  |
|    • R_DrawEntitiesOnList(true) -> Translucent brush entities & alias models      |
|    • R_DrawParticles(true) -> Translucent particles & sprites                     |
+-----------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
| 5. R_EndTranslucency() [OIT Mode]:                                                |
|    • Bind scene_fbo                                                               |
|    • Stencil test: Equal 2                                                        |
|    • Fullscreen triangle with oit_resolve shader (GLS_BLEND_ALPHA: SrcAlpha/1-Src)|
+-----------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
| 6. View Model & Post-Processing:                                                  |
|    • R_DrawViewModel() -> Player weapon drawn over resolved scene                 |
|    • GL_PostProcess() -> Underwater screen warp + Gamma & Contrast curve blit     |
|    • 2D UI / HUD / Console rendered on top                                        |
+-----------------------------------------------------------------------------------+
```

### Key Mechanical Invariants in C Ironwail
1. **Single-Pass Translucency Accumulation:** All translucent surfaces—whether BSP liquids, bmodel glass, alias monsters, or smoke particles—accumulate together into the OIT buffers. No translucent entity draws directly to `scene_fbo` while OIT is active.
2. **View Model Order:** Weapon view models render *after* the OIT resolve, guaranteeing they are never tinted or occluded by water or translucency in the world behind them.
3. **Liquid Alpha Segmentation:** Liquid surfaces are strictly partitioned: faces with `alpha == 1.0` render in the opaque pass with depth writes enabled; faces with `alpha < 1.0` render exclusively in the translucency accumulation pass with depth writes disabled.

---

## 3. Go WebGPU Unified Frame Graph Architecture

In WebGPU, render pass attachments are immutable for the duration of a `wgpu.RenderPass`, and attachments cannot be sampled inside the pass that wrote them. The rebuilt Go pipeline maps C's sequence into five explicit, non-overlapping passes recorded into a single shared `wgpu.CommandEncoder`:

```
                       +-----------------------------------+
                       | dc.beginFrameGraph()              |
                       | (Pre-allocate uniform ring buffer)|
                       +-----------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
| PASS 1: OPAQUE SCENE PASS                                                         |
| • Color Attachment: SceneRenderTargetView (RGBA16F or RGBA8) [LoadOp: Clear]      |
| • Depth Attachment: WorldDepthTextureView (Depth32Float)     [LoadOp: Clear (1.0)]|
| • Pipeline Depth State: DepthWriteEnabled = true, DepthCompare = LessEqual        |
| • Draw Calls:                                                                     |
|   1.1 World Opaque BSP Surfaces (batched indices by lightmap/texture)             |
|   1.2 World Sky (Skybox mesh / Dome / Cloud layers)                               |
|   1.3 Opaque Brush Entities (doors, elevators, triggers)                          |
|   1.4 Opaque Alias Models (players, monsters, items)                              |
|   1.5 Opaque Liquids (water/slime/lava where alpha == 1.0)                        |
|   1.6 Opaque Particles                                                            |
+-----------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
| PASS 2: TRANSLUCENT ACCUMULATION PASS (OIT MRT)                                   |
| • Color Attachment 0: OITAccumTextureView  (RGBA16F)  [LoadOp: Clear [0,0,0,0]]   |
| • Color Attachment 1: OITRevealTextureView (R8Unorm)  [LoadOp: Clear [1,1,1,1]]   |
| • Depth Attachment:   WorldDepthTextureView           [LoadOp: Load, Write=false] |
| • Blend Equations:                                                                |
|   - Target 0 (Accum):  Color=(One, One, Add), Alpha=(One, One, Add)               |
|   - Target 1 (Reveal): Color=(Zero, OneMinusSrcColor, Add), Alpha=(Zero, 1-Src, +)|
| • Draw Calls:                                                                     |
|   2.1 World Translucent Liquids (Water, Slime, Lava, Teleporters where alpha < 1) |
|   2.2 Translucent Brush Entities (Glass windows, func_walls, submerged brushes)   |
|   2.3 Translucent Alias Entities (Cloaked player, translucent projectiles)        |
|   2.4 Translucent Particles, Sprites & Decals                                     |
| • (Fallback for r_oit 0): Directly draws 2.1-2.4 to SceneRT in CPU-sorted order   |
+-----------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
| PASS 3: OIT RESOLVE & VIEW MODEL PASS                                             |
| • Color Attachment: SceneRenderTargetView [LoadOp: Load, StoreOp: Store]          |
| • Depth Attachment: None (or local viewmodel depth range)                         |
| • Draw Calls:                                                                     |
|   3.1 Fullscreen OIT Resolve Triangle:                                            |
|       Samples OITAccumTexture & OITRevealTexture via OITResolvePipeline           |
|       Blend: GLS_BLEND_ALPHA (SrcAlpha, OneMinusSrcAlpha, Add)                    |
|   3.2 View Weapon Model (R_DrawViewModel parity)                                  |
+-----------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
| PASS 4: SCENE COMPOSITE & POST-PROCESS PASS                                       |
| • Color Attachment: SwapchainTextureView [LoadOp: Clear]                          |
| • Draw Calls: Fullscreen Blit from SceneRenderTargetView applying:                |
|   4.1 Underwater Sinusoidal Warp (when camera leaf contents is liquid)            |
|   4.2 Fullscreen Polyblend (pain flash, bonus item pick-up color tints)           |
|   4.3 Gamma & Contrast Power Curve: pow(color.rgb * contrast, vec3(gamma))        |
+-----------------------------------------------------------------------------------+
                                         |
                                         v
+-----------------------------------------------------------------------------------+
| PASS 5: 2D OVERLAY PASS                                                           |
| • Color Attachment: SwapchainTextureView [LoadOp: Load, StoreOp: Store]           |
| • Draw Calls: gogpu 2D HUD, Status Bar, Menu Overlays, Developer Console          |
+-----------------------------------------------------------------------------------+
                                         |
                                         v
                       +-----------------------------------+
                       | dc.endFrameGraph()                |
                       | (Single queue.Submit())           |
                       +-----------------------------------+
```

---

## 4. Four-Quadrant Transparency Test Map (`test_transparency`)

To provide deterministic validation without requiring player navigation, a dedicated Quake map (`test_transparency.map` & `test_transparency.bsp`) is designed such that all test vignettes are in direct view from the initial player spawn point (`origin (0, 0, 0)`, `angles (0, 0, 0)`, 90° FOV):

```
+------------------------------------+------------------------------------+
| QUADRANT 1: Top-Left               | QUADRANT 2: Top-Right               |
| Submerged Geometry & Dynamic BModel| Stacked Multi-Layer Translucency   |
| • Recessed water pool (alpha 0.6)  | • Front: Translucent glass (a=0.5) |
| • Oscillating elevator (func_door) | • Middle: Suspended water cube     |
|   dipping in/out of liquid pool    | • Back: Moving entity (func_train) |
| • Submerged progs/armor.mdl        | • Rear: High-contrast opaque wall  |
| • Lightmap gradient (lit water test| • Tests dynamic depth behind multi-|
|   vs unlit turbulent shader)       |   layer transparent surfaces       |
+------------------------------------+------------------------------------+
| QUADRANT 3: Bottom-Left            | QUADRANT 4: Bottom-Right           |
| Multi-Liquid & Teleporter Overlap  | Submerged Particles & Sprites      |
| • Slime pool (r_slimealpha 0.7)    | • Submerged progs/flame.spr sprite |
| • Lava pool (r_lavaalpha 1.0)      | • Particle fountain on moving      |
| • Teleporter field (r_telealpha 0.4|   platform bursting from underwater|
| • Tests separate per-liquid alpha  |   into open air                    |
|   uniforms & material pipeline bind| • Tests particle depth-testing and |
|   states without state leakage     |   OIT accumulation inside liquids  |
+------------------------------------+------------------------------------+
```

### Automated Parity Harness & Synthetic Suite
1. **Parity Harness Task (`mise run parity-transparency`):**
   - Launches C Ironwail (`ironwail -basedir <QUAKE_DIR> +map test_transparency`) and Go Ironwail (`ironwailgo -basedir <QUAKE_DIR> +map test_transparency`) at 1280x720.
   - Captures frame 0 / frame 30 screenshots and produces side-by-side PNG diffs, computing PSNR and SSIM scores.
2. **Pure-Go Synthetic BSP Test Suite:**
   - Extends [`CreateSyntheticWorldModel()`](file:///home/darkliquid/Projects/ironwail-go/internal/server/synthetic_bsp_helper.go) to generate in-memory BSP trees containing liquid faces, glass brush entities, and depth-testing geometry.
   - Executes headless raster readback in `internal/renderer/` tests without requiring external game data or pak files.

---

## 5. Diagnostic Tooling & GPU Trace Automation

### 5.1 In-Engine Intermediate Attachment Dumper (`r_dump_passes`)
Controlled via console cvar `r_dump_passes 1` or CLI flag `-screenshot-passes <prefix>`:
- Performs GPU staging buffer copy for every active attachment at the end of the frame, saving high-resolution PNGs:
  - `01_opaque_scene.png`: `SceneRenderTarget` after Pass 1.
  - `02_scene_depth.png`: Normalized linearized [0.0, 1.0] depth map from `WorldDepthTextureView`.
  - `03_oit_accum_rgb.png` & `03_oit_accum_a.png`: HDR color and weight buffers from `OITAccumTexture`.
  - `04_oit_reveal.png`: Normalized grayscale revealage from `OITRevealTexture`.
  - `05_resolved_scene.png`: `SceneRenderTarget` immediately following Pass 3 resolve.
  - `06_viewmodel_scene.png`: `SceneRenderTarget` after viewmodel rendering.
  - `07_postprocessed.png`: `SwapchainTextureView` after water warp and gamma/contrast.
  - `08_final_swapchain.png`: Complete presented frame with 2D overlay.

### 5.2 Interactive Live Pass Isolation (`r_pass_isolate`)
Real-time console cvar for viewport isolation without file dumps:
- `0`: Normal rendering.
- `1`: Visualize raw `OITAccum` target.
- `2`: Visualize raw `OITReveal` target.
- `3`: Visualize linearized Depth buffer.
- `4`: Opaque scene only (bypassing translucency and UI).
- `5`: Translucency accumulation only (over black background).

### 5.3 Scripted GPU Trace Automation (RenderDoc & gfxrecon)
- `tasks/gpu-trace/trace-vulkan.sh`: Captures Vulkan API traces of ironwail-go using `gfxrecon-capture.py` and RenderDoc layer.
- `tasks/gpu-trace/trace-opengl.sh`: Captures OpenGL API traces of C Ironwail using RenderDoc.
- `tasks/gpu-trace/trace-compare.py`: Automated CLI script parsing trace metadata and outputting a side-by-side markdown table of draw counts, pipeline blend equations, depth write flags, and attachment formats.

---

## 6. WebGPU & Browser Resource Constraints

1. **Strict 4-Bind-Group Layout:**
   - `Group 0`: Per-frame / Per-draw Scene & Entity Uniforms (dynamic buffer offset).
   - `Group 1`: Diffuse Texture Atlas / Array.
   - `Group 2`: Lightmap Texture Array.
   - `Group 3`: Fullbright Texture Atlas / Dynamic lights / Material animations.
2. **Dynamic Uniform Ring Buffer:**
   - Replaces per-draw bind group allocations with a single 256-byte aligned dynamic uniform buffer ring allocated per frame. Draw calls pass their uniform slice offset via `SetBindGroup(0, uniformBindGroup, []uint32{offset})`.
3. **Elimination of Mid-Frame Swapchain Discards:**
   - All 3D rendering targets `SceneRenderTarget` (RGBA16F/RGBA8). The swapchain texture is only touched during Pass 4 (Post-process blit) and Pass 5 (2D Overlay), preventing Vulkan load-op discard artifacts.

---

## 7. Verification and Acceptance Criteria

1. **Parity Metric:** Side-by-side screenshot comparison on `test_transparency.bsp` between C Ironwail and Go Ironwail achieves SSIM &ge; 0.95 and PSNR &ge; 35 dB across all four quadrants.
2. **Stacked Translucency Test:** Quadrant 2 validates that the moving brush entity, suspended water block, and front glass brush all remain visible with correct depth-testing and zero popping/culling.
3. **Automated Unit Tests:** `go test ./internal/renderer/...` passes with 100% success on synthetic multi-liquid BSP models in headless mode.
4. **Diagnostic Tooling:** `r_dump_passes 1` successfully generates all 8 intermediate stage PNGs.
5. **Zero WebGPU Validation Errors:** No Vulkan validation layer warnings or WebGPU pipeline layout errors emitted.
