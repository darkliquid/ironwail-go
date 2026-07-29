# Chapter 3: The Renderer — OpenGL Then, WebGPU Now

The renderer is the most divergent subsystem in `ironwail-go`. The C Ironwail
renderer is a modernized OpenGL path — core-profile shaders, UBOs, SSBOs,
indirect draws — but it is still fundamentally an immediate-mode, single-pass,
single-framebuffer OpenGL renderer. The Go port replaces it with a WebGPU
renderer built on the `gogpu` library: explicit pipelines, bind groups,
command-buffer submission, render passes, and an offscreen scene target.

This chapter compares the two architectures and explains the conceptual leaps
required to move from one to the other. Chapter 4 will walk through the render
stages one by one.

---

## The C renderer: `R_RenderView` and the single framebuffer

The C Ironwail renderer lives in `gl_*.c` and `r_*.c`. The entry point is
`R_RenderView`, which calls `R_SetupView` (`gl_rmain.c:964`) and then
`R_RenderScene` (`gl_rmain.c:1888`). The scene rendering order, from
`R_RenderScene`, is:

```c
void R_RenderScene (void)
{
    R_SetupScene ();
    R_Clear ();
    Fog_EnableGFog ();
    S_ExtraUpdate ();

    R_DrawEntitiesOnList (false);   // opaque world geometry + opaque entities
    R_DrawParticles (false);        // opaque particles
    Sky_DrawSky ();                 // sky
    R_DrawWater (false);            // opaque water (alpha == 1.0)
    R_BeginTranslucency ();         // set up translucent mode
    R_DrawWater (true);             // translucent water (alpha < 1.0)
    R_DrawEntitiesOnList (true);    // translucent entities
    R_DrawParticles (true);         // translucent particles
    R_EndTranslucency ();
    R_DrawViewModel ();             // first-person weapon
    R_ShowTris ();                  // debug wireframe overlay
}
```

Key characteristics of the C renderer:

### Single framebuffer, no intermediate submits

The water diagnosis doc captures the essential constraint: *"C Ironwail (OpenGL)
renders the entire frame to a single framebuffer within one `R_RenderView` call.
There are no intermediate command buffer submits."* [WaterDiag](#ref-waterdiag)
Everything — opaque, translucent, viewmodel, particles — draws into the same
framebuffer in sequence. Blending just works because the destination buffer
accumulates results naturally.

### Per-texture binding

The C renderer uses `GL_Bind` to bind individual textures to texture units
before each draw. `gl_texmgr.c` manages `gltexture_t` objects in a linked list,
with samplers created and deleted as a group. The world renderer in `r_world.c`
builds texture chains — linked lists of surfaces that share a texture — and
draws them in batches, but each batch still requires a `GL_BindTextures` call.
This is the classic OpenGL texture-binding pattern.

### OIT as an option

