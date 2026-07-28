# Building the Ironwail-Go WebGPU Renderer — A Stage-by-Stage Learning Plan

This document is a curriculum for a reader who knows Go but **does not know
graphics programming or WebGPU**. It walks from "what is a GPU?" all the way to
"how does Ironwail-Go render a full Quake frame?" Each stage maps to a real
part of this codebase, cites teaching material from
[scratchapixel.com](https://www.scratchapixel.com/) (computer graphics theory)
and [webgpufundamentals.org](https://webgpufundamentals.org/) (WebGPU API
practice), and ends with a concrete build-it-yourself milestone you can verify
against the Ironwail-Go source.

The goal is not to read the renderer top-to-bottom once. It is to **rebuild a
smaller version of it, stage by stage**, until the full system is obvious.

---

## How to use this plan

- Work stages in order. Later stages assume the vocabulary and code from
  earlier ones.
- Each stage has three parts:
  1. **Concept** — theory with citations to read first.
  2. **Ironwail-Go reality** — where this lives in this repo, with `file:line`
     references.
  3. **Milestone** — a small thing to build or modify to prove you understand
     it. Build it in a scratch directory, then compare to the real code.
- Do not skip the milestones. Graphics is a "you understand it by making pixels
  appear" discipline. Reading alone is insufficient.
- **`file:line` references are a snapshot of 2026-07-27.** The renderer is
  under active development. When a line number no longer matches, use the
  symbol/function name (the part before the line number) with your editor's
  jump-to-symbol or `grep` to find the current location. The function names
  are more stable than the line numbers.

---

## Stage 0 — Mental Model: What a GPU Is and What "Rendering" Means

### Concept

Read, in this order:

1. **scratchapixel** — [*Introduction / "What is computer graphics?"*](https://www.scratchapixel.com/lessons/introduction-to-computer-graphics/what-is-computer-graphics.html)
   and [*Rasterization: Practical Implementation*](https://www.scratchapixel.com/lessons/3d-basic-rendering/rasterization-practical-implementation).
   Goal: understand that a 3D renderer's job is to turn 3D geometry
   (triangles) plus a camera into a 2D image (pixels), and that the GPU is a
   massively parallel machine built to do this fast.
2. **scratchapixel** — [*Ray Tracing vs Rasterization*](https://www.scratchapixel.com/lessons/3d-basic-rendering/rasterization-practical-implementation).
   Quake is **rasterization**, not ray tracing. Know the difference.
3. **webgpufundamentals** — [*WebGPU Fundamentals* intro article](https://webgpufundamentals.org/webgpu/lessons/webgpu-fundamentals.html).
   This establishes the WebGPU mental model: the CPU writes a "recipe" (commands)
   into a command buffer, the GPU executes it later. The CPU and GPU do **not**
   share memory directly.

### Key vocabulary to lock in

| Term | One-line meaning |
| --- | --- |
| Vertex | A point in 3D space (+ extra data like color, texture coordinate) |
| Triangle | Three vertices; the unit of geometry the GPU rasterizes |
| Fragment / pixel | A potential pixel produced by rasterizing a triangle |
| Texture | A 2D array of texels (color values) the GPU can sample |
| Buffer | A flat byte array on the GPU holding vertices, indices, or uniform data |
| Shader | A small program running on the GPU (vertex stage, fragment stage) |
| Pipeline | The full "recipe" wired up: shaders + buffers + state |
| Bind group | The bundle of GPU resources (buffers, textures) bound to a pipeline |
| Render pass | One "draw these things to this image" operation |

### Ironwail-Go reality

The CPU/GPU split is visible in the `DrawContext` struct:
`internal/renderer/renderer_gogpu.go:16`. The `Renderer` struct (same file,
line 101) holds **all the GPU-side resources** (pipelines, buffers,
textures). The CPU-side game logic never touches pixels directly; it only
fills buffers and submits command encoders.

### Milestone

Install a minimal WebGPU "clear the screen to a color" program. Use
[webgpufundamentals' "Getting a WebGPU adapter and device"](https://webgpufundamentals.org/webgpu/lessons/webgpu-fundamentals.html)
as the template. In this repo's terms, you are reproducing what
`Core.InitHeadless()` does at `internal/renderer/core_gogpu.go:68`: create an
Instance, request an Adapter, open a Device, get a Queue.

**Done means:** a window (or headless canvas) shows a solid color, and you can
explain *in your own words* why the color is chosen on the CPU but filled on
the GPU.

---

## Stage 1 — The Triangle: Buffers, Shaders, Pipelines

### Concept

1. **webgpufundamentals** — [*WebGPU from Squares*](https://webgpufundamentals.org/webgpu/lessons/webgpu-getting-started.html)
   (the "draw a triangle" lesson). This is the canonical first program.
   Understand: vertex buffer → vertex shader → rasterizer → fragment shader →
   render target.
2. **scratchapixel** — [*Rasterization: Practical Implementation*](https://www.scratchapixel.com/lessons/3d-basic-rendering/rasterization-practical-implementation/rasterization-stage.html),
   specifically the rasterization and vertex transform stages. This gives you
   the *why* behind the *how* of the WebGPU lesson.
3. **webgpufundamentals** — [*WGSL — the shading language*](https://webgpufundamentals.org/webgpu/lessons/webgpu-wgsl.html).
   WGSL is the shader language WebGPU uses. Ironwail-Go writes all shaders in
   WGSL as Go string constants.

### Ironwail-Go reality

The simplest real shader in this codebase is the **polyblend** fullscreen
triangle:

- Shader source: `polyBlendVertexShaderWGSL` and
  `polyBlendFragmentShaderWGSL` at `internal/renderer/polyblend_gogpu.go:15`
  and `:34`.
- Pipeline setup: `ensurePolyBlendResourcesLocked()` in the same file around
  line 83.
- Per-frame use: `renderPolyBlendHAL()` at `polyblend_gogpu.go:224`, called from
  `RenderFrame()` at `renderer_gogpu_frame.go:218`.

This is a single hardcoded triangle that covers the screen and tints it a solid
color (Quake's "polyblend" — used for underwater/palette flash effects). It is
the perfect "hello triangle" to study because it has no vertex buffer (the
positions are baked into the shader), no textures, and a trivial fragment
shader.

For a real vertex-buffer example, study the **particle** pipeline:
- Shader: `particleVertexShaderWGSL` / `particleFragmentShaderWGSL` at
  `internal/renderer/particle_gogpu.go:20` and `:75`.
- Pipeline: `ensureParticleResourcesLocked()` at `particle_gogpu.go:148`.

### Milestone

Build a program that draws a single colored triangle using a vertex buffer and
a WGSL vertex+fragment shader. Then change it to draw a fullscreen triangle
that fills the screen with a color you choose from CPU-side uniform data (a
small buffer copied up each frame). That second version is, structurally,
exactly the Ironwail-Go polyblend pipeline minus the alpha blending.

**Done means:** you can write, from memory, a WGSL vertex shader that takes a
`@location(0)` position and a fragment shader that outputs a `@location(0)`
color, and you can explain what a `bind group` and `pipeline` are.

---

## Stage 2 — Matrices, the Camera, and "3D to 2D"

### Concept

1. **scratchapixel** — [*Ray-Tracing: Generating Camera Rays*](https://www.scratchapixel.com/lessons/3d-basic-rendering/ray-tracing-generating-camera-rays.html)
   and, more importantly, [*Rasterization: Viewing Frustum, Perspective
   Projection*](https://www.scratchapixel.com/lessons/3d-basic-rendering/rasterization-practical-implementation/perspective-projection.html).
   Goal: deeply understand that the **view matrix** transforms world space into
   camera/eye space, and the **projection matrix** transforms eye space into
   clip space (the GPU's normalized cube). Together they are the "VP matrix".
2. **scratchapixel** — [*Transformations*](https://www.scratchapixel.com/lessons/mathematics-of-quantum-mechanics/geometry/transformations.html)
   (matrices, point/vector transforms).
3. **webgpufundamentals** — [*Matrix Math*](https://webgpufundamentals.org/webgpu/lessons/webgpu-matrix-math.html)
   and [*Matrix Stacks / Scene Graphs*](https://webgpufundamentals.org/webgpu/lessons/webgpu-matrix-graph.html).
   These show the WebGPU-side practice of packing matrices into a uniform
   buffer and reading them in WGSL.

### Ironwail-Go reality

- Camera state and VP computation: `internal/renderer/camera.go`.
- The VP matrix is packed into the world uniform buffer:
  `worldUniformsWGSL` at `internal/renderer/world_shaders_gogpu.go:10` defines
  the WGSL `struct Uniforms` that holds `viewProjection: mat4x4<f32>`.
- Uniform buffer packing: `internal/renderer/renderer_gogpu_uniforms.go`.
- The world vertex shader multiplies `position` by this matrix:
  `worldVertexShaderWGSL` at `world_shaders_gogpu.go:27`.

### Milestone

Extend your triangle program: place three triangles at different positions in
"world space" and add a camera you can move with the keyboard. Build a
`mat4x4` view + projection matrix, put it in a uniform buffer, and use it in
the vertex shader. Now you have a "3D scene" with one object type.

**Done means:** you can explain the difference between world space, eye space,
clip space, and screen space, and you can write a WGSL vertex shader that
transforms `position` by a `mat4x4<f32>` uniform.

---

## Stage 3 — Loading Real Geometry: The BSP World

### Concept

1. **scratchapixel** — [*Polygon Meshes*](https://www.scratchapixel.com/lessons/3d-basic-rendering/building-a-scene/polygon-mesh.html)
   and [*Rasterization: Triangle
   Rasterization*](https://www.scratchapixel.com/lessons/3d-basic-rendering/rasterization-practical-implementation/rasterization-stage.html).
   Understand that a 3D model is a list of vertices + a list of index triples.
2. **webgpufundamentals** — [*Storage
   Buffers*](https://webgpufundamentals.org/webgpu/lessons/webgpu-storage-buffers.html)
   (how to feed a lot of geometry to the GPU) and
   [*Vertex Buffers*](https://webgpufundamentals.org/webgpu/lessons/webgpu-vertex-buffers.html).
3. Read the BSP package doc: `docs/internal/bsp.md` and the `internal/bsp`
   package. A Quake `.bsp` file is a list of vertices, edges, faces, planes,
   and leaves. You do not need to parse it yourself — Ironwail-Go does that for
   you — but you must understand that the world is "one big mesh with extra
   metadata".

### Ironwail-Go reality

This is the largest stage. The world pipeline is the heart of the renderer.

- **BSP → GPU vertex upload**: `UploadWorld()` at
  `internal/renderer/world_upload_gogpu.go:18` orchestrates everything.
- **Vertex construction** (the bridge between BSP data and the GPU vertex
  format): `WorldGeometry` in `internal/renderer/world_geometry_gogpu.go`.
- **The vertex layout contract**: read `docs/VERTEX_LAYOUT.md`. It documents
  the 48-byte `WorldVertex` — the single struct that flows from Go → byte
  packer → WGSL `@vertex` input. This is the most important document in the
  renderer for a learner.
- **Byte packing** (how Go structs become GPU bytes):
  `appendGoGPUWorldVertexBytes` in `internal/renderer/world_gogpu.go:150`.
- **Pipeline creation**: `createWorldPipeline()` and friends at
  `internal/renderer/world_pipelines_gogpu.go:13`.
- **The render pass itself**: `renderWorldInternal()` at
  `internal/renderer/world_render_gogpu.go:16`.
- **Shaders**: `worldVertexShaderWGSL` and `buildWorldFragmentShaderWGSL()` at
  `internal/renderer/world_shaders_gogpu.go:27` and `:83`.

### Milestone

Build a program that loads a list of triangles from a JSON file (vertex
positions + colors), uploads them to a GPU vertex buffer + index buffer, and
renders them with a camera. This is a "static mesh viewer". Then, make the
vertex struct carry a texture coordinate (UV) alongside position — you now have
the shape of `WorldVertex`, just smaller.

**Done means:** you can explain what a vertex buffer layout is, why the 48-byte
stride matters, and what an index buffer does. You can read
`docs/VERTEX_LAYOUT.md` and follow the contract end-to-end.

---

## Stage 4 — Textures and the Texture Atlas

### Concept

1. **scratchapixel** — [*Texture Mapping*](https://www.scratchapixel.com/lessons/3d-basic-rendering/rasterization-practical-implementation/texturing-perspective.html)
   — understand UV coordinates, texture sampling, and why perspective-correct
   interpolation matters.
2. **webgpufundamentals** — [*Textures*](https://webgpufundamentals.org/webgpu/lessons/webgpu-textures.html)
   and [*Sampler
   Parameters*](https://webgpufundamentals.org/webgpu/lessons/webgpu-samplers.html).
3. **webgpufundamentals** — [*Atlas / Texture
   Arrays*](https://webgpufundamentals.org/webgpu/lessons/webgpu-textures.html#texture-atlas)
   concept. A Quake map has hundreds of small textures. Binding each one
   individually is impossible (bind group limits). Solution: pack them all into
   one big image (an **atlas**) and use UV offsets to pick which texture each
   face uses.

### Ironwail-Go reality

- **Atlas packer**: the binary-tree packer in
  `internal/renderer/world_atlas_gogpu.go` (`TextureAtlasNode`,
  `AtlasLayer`).
- **Atlas upload + GPU texture array**: `world_resources_gogpu.go` (search for
  atlas creation).
- **Per-face texture index**: each `WorldVertex` carries a `texture_index`
  field (see `docs/VERTEX_LAYOUT.md`); the fragment shader uses it to sample
  the right slice of the atlas array.
- **Texture animation**: Quake textures animate (water, lava, sky). The
  animation chain lives in `internal/renderer/surface/surface.go`
  (`BuildTextureAnimations`), and the current frame is written into a
  **materials buffer** updated each frame: `world_material_gogpu.go`
  (`animateWorldMaterials` at line 24, `updateWorldMaterialsBuffer` at line
  61).
- **Fragment shader sampling**: `buildWorldFragmentShaderWGSL()` in
  `world_shaders_gogpu.go:83` — read how it samples `material_texture` using
  the per-vertex UV and a `texture_index`.

### Milestone

Extend your mesh viewer: give each triangle a texture, and pack 4 small
textures into one 256×256 atlas. Use a per-vertex `texture_index` to select
which slice of the atlas each triangle samples. Then animate one texture
(swap its frame every second) by updating a small "materials" buffer on the
CPU each frame — exactly the Ironwail-Go pattern.

**Done means:** you can explain the difference between a texture, a sampler, a
texture atlas, and a texture array, and why Quake needs the atlas pattern.

---

## Stage 5 — Lightmaps: Pre-Baked Lighting

### Concept

1. **scratchapixel** — [*Shading*](https://www.scratchapixel.com/lessons/3d-basic-rendering/rasterization-practical-implementation/lighting-and-shading.html)
   and [*Global Illumination and Path
   Tracing*](https://www.scratchapixel.com/lessons/3d-basic-rendering/global-illumination-path-tracing)
   — understand that Quake does **not** compute lighting at runtime; it is
   pre-baked offline by the map compiler (`qrad`) and stored as a
   **lightmap**: a small grayscale texture per face.
2. **webgpufundamentals** — [*Multi-texturing*](https://webgpufundamentals.org/webgpu/lessons/webgpu-textures.html)
   — the fragment shader samples **two** textures (the material and the
   lightmap) and multiplies them.

### Ironwail-Go reality

- **Lightmap sample extraction**: `internal/renderer/lightmap_samples.go` and
  `internal/renderer/world/lightmap_samples.go`.
- **Lightmap page stacking + GPU upload**: `uploadWorldLightmapArray()` at
  `internal/renderer/world_lightmap_gogpu.go:11`. Note the 1px padding and the
  "stack vertically" workaround for Vulkan.
- **Lightstyles** (animated lighting, e.g. flickering lights): the renderer
  evaluates lightstyle values per frame and rebuilds lightmap pages whose
  style changed — see the lightstyle handling around
  `renderer_gogpu_worldstate.go` and the `lightstyle_values` field in the
  world uniforms.
- **Fragment shader lightmap sampling**: `buildWorldFragmentShaderWGSL()`
  samples `lightmap_texture` and multiplies it into the final color.

### Milestone

Add a second texture to your viewer: a per-triangle "lightmap". Bake a simple
grayscale lightmap by hand for 3 triangles (e.g. one bright, one dim, one
half-lit), sample it in the fragment shader, and multiply it with the material
texture. Then add a "lightstyle": a single float you animate on the CPU and
multiply into one triangle's lightmap sample to make it flicker.

**Done means:** you can explain why Quake uses lightmaps instead of real-time
lighting, and what a "lightstyle" is.

---

## Stage 6 — Visibility: BSP, PVS, and "Don't Draw What You Can't See"

### Concept

1. **scratchapixel** — [*Rasterization: Clipping and Culling*](https://www.scratchapixel.com/lessons/3d-basic-rendering/rasterization-practical-implementation/clipping-and-culling.html).
2. The Quake BSP concept: read the BSP doc (`docs/internal/bsp.md`). The world
   is a tree of splitting planes; the leaves are convex "rooms". Each leaf has
   a **PVS** (Potentially Visible Set) bitmask saying which other leaves can
   be seen from it.
3. **Frustum culling** and **occlusion** as concepts — scratchapixel covers
   these under the rasterization practical implementation series.

### Ironwail-Go reality

- **BSP leaf lookup + PVS**: `WorldRenderData` in
  `internal/renderer/world.go:57` is a passive data holder; actual visible
  face selection is done by `selectVisibleWorldFaces` in
  `internal/renderer/world_shared.go:172` (and called from
  `world_render_gogpu.go:333`).
- **Face classification** (opaque, alpha-test, translucent, turbulent/sky):
  helpers in `internal/renderer/world_shared.go`.
- **The visibility scratch buffer**: same file.
- **What gets drawn**: `renderWorldInternal()` at
  `internal/renderer/world_render_gogpu.go:16` only draws the faces that passed
  visibility — this is why Quake can render huge maps at 60fps.

### Milestone

This stage is more "understand" than "build". Add a "visibility" concept to
your viewer: divide your scene into 4 "rooms" (quadrants), and only draw the
triangles in the current room + the one you can see into. Hardcode the visibility
table. Observe the draw call count drop.

**Done means:** you can explain why a BSP exists, what a PVS is, and why
visibility culling is the single most important optimization in a Quake-style
renderer.

---

## Stage 7 — Depth Testing and the Opaque/Transparent Ordering Problem

### Concept

1. **scratchapixel** — [*Rasterization: Depth Buffer /
   Z-Buffer*](https://www.scratchapixel.com/lessons/3d-basic-rendering/rasterization-practical-implementation/z-buffer-depth.html).
   The depth buffer is how the GPU knows which triangle is "in front".
2. **webgpufundamentals** — [*Transparency, Sorting, and the Depth
   Buffer*](https://webgpufundamentals.org/webgpu/lessons/webgpu-transparency.html)
   — the classic problem: opaque objects use depth testing (draw in any
   order), but translucent objects must be **sorted back-to-front** and drawn
   with depth-write off.
3. Read `docs/diagnoses/qbj2_water.md` — a real example of how hard the
  opaque/translucent ordering problem is in practice, and how it bit this
  codebase. The issue is resolved but the diagnosis record is a valuable
  case study.

### Ironwail-Go reality

- **Depth texture**: `createWorldDepthTexture()` at
  `internal/renderer/world_depth_gogpu.go:21`.
- **Multiple pipelines for blend/depth state**: `world_pipelines_gogpu.go`
  has separate opaque, alpha-test, translucent, turbulent, sky pipelines — each
  with different blend state and depth-write settings.
- **Translucent face collection + sorting**: `world_gogpu_translucent.go`
  (`renderGoGPUSortedTranslucentFaceRendersHAL`).
- **The render order** in `RenderFrame()`: opaque world → opaque entities →
  translucent water → translucent entities. See
  `renderer_gogpu_frame.go:82` and the phase comments.
- **OIT (Order-Independent Transparency)**: when enabled, the renderer uses
  weighted-blended transparency to avoid sorting — see `oit_render_path.go`
  and `oit_mode.go`. This is an advanced, optional path.

### Milestone

Add a translucent triangle to your viewer. Make it fail first: draw it before
the opaque ones with depth-write on, and watch it occlude things it shouldn't.
Then fix it: draw opaque first (depth-write on, depth-test on), then the
translucent one (depth-write off, depth-test on, alpha blend). Sort multiple
translucent triangles by distance to the camera.

**Done means:** you can explain, for any given face in the Ironwail-Go world
pipeline, whether it uses depth-write-on or off and *why*, and you can
articulate the exact render order in `RenderFrame()`.

---

## Stage 8 — Special World Passes: Sky, Liquids (Turbulent), and Fog

### Concept

1. **scratchapixel** — [*Procedural
   Texturing*](https://www.scratchapixel.com/lessons/procedural-texturing)
   — Quake's water/lava/sky surfaces use a "turbulent" warp: the UV coordinates
   are animated with a sine function to make the texture swim. This is the
   oldest procedural effect in real-time graphics.
2. **scratchapixel** — [*Fog / Volumetric
   Scattering*](https://www.scratchapixel.com/lessons/3d-basic-rendering/global-illumination-path-tracing)
   (the fog concept — Quake uses exponential distance fog).
3. **webgpufundamentals** — no specific sky/fog lesson, but the skybox concept
   is "draw a big cube around the camera with a texture on it, and disable
   depth-write so the world draws on top".

### Ironwail-Go reality

- **Turbulent (water/lava/sky) pipeline**: the `turbulent` and
  `translucent-turbulent` pipelines in `world_pipelines_gogpu.go`. The
  fragment shader warps UVs over time — see `buildWorldFragmentShaderWGSL()`
  for the warp math and the `time` uniform.
- **Sky faces**: Quake sky is drawn as a special surface that ignores depth
  and uses a two-layer scrolling texture. See the `sky` pipeline and the
  `skyWindPhase`/`skyWindDir` fields in `worldUniformsWGSL`.
- **External skybox** (custom PNG/TGA/JPG cubemaps): `skybox_external.go`
  for loading, `world_external_sky_gogpu.go` for the GPU bind group/pipeline.
- **Fog**: the `fog_color` / `fog_density` uniforms in `worldUniformsWGSL`
  (`world_shaders_gogpu.go:10`); the fragment shader applies exponential fog
  based on view distance.
- **Water warp (screen-space)**: `warpscale_gogpu.go` — when underwater, the
  final composited scene is distorted by a sinusoidal screen-space warp. See
  `sceneCompositeFragmentShaderWGSL` at `warpscale_gogpu.go:45`.

### Milestone

Add a "water surface" to your viewer: a quad with a texture whose UVs scroll
over time (the turbulent effect). Then add distance fog: blend the fragment
color toward a fog color based on the view-space Z. Then add a skybox: a big
cube with a texture, drawn with depth-write off and depth-test set to "less or
equal" trickery so it sits behind everything.

**Done means:** you can explain the difference between surface-space water warp
(turbulent UVs) and screen-space water warp (the underwater scene composite),
and you can point to both in the codebase.

---

## Stage 9 — Dynamic Lights (Cluster Compute)

### Concept

1. **scratchapixel** — [*Point Lights and
   Shading*](https://www.scratchapixel.com/lessons/3d-basic-rendering/rasterization-practical-implementation/lighting-and-shading.html).
2. **webgpufundamentals** — [*Compute
   Shaders*](https://webgpufundamentals.org/webgpu/lessons/webgpu-compute-shaders.html).
   Understand that WebGPU can run general-purpose programs on the GPU, not
   just graphics. A compute shader writes to storage buffers or textures.
3. The **cluster forward** concept: divide the camera frustum into a 3D grid
   of "clusters" (32×16×32 tiles). For each cluster, compute which lights
   affect it. The fragment shader then only iterates the lights in its
   cluster. This is how modern engines do many dynamic lights cheaply.

### Ironwail-Go reality

- **Cluster compute pipeline**: `createWorldClusterComputePipeline()` at
  `internal/renderer/world_cluster_compute_gogpu.go:13`.
- **Compute shader**: `worldClusterComputeShaderWGSL` at
  `internal/renderer/world_compute_shaders_gogpu.go:5`.
- **Dispatch + light upload**: `dispatchWorldClusterCompute()` at
  `world_cluster_compute_gogpu.go:75`, called from `renderWorldInternal` at
  `world_render_gogpu.go:99`.
- **Dynamic light gathering**: `internal/renderer/dynamic_light.go` and
  `dynamic_light_pool.go`.
- **Log-depth setup**: `Core.SetupFrameData()` at `core_gogpu.go:158`
  computes the z-scale/bias used for cluster z-slicing.
- **Fragment shader light loop**: `buildWorldFragmentShaderWGSL()` reads the
  cluster bitmask and iterates the assigned lights.

### Milestone

This is an advanced stage — consider it optional. If you attempt it: add 3
moving point lights to your viewer with a naive "loop all lights in the
fragment shader" approach first. Then, if you want, implement a 2D grid of
clusters with a compute shader that writes a bitmask of light indices per
cluster. The Ironwail-Go code is a reference implementation of exactly this.

**Done means:** you can explain what a compute shader is, how it differs from a
vertex/fragment shader, and why cluster forward beats "loop all lights".

---

## Stage 10 — Entities: Brush, Alias (MDL), Sprite, Decal

### Concept

1. **scratchapixel** — [*Polygon
   Meshes*](https://www.scratchapixel.com/lessons/3d-basic-rendering/building-a-scene/polygon-mesh.html)
   (brush entities are just more BSP-like geometry, but transformed per
   entity) and [*Rasterizing a
   Triangle*](https://www.scratchapixel.com/lessons/3d-basic-rendering/rasterization-practical-implementation/rasterization-stage.html).
2. **Alias models (MDL)**: Quake's model format. An MDL is a mesh + a set of
   animation frames; the renderer interpolates between frames. Read
   `internal/model.md` and the `internal/renderer/alias/` package
   (`mesh.go`, `model.go`) for the interpolation math.
3. **Sprites**: 2D billboards that always face the camera. scratchapixel's
   billboard concept is covered under the rasterization practical
   implementation.
4. **Decals**: small flat patches projected onto surfaces (bullet holes). Read
   `internal/renderer/decal_shared.go` and `internal/renderer/mark_system.go`.

### Ironwail-Go reality

This is a big stage with four sub-pipelines, each with its own shader and
pipeline:

| Entity type | Pipeline setup | Render fn | Shader |
| --- | --- | --- | --- |
| Brush entity | `world_gogpu_brush_render.go` | `renderOpaqueBrushEntitiesHAL` (:11) | reuses world shaders |
| Alias (MDL) | `world_gogpu_alias.go` `ensureAliasResourcesLocked` (:24) | `renderAliasEntitiesHAL` (:582) | `AliasVertexShaderWGSL` at `world/gogpu/shaders.go:3` |
| Sprite | `world_gogpu_sprite.go` `ensureSpriteResourcesLocked` (:13) | `renderSpriteEntitiesHAL` (:336) | `SpriteVertexShaderWGSL` at `world/gogpu/shaders.go:82` |
| Decal | `world_gogpu_decal.go` `ensureDecalResourcesLocked` (:12) | `renderDecalMarksHAL` (:240) | `DecalVertexShaderWGSL` at `world/gogpu/shaders.go:157` |

The **viewmodel** (first-person weapon) is a special alias-model render with
its own depth handling: `renderViewModelHAL()` at `world_gogpu_alias.go:593`.

All of these are orchestrated in `renderEntities()` at
`renderer_gogpu_frame.go:586`, which orders them into opaque → sky →
translucent passes.

### Milestone

Add a "model" to your viewer: a second mesh loaded from a file, with its own
model matrix (so you can move it independently of the world). That is a brush
entity. Then add a "billboard": a quad whose vertices are recomputed each frame
to face the camera — that is a sprite. Then add frame interpolation: load two
versions of your mesh and lerp their vertices — that is an alias model.

**Done means:** you can explain the difference between a brush entity (world
geometry, transformed) and an alias model (separate mesh, animated), and why
the viewmodel needs special depth handling.

---

## Stage 11 — Particles

### Concept

1. **scratchapixel** — particles are not a dedicated lesson, but they are
   just camera-facing billboards (sprites) with a procedural (circle
   falloff) texture. Reuse the sprite and procedural texturing concepts.
2. **webgpu fundamentals** — a particle system is "a vertex shader that
   computes positions from a buffer of particle data".

### Ironwail-Go reality

- CPU-side simulation + vertex generation:
  `internal/renderer/particle.go`.
- GPU pipeline + shaders: `particle_gogpu.go`
  (`particleVertexShaderWGSL` at :20, `particleFragmentShaderWGSL` at :75,
  `ensureParticleResourcesLocked` at :148, `renderParticlesHAL` at :354).
- The fragment shader draws a soft circle (radial alpha falloff) — read it to
  see procedural shading.

### Milestone

Add 100 particles to your viewer: each is a billboard with a procedurally
generated soft circle. Simulate them on the CPU (e.g. gravity) and upload
their positions each frame.

**Done means:** you understand that particles are "sprites with procedural
shading and CPU-side physics".

---

## Stage 12 — Post-Processing: Scene Composite, PolyBlend, Overlay

### Concept

1. **webgpufundamentals** — [*Post-Processing*](https://webgpufundamentals.org/webgpu/lessons/webgpu-post-processing.html)
   and [*Render to
   Texture*](https://webgpufundamentals.org/webgpu/lessons/webgpu-render-to-texture.html).
   The key idea: render the 3D scene to an offscreen texture, then draw that
   texture to the screen with a fullscreen shader that can distort it.
2. **scratchapixel** — the underwater warp is a screen-space sine distortion;
   think of it as a procedural image filter.

### Ironwail-Go reality

This is where the frame's 3D work becomes the final image. Three post passes,
in order:

1. **Scene composite** (`compositeSceneRenderTarget()` at
   `warpscale_gogpu.go:472`): blits the offscreen scene render target to the
   swapchain surface, applying the underwater warp if the camera is in water.
   Shaders: `sceneCompositeVertexShaderWGSL` (:16),
   `sceneCompositeFragmentShaderWGSL` (:45).
2. **PolyBlend** (`renderPolyBlendHAL()` at `polyblend_gogpu.go:224`): a
   fullscreen tint — Quake's "palette flash" / underwater color wash.
   Shaders: `polyBlendVertexShaderWGSL` (:15),
   `polyBlendFragmentShaderWGSL` (:34).
3. **2D overlay** (`flush2DOverlay()` at
   `internal/renderer/renderer_gogpu_overlay.go:32`): the HUD/menu/console,
   rendered CPU-side into a single texture and blitted to the screen.
   Pipeline: `overlay_composite_gogpu.go`
   (`overlayCompositeVertexShaderWGSL` at :11,
   `overlayCompositeFragmentShaderWGSL` at :37).

### Milestone

Render your scene to an offscreen texture instead of directly to the screen.
Then add a fullscreen pass that samples that texture and draws it to the
screen. Add a "drunk mode" toggle that applies a sine distortion to the UVs in
that fullscreen pass — you have just built the underwater warp.

**Done means:** you can explain what a render target is, why Ironwail-Go uses
one (the warp), and why the overlay is drawn last.

---

## Stage 13 — The Frame: Putting It All Together

### Concept

Re-read, now with everything you've learned:

1. **webgpufundamentals** — [*WebGPU Animation*](https://webgpufundamentals.org/webgpu/lessons/webgpu-animation.html)
   and the fundamentals article's "the render loop" section.
2. **scratchapixel** — [*Rasterization: Putting It All
   Together*](https://www.scratchapixel.com/lessons/3d-basic-rendering/rasterization-practical-implementation/rasterization-stage.html).

### Ironwail-Go reality

Read `RenderFrame()` at `internal/renderer/renderer_gogpu_frame.go:82` end to
end. You should now be able to map every call to a stage above:

| Frame phase | Code | Stage in this plan |
| --- | --- | --- |
| Clear | `renderer_gogpu_frame.go:113-129` | Stage 0/1 |
| Cluster compute dispatch | `world_render_gogpu.go:99` | Stage 9 |
| World BSP render | `renderWorldInternal` `world_render_gogpu.go:16` | Stages 3,4,5,6,7,8 |
| Opaque brush/alias/sprite/particle entities | `renderEntities` `renderer_gogpu_frame.go:586` | Stage 10, 11 |
| Translucent water + entities (sorted) | `renderGoGPUSortedTranslucentFaceRendersHAL` | Stage 7 |
| Viewmodel | `renderViewModelHAL` `world_gogpu_alias.go:593` | Stage 10 |
| Scene composite (warp) | `compositeSceneRenderTarget` `warpscale_gogpu.go:472` | Stage 12 |
| PolyBlend | `renderPolyBlendHAL` `polyblend_gogpu.go:224` | Stage 12 |
| 2D overlay (HUD/menu/console) | `flush2DOverlay` `renderer_gogpu_overlay.go:32` | Stage 12 |

### Milestone

This is the capstone. Combine all your prior milestones into one program:
- A world with lightmaps and visibility (Stages 3-6)
- A skybox and a water surface (Stage 8)
- A moving entity and a viewmodel (Stage 10)
- Particles (Stage 11)
- A scene composite with underwater warp + an overlay (Stage 12)

Render it all in one frame loop, in the order above. You have now built a
(minimal) Quake renderer.

**Done means:** you can read `renderer_gogpu_frame.go:82` top to bottom and
explain every line.

---

## Stage 14 (Optional / Advanced) — Order-Independent Transparency

### Concept

1. **webgpufundamentals** — no dedicated OIT lesson, but the technique
   (weighted-blended transparency, McGuire & Bavoil 2013) is a well-known
   extension of the blending concepts in Stage 7.

### Ironwail-Go reality

- Mode selection: `internal/renderer/oit_mode.go`.
- Render path: `internal/renderer/oit_render_path.go`.
- Stub: `internal/renderer/oit_stub.go`.
- Shared helpers: `internal/renderer/oit/`.

This is an **optional** path that replaces the sorted-translucent pass with a
weighted-blended one, avoiding the sort. It is enabled by a cvar.

### Milestone (optional)

Implement weighted-blended transparency in your viewer: render translucent
objects to an accumulation texture + revealage texture, then composite. Compare
visual quality to the sorted approach.

---

## Reference: Where to Read First in This Repo

When you get stuck on a stage, read these in this order:

1. `docs/VERTEX_LAYOUT.md` — the vertex contract (Stage 3+).
2. `docs/internal/renderer.md` — the package overview.
3. `docs/LEARNING_GUIDE.md` — engine-wide architecture.
4. The WGSL shader files — they are the most readable part of the renderer
   because they are self-contained programs:
   - `internal/renderer/world_shaders_gogpu.go`
   - `internal/renderer/world_compute_shaders_gogpu.go`
   - `internal/renderer/world/gogpu/shaders.go`
   - `internal/renderer/polyblend_gogpu.go`
   - `internal/renderer/warpscale_gogpu.go`
   - `internal/renderer/overlay_composite_gogpu.go`
   - `internal/renderer/particle_gogpu.go`
5. The `docs/diagnoses/` folder — consolidated bug investigation records:
   - `qbj2_water.md` — water translucency (resolved, exercises Stage 7)
   - `qbj2_materials.md` — texture atlas and materials buffer issues
     (exercises Stage 4)

## State of the Renderer (as of 2026-07-27)

**Important context:** The renderer is functional but **unfinished and has
known bugs**. This plan documents *what is currently in the code*, not what is
"correct" or final. Expect specifics (line numbers, pipeline names, shader
fields) to drift as fixes land. Treat the diagnosis docs as a snapshot, not a
closed investigation. Known open/partially-resolved issues:

- **Water translucency (qbj2):** **Resolved** (commit `6802fc5`). Translucent
  water is now drawn within the world render pass using the translucent
  turbulent pipeline, with dynamic uniform buffer offsets for the alpha value.
  See `docs/diagnoses/qbj2_water.md` for the full diagnosis record.
- **Texture atlas overflow (qbj2, BSP2 large maps):** **Open.** The materials
  uniform buffer is hardcoded to 256 entries but `baseMaterials` is allocated
  as `textureCount + 2` without clamping. When a map has >254 textures, a
  silent buffer overflow occurs. See `docs/diagnoses/qbj2_materials.md`.
- **Brush entity texture animation:** **Resolved** (commit `aa17df6`). Pressed
  buttons now show their alternate textures via a frame-1 materials buffer.
  See `docs/diagnoses/qbj2_materials.md`.
- **General parity gaps:** The renderer does not yet match C Ironwail
  feature-for-feature. Consult `docs/PARITY.md` for the current gap list
  before assuming any visual discrepancy you see is a "bug" versus a
  not-yet-implemented feature.

When a diagnosis doc and the code disagree, **the code wins**. The docs are a
narrative of past understanding; re-derive from the code when in doubt.

## Reference: External Citations

| Source | Use it for |
| --- | --- |
| [scratchapixel.com](https://www.scratchapixel.com/) — *Introduction to Computer Graphics* | The "why" of every stage: what a GPU does, what rasterization is, what projection is, what a lightmap is. |
| [scratchapixel.com](https://www.scratchapixel.com/) — *Rasterization: Practical Implementation* | The deep mechanics of vertex transform, clipping, depth buffer, texturing, transparency. |
| [webgpufundamentals.org](https://webgpufundamentals.org/) — *WebGPU Fundamentals* | The "how" of every stage: the WebGPU API, buffers, textures, shaders, pipelines, bind groups. |
| [webgpufundamentals.org](https://webgpufundamentals.org/) — *WGSL* | The shading language Ironwail-Go uses for all shaders. |
| [webgpufundamentals.org](https://webgpufundamentals.org/) — *Compute Shaders* | The cluster dynamic-light stage (Stage 9). |
| [webgpufundamentals.org](https://webgpufundamentals.org/) — *Post-Processing / Render to Texture* | The scene composite and overlay stages (Stage 12). |

## A Note on Scope and Pacing

This plan is deliberately long. A motivated reader working full-time should
expect roughly:

- Stages 0-2: 2-3 days (the WebGPU on-ramp).
- Stages 3-5: 1 week (the world pipeline is the hardest part).
- Stage 6: 2-3 days (mostly conceptual).
- Stage 7: 3-4 days (the ordering bug class is subtle).
- Stage 8: 3-4 days.
- Stage 9: 1 week (compute shaders are a new mental model).
- Stages 10-11: 1 week (four entity sub-pipelines).
- Stages 12-13: 1 week (post-processing + integration).
- Stage 14: optional, 3-5 days.

Total: roughly 6-8 weeks of focused part-time study to understand the whole
renderer, and to be able to read any file in `internal/renderer/` and explain
what it does and why.