C Ironwail supports Order-Independent Transparency via weighted-blended
transparency (McGuire & Bavoil 2013). `R_BeginTranslucency` (`gl_rmain.c:1833`)
checks `R_GetEffectiveAlphaMode() == ALPHAMODE_OIT` and, if so, binds a
separate OIT framebuffer with accumulation and revealage textures, sets up
stencil state, and renders translucent objects into it. A final OIT resolve
pass composites the result back into the scene framebuffer. [WaterDiag](#ref-waterdiag)

### OpenGL state machine

The C renderer manipulates GL state directly:
`glEnable(GL_POLYGON_OFFSET_FILL)`, `glDisable(GL_STENCIL_TEST)`,
`glStencilFunc`, `glBlendFunc`, `glDepthMask`. State leaks between draws are
a constant hazard — forgetting to reset depth-write or blend mode after a
special pass produces visual corruption. Ironwail wraps this in `GL_SetState`
with `GLS_*` flags (`GLS_BLEND_ALPHA`, `GLS_NO_ZTEST`, `GLS_NO_ZWRITE`,
`GLS_CULL_NONE`), but the underlying model is a global mutable state machine.

---

## The Go renderer: WebGPU command buffers and explicit pipelines

The Go port's renderer lives in `internal/renderer/*_gogpu.go`. The entry point
is `RenderFrame()` at `renderer_gogpu_frame.go:82`. The renderer package doc
states its core design: *"abstracts the complexities of modern GPU APIs
(specifically WebGPU via the `gogpu` library) and provides a unified interface
for rendering 3D world geometry, 2D overlays, and special effects."*
[RendererDocs](#ref-rendererdocs)

### The CPU/GPU split

The learning plan explains the mental model: *"the CPU writes a 'recipe'
(commands) into a command buffer, the GPU executes it later. The CPU and GPU do
not share memory directly."* [LearningPlan](#ref-learningplan) This is visible in
the `DrawContext` struct (`renderer_gogpu.go:16`):

```go
type DrawContext struct {
    ctx               *gogpu.Context     // the underlying gogpu context
    gamma             float32
    renderer          *Renderer
    canvas            CanvasState
    sceneRenderActive bool
    sceneRenderTarget *wgpu.TextureView
    overlay           *overlay2D         // CPU-side 2D compositor buffer
}
```

The `Renderer` struct (same file, `:101`) holds all GPU-side resources:
pipelines, buffers, textures, bind groups. The CPU-side game logic never touches
pixels directly — it fills buffers and submits command encoders.

### The `Core`: headless-capable GPU initialization

The `Core` struct (`core_gogpu.go:46`) holds the wgpu Instance, Adapter,
Device, and Queue. `CoreConfig` specifies backend type, graphics API, validation,
and GPU preference. `DefaultCoreConfig()` returns `BackendGo`,
`GraphicsAPIAuto`, validation enabled, and `GPUPreferHighPerformance`. The `Core`
is used for both windowed and headless/screenshot rendering. This is a direct
consequence of WebGPU's design — you create an Instance, request an Adapter,
open a Device, get a Queue. There is no implicit context like OpenGL's
`wglMakeCurrent`. [LearningPlan](#ref-learningplan)

### Explicit pipeline objects

In OpenGL, pipeline state (shaders, blend mode, depth test/write, cull mode) is
set imperatively before each draw. In WebGPU, you create a
`RenderPipelineDescriptor` once — with vertex shader, fragment shader, blend
state, depth-stencil state, primitive topology, vertex buffer layout — and the
device compiles it into an immutable `RenderPipeline` object. At draw time, you
bind the pipeline and issue draws. State cannot leak between draws because there
is no mutable global state — each draw uses exactly the pipeline it was issued
under.

The Go port has separate pipelines for each pass type:
- **Opaque world** — depth-write on, depth-test on, no blending.
- **Alpha-test** — depth-write on, depth-test on, alpha-to-coverage or
  `discard` in fragment shader.
- **Translucent** — depth-write off, depth-test on, alpha blending.
- **Turbulent (water/lava/sky)** — UV warping in fragment shader, separate
  pipeline for translucent-turbulent.
- **Sky** — depth-write off, special two-layer scrolling texture.
- **Particles** — point-sprite billboards with procedural fragment shading.
- **PolyBlend** — fullscreen triangle, alpha blend.
- **Scene composite** — fullscreen triangle sampling the offscreen scene
  target, with underwater warp.
- **2D overlay** — fullscreen blit of the CPU-composited overlay texture.

### Bind groups: the resource binding model

In C OpenGL, textures and uniforms are bound to numbered texture units and
UBO binding points. In WebGPU, resources are organized into **bind groups** —
immutable bundles of resources (buffers, textures, samplers) bound to a
pipeline at specific `@group(N) @binding(M)` slots. The Go world shader declares:

```wgsl
@group(0) @binding(0) var<uniform> uniforms: Uniforms;
@group(0) @binding(1) var<uniform> materials: array<MaterialData, 256>;
@group(1) @binding(0) var worldSampler: sampler;
@group(1) @binding(1) var worldTexture: texture_2d<f32>;
@group(2) @binding(0) var worldLightmapSampler: sampler;
@group(2) @binding(1) var worldLightmap: texture_2d<f32>;
@group(3) @binding(0) var worldFullbrightSampler: sampler;
@group(3) @binding(1) var worldFullbrightTexture: texture_2d<f32>;
@group(4) @binding(0) var lightClusters: texture_3d<u32>;
@group(4) @binding(1) var<storage, read> dynamicLights: DynamicLights;
```

Each bind group is created once and rebound per draw. This is more verbose than
OpenGL's `GL_Bind`, but it eliminates the "forgot to bind a texture" class of
bug and allows the GPU driver to pre-validate resource compatibility.

---

## The 48-byte `WorldVertex` contract

The most important structural decision in the Go renderer is that **every world,
brush, alias, sprite, and decal vertex uses the same 48-byte layout**. The
vertex layout doc calls this the "three-place contract": the Go struct, the byte
packing functions, and the WGSL pipeline vertex layout must all agree.

```
Offset  Size  Field           Go type         WGSL type        Purpose
------  ----  -----           -------         ---------        -------
 0      12    Position        [3]float32      vec3<f32>        XYZ world position
12       8    TexCoord        [2]float32      vec2<f32>        UV into texture atlas
20       8    LightmapCoord   [2]float32      vec2<f32>        UV into lightmap array
28      12    Normal          [3]float32      vec3<f32>        Surface direction
40       4    LightmapLayer   float32         f32              Lightmap page index
44       4    MaterialID      uint32          u32              Materials buffer index
 48 bytes total (stride)
```

The Go struct lives in `internal/renderer/world/types.go`. Four packing
functions convert `WorldVertex` slices to flat byte arrays for GPU upload:
`createWorldVertexBuffer` (static world), `appendGoGPUWorldVertexBytes` (brush
entities), `VertexBytes` (sky brushes), and `aliasVertexBytesInto` (alias
models). The WGSL `VertexInput` struct in the shader must match. If any one
disagrees, the GPU reads vertex data at wrong offsets — textures scramble,
lighting artifacts appear, geometry disappears. [VertexLayout](#ref-vertexlayout)

In C, each vertex type (world, alias, sprite) has its own vertex format and
its own `glVertexAttribPointer` setup. The Go port unifies them into one
layout, which simplifies pipeline creation and buffer management at the cost
of some wasted bytes (a particle doesn't need a lightmap coordinate, but it
carries one anyway).

---

## Texture atlas + per-vertex material ID

This is one of the biggest conceptual departures from the C renderer.

### The problem

A Quake map has hundreds of small textures. In C OpenGL, the renderer binds
each texture individually before drawing the surfaces that use it. This works
because OpenGL's state machine tolerates frequent `GL_Bind` calls (though it is
slow). In WebGPU, binding individual textures per draw is impractical — bind
group limits and the overhead of creating/rebinding per-texture would cripple
performance. [LearningPlan](#ref-learningplan)

### The Go solution

The Go port packs all world textures into a single **texture atlas** — one large
2D texture (or texture array) with per-face UV offsets. Each `WorldVertex`
carries a `MaterialID` (uint32 at offset 44). The fragment shader looks up
`materials[materialID]` in a uniform buffer to find the atlas bounds and layer,
then samples the atlas at the correct sub-region. The materials buffer is updated
each frame for texture animation (water, lava, sky texture chains).

The atlas packer is a binary-tree packer in
`internal/renderer/world_atlas_gogpu.go` (`TextureAtlasNode`, `AtlasLayer`).
The materials buffer is a GPU uniform buffer with 256 entries of 32 bytes each
(32 = atlas bounds `vec4` + layer `f32` + padding). The `animateWorldMaterials`
function (`world_material_gogpu.go:24`) rewrites it each frame with the current
animation frame. A separate frame-1 buffer handles pressed button textures
(commit `aa17df6`).

### The open bug

The materials buffer is hardcoded to 256 entries, but `baseMaterials` is
allocated as `textureCount + 2` without clamping. When a map has more than 254
textures — as the qbj2 mod's `start` map does — a silent buffer overflow occurs.
This is the **texture atlas overflow** bug, currently open.
[MaterialsDiag](#ref-materialsdiag) It is a direct consequence of the atlas design:
the C renderer's per-texture binding has no such limit.

---

## Lightmap array with 1px padding and the Vulkan workaround

Quake's pre-baked lighting is stored in lightmaps — 16x16 texel blocks per
surface. The C renderer uploads these into a single large lightmap texture
(2D, or a 2D array in Ironwail's modernized path).

The Go port uses a **lightmap texture array** with 1px padding between pages
and a vertical-stacking workaround for Vulkan. The `uploadWorldLightmapArray()`
function (`world_lightmap_gogpu.go:11`) handles this. Lightstyles (animated
lighting like flickering lights) are evaluated per frame, and lightmap pages
whose style changed are rebuilt. The fragment shader samples
`worldLightmap` using the per-vertex `lightmapCoord` and `lightmapLayer`.
[LearningPlan](#ref-learningplan)

C never allocates lightmaps for `SURF_DRAWTURB` (water/lava) surfaces — they
are always fullbright. Ironwail added optional lit water via `r_litwater`. The
Go port samples the lightmap when `litWater > 0.5` in the WGSL uniform,
defaulting to `vec3<f32>(0.5)` (fullbright when multiplied by 2.0).
[WaterDiag](#ref-waterdiag)

---

## Cluster-forward dynamic lights via compute shader

This is a feature the C renderer **does not have**. The Go port implements a
cluster-forward dynamic lighting system:

- A **compute shader** (`world_cluster_compute_gogpu.go:13`) divides the camera
  frustum into a 3D grid of clusters (32×16×32 tiles). For each cluster, it
  computes which dynamic lights affect it and writes a bitmask.
- The **fragment shader** reads the cluster bitmask and iterates only the
  assigned lights, rather than looping all lights.
- Dynamic lights are gathered on the CPU in `internal/renderer/dynamic_light.go`
  and `dynamic_light_pool.go`, then uploaded to a storage buffer.
- The `Core.SetupFrameData()` function (`core_gogpu.go:158`) computes the
  z-scale/bias used for cluster z-slicing (log-depth).
- The compute dispatch happens before the world render pass in
  `renderWorldInternal()` at `world_render_gogpu.go:99`.

This is a modern rendering technique that goes beyond anything in C Ironwail's
OpenGL path. It exists because WebGPU's compute shader support makes it natural
to implement, and because the qbj3 stress maps push dynamic light counts that
would be prohibitively expensive with a naive "loop all lights" approach.
[LearningPlan](#ref-learningplan)

---

## OIT: weighted-blended transparency as an optional path

The Go port also implements Order-Independent Transparency as an optional path,
enabled by a cvar:

- **Mode selection**: `internal/renderer/oit_mode.go`.
- **Render path**: `internal/renderer/oit_render_path.go`.
- **Stub**: `internal/renderer/oit_stub.go`.
- **Shared helpers**: `internal/renderer/oit/`.

When enabled, the renderer replaces the sorted-translucent pass with a
weighted-blended one (accumulation texture + revealage texture), avoiding the
back-to-front sort. This mirrors C Ironwail's `ALPHAMODE_OIT` path, but the Go
implementation is a separate render path rather than a state switch within the
same pass. [LearningPlan](#ref-learningplan)

---

## Render order parity

Despite the architectural divergence, the Go renderer preserves the C render
order. The `RenderFrame()` function (`renderer_gogpu_frame.go:82`) executes
ordered phases:

| Phase | C function | Go function |
| --- | --- | --- |
| Clear | `R_Clear` | `:113-129` (clear or preserve scene target) |
| World BSP | `R_DrawWorld` → `R_DrawTextureChains` | `renderWorld` → `renderWorldInternal` (`world_render_gogpu.go:16`) |
| Opaque entities | `R_DrawEntitiesOnList(false)` | `renderEntities` (`:586`) |
| Translucent water | `R_DrawWater(true)` | within `renderWorldInternal` (translucent turbulent pipeline) |
| Translucent entities | `R_DrawEntitiesOnList(true)` | `renderGoGPUSortedTranslucentFaceRendersHAL` (`world_gogpu_translucent.go`) |
| Viewmodel | `R_DrawViewModel` | `renderViewModelHAL` (`world_gogpu_alias.go:593`) |
| Scene composite | (post-process via FBO) | `compositeSceneRenderTarget` (`warpscale_gogpu.go:472`) |
| PolyBlend | (inline in `R_SetupView`/`V_CalcBlend`) | `renderPolyBlendHAL` (`polyblend_gogpu.go:224`) |
| 2D overlay | `Draw_Console`/`SCR_UpdateScreen` | `flush2DOverlay` (`renderer_gogpu_overlay.go:32`) |

The key parity principle from the water diagnosis: *"no face is drawn both
opaquely and translucently. The split is by alpha value, not by pass."*
[WaterDiag](#ref-waterdiag) Both passes use the same framebuffer (in C) or the
same render pass (in Go). The Go port had to learn this the hard way — the
original architecture split the frame into multiple `queue.Submit()` calls,
and Vulkan drivers discarded the framebuffer contents between submits,
causing translucent water to blend over black instead of opaque geometry.
Commit `6802fc5` fixed this by drawing translucent liquid faces **within the
world render pass itself**, matching C's single-framebuffer model.
[WaterDiag](#ref-waterdiag)

---

## The offscreen scene render target

Unlike C, which renders directly to the window framebuffer (or a single FBO
for post-processing), the Go renderer uses an offscreen **scene render target**
that is later composited to the swapchain surface. This exists for the
**underwater warp** — a screen-space sinusoidal distortion applied when the
camera is in water. The scene composite pass
(`compositeSceneRenderTarget` at `warpscale_gogpu.go:472`) blits the offscreen
target to the swapchain, applying the warp if active. This adds an extra
render pass and texture allocation that C does not strictly need (C applies the
warp via OpenGL's `glScissor` and viewport tricks), but it is the clean
WebGPU way to do post-processing. [LearningPlan](#ref-learningplan)

---

## The 2D overlay: CPU compositing

The Go port composites the HUD, menu, and console into a single CPU-side
texture buffer (`overlay2D` in `DrawContext`) and blits it to the screen as one
GPU draw. This is different from C, which draws 2D elements via immediate-mode
GL calls. The `flush2DOverlay` function (`renderer_gogpu_overlay.go:32`) does
the blit. This approach reduces GPU draw calls for 2D (which can be hundreds of
text characters and pic draws per frame) to a single fullscreen blit.
Commit `3b9cfeb` pooled the overlay CPU buffer and cached the GPU texture to
avoid per-frame allocation. [RendererDocs](#ref-rendererdocs)

---

## What this means for parity

The architectural divergence is real and has cost. The bugs documented in the
`docs/diagnoses/` folder — water translucency, atlas overflow, lightmap
fallbacks, texture corruption on multi-layer atlas maps — are all consequences
of the architectural difference between OpenGL's implicit state model and
WebGPU's explicit pipeline model. Each fix is a lesson in how WebGPU's
constraints reshape the renderer:

- **Vulkan discard between submits** → draw translucent water within the world
  render pass (commit `6802fc5`).
- **Per-texture binding limit** → texture atlas with per-vertex material ID
  (commit `e99fad0`), which then introduced the 256-entry overflow bug.
- **Lightmap texture array mismatch** → `TextureViewDimension2DArray` fallback
  fix for faces without lightmap data.
- **Dynamic uniform buffer offset collision** → dynamic uniform buffer offsets
  for per-pass alpha values (commit `6802fc5`).

Chapter 4 walks through each render stage in detail, with file:line references
and the specific bugs encountered at each stage.

---

## References

<a id="ref-learningplan"></a>[LearningPlan] [`docs/RENDERER_LEARNING_PLAN.md`](../../docs/RENDERER_LEARNING_PLAN.md), ironwail-go repository.

<a id="ref-materialsdiag"></a>[MaterialsDiag] [`docs/diagnoses/qbj2_materials.md`](../../docs/diagnoses/qbj2_materials.md), ironwail-go repository.

<a id="ref-rendererdocs"></a>[RendererDocs] [`docs/internal/renderer.md`](../../docs/internal/renderer.md), ironwail-go repository.

<a id="ref-vertexlayout"></a>[VertexLayout] [`docs/VERTEX_LAYOUT.md`](../../docs/VERTEX_LAYOUT.md), ironwail-go repository.

<a id="ref-waterdiag"></a>[WaterDiag] [`docs/diagnoses/qbj2_water.md`](../../docs/diagnoses/qbj2_water.md), ironwail-go repository.


[ironwail]: https://github.com/andrei-drexler/ironwail
[gogpu]: https://github.com/gogpu/gogpu
[scratchapixel]: https://www.scratchapixel.com/
[webgpufundamentals]: https://webgpufundamentals.org/
[oto]: https://github.com/ebitengine/oto
