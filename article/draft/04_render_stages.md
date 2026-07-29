# Chapter 4: Render Stages, Broken Down

Chapter 3 compared the C OpenGL renderer and the Go WebGPU renderer
architecturally. This chapter walks through a single rendered frame,
stage by stage, from clear to overlay. For each stage: what it is for, where
it lived in C, how it works in Go, and what bugs were encountered.

The stage numbering follows `docs/RENDERER_LEARNING_PLAN.md` (Stages 0–14),
which is the project's canonical curriculum for learning the renderer.
[LearningPlan](#ref-learningplan) The frame orchestration lives in
`RenderFrame()` at `renderer_gogpu_frame.go:82`, and the world render pass
lives in `renderWorldInternal()` at `world_render_gogpu.go:16`.

---

## Stage 0: The GPU core — Instance, Adapter, Device, Queue

### Purpose

Before any rendering can happen, the engine must establish a connection to
the GPU. In WebGPU, this is a four-step hierarchy: create an Instance (the
entry point to the WebGPU API), request an Adapter (a physical GPU), open a
Device (a logical GPU context with its own queue), and get the Queue (the
command submission interface). [LearningPlan](#ref-learningplan)

### C reference

OpenGL has no equivalent — the context is created implicitly by the
platform layer (`SDL_GL_CreateContext` in `gl_vidsdl.c`). There is no
adapter selection; the OS's default GPU is used.

### GoGPU reality

The `Core` struct (`core_gogpu.go:46`) holds the Instance, Adapter, Device,
and Queue. `CoreConfig` specifies backend type (`BackendGo`), graphics API
(`GraphicsAPIAuto`), validation (enabled by default), and GPU preference
(`GPUPreferHighPerformance`). `DefaultCoreConfig()` returns these defaults.

The `Core` is used for both windowed and headless/screenshot rendering. In
windowed mode, the `gogpu.App` event loop owns the surface; in headless
mode, `Core.InitHeadless()` creates an offscreen surface for screenshot
capture. The GPU preference was the subject of gogpu issue #176 (adapter
power preference not forwarded on hybrid-GPU Linux systems).
[GogpuIssues](#ref-gogpuissues)

### Bugs/lessons

The screenshot path was originally a stub writing `RGB(20,20,46)` — a
plausible-looking dark color that was not a real GPU readback. This
actively misled the water translucency investigation until it was fixed.
[WaterDiag](#ref-waterdiag)

---

## Stage 1: The triangle — pipelines, shaders, bind groups

### Purpose

The fundamental unit of WebGPU rendering: a vertex buffer + a WGSL vertex
shader + a WGSL fragment shader + a pipeline object + a bind group, all
wired together to produce pixels. [LearningPlan](#ref-learningplan)

### C reference

In C, this is `glBegin/glEnd` (legacy) or VBO + `glDrawArrays` (core
profile). Shaders are GLSL, compiled at runtime. Pipeline state is mutable
and set imperatively.

### GoGPU reality

The simplest real shader is the **polyblend** fullscreen triangle
(`polyblend_gogpu.go:15`). It has no vertex buffer — positions are baked
into the shader using `@builtin(vertex_index)`:

```wgsl
@vertex
fn vs_main(@builtin(vertex_index) vertexIndex: u32) -> VertexOutput {
    var positions = array<vec2<f32>, 3>(
        vec2<f32>(-1.0, -1.0),
        vec2<f32>( 3.0, -1.0),
        vec2<f32>(-1.0,  3.0),
    );
    var output: VertexOutput;
    output.clipPosition = vec4<f32>(positions[vertexIndex], 0.0, 1.0);
    return output;
}
```

The fragment shader (`polyblend_gogpu.go:34`) reads a `blendColor` uniform
and outputs it. This is a fullscreen tint — Quake's "polyblend" used for
underwater color wash and damage flashes. The pipeline setup is in
`ensurePolyBlendResourcesLocked()` (`:83`); per-frame use is
`renderPolyBlendHAL()` (`:224`), called from `RenderFrame()` at
`:218`. [LearningPlan](#ref-learningplan)

For a real vertex-buffer example, the **particle** pipeline
(`particle_gogpu.go:20`) uses instanced vertices with per-particle position
and color attributes.

### Bugs/lessons

The naga WGSL→SPIR-V compiler had a bug with scalar `mix()` (gogpu issue
#162) — `mix(vec3, vec3, f32)` produced invalid SPIR-V that crashed on
NVIDIA. The workaround was `vec3<f32>(fog)` splat. Fixed in naga v0.17.0+.
[GogpuIssues](#ref-gogpuissues)

---

## Stage 2: Matrices, the camera, and 3D-to-2D

### Purpose

The **view matrix** transforms world space into camera/eye space. The
**projection matrix** transforms eye space into clip space (the GPU's
normalized cube). Together they are the VP matrix. [LearningPlan](#ref-learningplan)

### C reference

`R_SetupView` (`gl_rmain.c:964`) computes the view, calls
`AngleVectors` to get `vpn`/`vright`/`vup`, finds the view leaf via
`Mod_PointInLeaf`, and sets up the projection. The VP matrix is implicitly
the OpenGL projection/modelview stack.

### GoGPU reality

Camera state and VP computation live in `internal/renderer/camera.go`. The
VP matrix is packed into the world uniform buffer (`worldUniformsWGSL` at
`world_shaders_gogpu.go:10`):

```wgsl
struct Uniforms {
    viewProjection: mat4x4<f32>,
    cameraOrigin: vec3<f32>,
    fogDensity: f32,
    fogColor: vec3<f32>,
    time: f32,
    alpha: f32,
    litWater: f32,
    skyWindPhase: f32,
    // ...
}
```

The world vertex shader multiplies `position` by this matrix
(`worldVertexShaderWGSL` at `world_shaders_gogpu.go:27`). Uniform buffer
packing is in `renderer_gogpu_uniforms.go`. In `renderWorldInternal()`,
the VP is computed and written to the GPU buffer at `:144-153`.

---

## Stage 3: Loading real geometry — the BSP world

### Purpose

Upload the BSP world geometry (vertices, edges, faces, textures) to GPU
buffers and render it. The world is "one big mesh with extra metadata."
[LearningPlan](#ref-learningplan)

### C reference

`R_DrawWorld` in `r_world.c` recursively walks the BSP tree, marks visible
surfaces, builds texture chains, and draws them. The C renderer uses SSBOs
and indirect multi-draw for batching (`gl_bmodel_indirect_buffer`,
`GL_DrawTextures`).

### GoGPU reality

This is the largest stage. `UploadWorld()` at
`world_upload_gogpu.go:18` orchestrates everything:

- **BSP → GPU vertex upload**: `WorldGeometry` in
  `world_geometry_gogpu.go` constructs the vertex data.
- **Vertex construction**: the 48-byte `WorldVertex` struct (see Chapter 3
  and `docs/VERTEX_LAYOUT.md`) flows from Go struct → byte packer → WGSL
  `@vertex` input. [VertexLayout](#ref-vertexlayout)
- **Byte packing**: `appendGoGPUWorldVertexBytes` in `world_gogpu.go`.
- **Pipeline creation**: `createWorldPipeline()` and friends in
  `world_pipelines_gogpu.go:13`.
- **The render pass**: `renderWorldInternal()` at
  `world_render_gogpu.go:16` creates the command encoder, begins the
  render pass with `LoadOpClear`, sets the pipeline, sets vertex/index
  buffers, sets bind groups, and issues `DrawIndexed` calls.

The render pass descriptor (`world_render_gogpu.go:107-118`) attaches both
a color attachment (the surface view or scene render target) and a
depth-stencil attachment (`worldDepthTextureView`).

### Bugs/lessons

Texture corruption on multi-layer atlas maps (commit `d89b34c`) was caused
by not copying both atlas layer and bounds when animating textures. The
fix was in `animateWorldMaterials` to swap the entire material config.
[MaterialsDiag](#ref-materialsdiag)

---

## Stage 4: Textures and the texture atlas

### Purpose

A Quake map has hundreds of small textures. WebGPU cannot bind hundreds of
textures individually. Solution: pack them into a single atlas texture and
use per-vertex `materialID` to index into a materials uniform buffer that
holds the atlas bounds and layer for each texture. [LearningPlan](#ref-learningplan)

### C reference

C uses per-texture `GL_Bind` calls. Texture chains group faces by texture
to minimize bind calls, but each chain still requires a bind.

### GoGPU reality

- **Atlas packer**: binary-tree packer in `world_atlas_gogpu.go`
  (`TextureAtlasNode`, `AtlasLayer`). A 2048×2048 atlas with multiple layers.
- **Atlas upload + GPU texture**: `world_resources_gogpu.go` (search for
  atlas creation).
- **Per-face texture index**: each `WorldVertex` carries a `MaterialID`
  (uint32 at offset 44).
- **Materials buffer**: 256 entries of 32 bytes each, updated each frame by
  `animateWorldMaterials` (`world_material_gogpu.go:24`) with the current
  animation frame.
- **Fragment shader sampling**: `buildWorldFragmentShaderWGSL()` in
  `world_shaders_gogpu.go:83` samples `worldTexture` using per-vertex UV and
  `materials[materialID].atlasBounds`.

### Bugs/lessons

The **atlas overflow** bug (still open): the materials buffer is hardcoded
to 256 entries, but `baseMaterials` is allocated as `textureCount + 2`
without clamping. When the qbj2 mod's `start` map has more than 254
textures, the `WriteBuffer` call silently overflows the 8192-byte GPU
buffer. The `diagMaterialBufferCapacity` and `diagMaterialBufferWrite`
functions in `diag_atlas.go` log warnings but do not clamp. The fix would
require changing the uniform buffer to a storage buffer to remove the
256-entry limit. [MaterialsDiag](#ref-materialsdiag)

---

## Stage 5: Lightmaps — pre-baked lighting

### Purpose

Quake does not compute lighting at runtime. Lighting is pre-baked offline
by the map compiler (`qrad`) and stored as a lightmap: a small grayscale
texture per face. The fragment shader samples both the material texture and
the lightmap, and multiplies them. [LearningPlan](#ref-learningplan)

### C reference

`R_DrawTextureChains` in `r_world.c` binds the lightmap texture and draws
with multi-texturing. Lightstyles (animated lighting) are evaluated per
frame in `CL_RunLightStyles` (`cl_main.c`).

### GoGPU reality

- **Lightmap sample extraction**: `internal/renderer/lightmap_samples.go`
  and `internal/renderer/world/lightmap_samples.go`.
- **Lightmap page stacking + GPU upload**: `uploadWorldLightmapArray()` at
  `world_lightmap_gogpu.go:11`. Uses 1px padding and vertical stacking
  (a Vulkan workaround).
- **Lightstyles**: the renderer evaluates lightstyle values per frame and
  rebuilds lightmap pages whose style changed. The `setGoGPUWorldLightStyleValues`
  function is called from `RenderFrame` (`renderer_gogpu_frame.go:135`).
- **Fragment shader**: `buildWorldFragmentShaderWGSL()` samples
  `worldLightmap` and multiplies it into the final color.

C never allocates lightmaps for `SURF_DRAWTURB` (water/lava) surfaces —
they are fullbright. Ironwail added optional lit water via `r_litwater`. The
Go port samples the lightmap when `litWater > 0.5` in the WGSL uniform.
[WaterDiag](#ref-waterdiag)

### Bugs/lessons

The fallback lightmap was created as `TextureViewDimension2D` but the
shader declared `texture_2d_array<f32>`. WebGPU rejected it silently,
defaulting to fullbright white (×2.0 overbright). Fixed by using
`TextureViewDimension2DArray`. [WaterDiag](#ref-waterdiag)

---

## Stage 6: Visibility — BSP, PVS, and "don't draw what you can't see"

### Purpose

The single most important optimization in a Quake renderer. The BSP tree
organizes the world into convex leaves. Each leaf has a PVS (Potentially
Visible Set) bitmask saying which other leaves can be seen from it. Before
drawing, the engine finds the camera's leaf, looks up the PVS, and only
draws faces in visible leaves. [LearningPlan](#ref-learningplan)

### C reference

`R_MarkVisSurfaces` (`r_world.c:58`) and `R_MarkSurfaces` (`r_world.c:111`)
walk the BSP tree and mark visible surfaces using the PVS.

### GoGPU reality

- **BSP leaf lookup + PVS**: `WorldRenderData` in `world.go:57` is a passive
  data holder. Actual visible face selection is `selectVisibleWorldFaces` in
  `world_shared.go:172`, called from `world_render_gogpu.go:333`.
- **Face classification**: opaque, alpha-test, translucent,
  turbulent/sky — helpers in `world_shared.go`.
- **What gets drawn**: `renderWorldInternal()` only draws faces that passed
  visibility. This is why Quake can render huge maps at 60 FPS — the qbj3
  `qbj3_stickflip` map has 85,936 raw faces but only 1,002 visible at the
  spawn view. [Parity](#ref-parity)

### Bugs/lessons

Single-leaf PVS culled underwater geometry. Fixed by using `FatPVS` (from
C's `SV_FatPVS`) when the camera leaf contains water faces. Also, a BSP2
`HeadNode` traversal bug caused `FatPVS`/`PointInLeaf` to start at node 0
(submodel) instead of `Models[0].HeadNode[0]` — critical for BSP2 maps.
Fixed in `internal/bsp/tree.go`. [WaterDiag](#ref-waterdiag)

---

## Stage 7: Depth testing and the opaque/translucent ordering problem

### Purpose

Opaque objects use depth testing (draw in any order, the depth buffer
resolves which is in front). Translucent objects must be sorted
back-to-front and drawn with depth-write off. [LearningPlan](#ref-learningplan)

### C reference

C draws the entire frame to a single framebuffer with no intermediate
submits. Opaque water (`R_DrawWater(false)`) draws with blend=OPAQUE,
depth-write=ON. Translucent water (`R_DrawWater(true)`) draws with
blend=ALPHA, depth-write=OFF. Both use the same framebuffer. The key
principle: no face is drawn both opaquely and translucently — the split is
by alpha value, not by pass. [WaterDiag](#ref-waterdiag)

### GoGPU reality

- **Depth texture**: `createWorldDepthTexture()` at
  `world_depth_gogpu.go:21`.
- **Multiple pipelines**: `world_pipelines_gogpu.go` has separate opaque,
  alpha-test, translucent, turbulent, and sky pipelines — each with
  different blend state and depth-write settings.
- **Translucent face collection + sorting**: `world_gogpu_translucent.go`
  (`renderGoGPUSortedTranslucentFaceRendersHAL`).
- **Render order**: opaque world → opaque entities → translucent water →
  translucent entities (see the `RenderFrame` phase table in Chapter 3).
- **OIT** (optional): `oit_render_path.go` replaces the sort with
  weighted-blended transparency.

### Bugs/lessons

The water translucency bug (resolved in commit `6802fc5`) had three root
causes:

1. **Vulkan swapchain discard**: the original architecture split the frame
   into multiple `queue.Submit()` calls. The translucent water pass opened
   a new render pass with `LoadOpLoad` after the world pass had already
   submitted. Vulkan drivers may discard framebuffer contents between
   submits, so translucent water blended over black. Fix: draw translucent
   water **within the world render pass itself**.
2. **Uniform buffer offset collision**: the translucent water uniform
   (`alpha=0.6`) overwrote the opaque uniform (`alpha=1.0`) at offset 0.
   Fix: dynamic uniform buffer offsets.
3. **Worldspawn `wateralpha` bypass**: `ResolveLiquidAlphaSettings` only
   applied the override when `r_wateralpha` was exactly `1.0`. A stale
   config value prevented the map's `wateralpha=0.6` from taking effect.

[WaterDiag](#ref-waterdiag)

---

## Stage 8: Sky, liquids (turbulent), and fog

### Purpose

Quake's water/lava/sky surfaces use a "turbulent" warp: UV coordinates are
animated with a sine function to make the texture swim. Sky is a special
surface that ignores depth and uses a two-layer scrolling texture. Fog is
exponential distance fog. When underwater, the final composited scene is
distorted by a sinusoidal screen-space warp. [LearningPlan](#ref-learningplan)

### C reference

Turbulent warp is in `gl_warp.c` / `gl_warp_sin.h`. Sky is in `gl_sky.c`.
Fog is in `gl_fog.c`. The underwater screen-space warp is in
`gl_warp.c`'s `R_BloomScreen` / warp-scale pass.

### GoGPU reality

- **Turbulent pipeline**: the `turbulent` and `translucent-turbulent`
  pipelines in `world_pipelines_gogpu.go`. The fragment shader warps UVs
  over time using the `time` uniform.
- **Sky faces**: the `sky` pipeline. Two-layer scrolling texture with
  `skyWindPhase`/`skyWindDir` fields in `worldUniformsWGSL`.
- **External skybox**: `skybox_external.go` for loading (PNG/TGA/JPG
  cubemaps), `world_external_sky_gogpu.go` for GPU bind group/pipeline.
- **Fog**: `fog_color` / `fog_density` uniforms in `worldUniformsWGSL`;
  the fragment shader applies exponential fog based on view distance.
- **Screen-space warp**: `warpscale_gogpu.go` — the
  `sceneCompositeFragmentShaderWGSL` (`:45`) applies a sinusoidal UV
  distortion when the camera is in water.

The scene composite fragment shader (`warpscale_gogpu.go:64-79`) is the
underwater warp math:

```wgsl
let aspect = dpdy(uv.y) / dpdx(uv.x);
let warpV = vec2<f32>(warpAmp, warpAmp * aspect);
let remapped = warpV + uv * (1.0 - 2.0 * warpV);
uv = remapped + warpV * sin(vec2<f32>(remapped.y / aspect, remapped.x)
    * (3.14159265 * 8.0) + warpTime);
return textureSample(sceneTexture, sceneSampler, uv * uvScale);
```

### Bugs/lessons

The scene composite shader's use of `dpdx`/`dpdy` was one of the naga SPIR-V
bugs surfaced in gogpu issue #157 — derivatives produced invalid SPIR-V.
[GogpuIssues](#ref-gogpuissues)

---

## Stage 9: Dynamic lights (cluster compute)

### Purpose

Divide the camera frustum into a 3D grid of clusters (32×16×32 tiles). A
compute shader determines which lights affect each cluster. The fragment
shader iterates only the lights in its cluster, rather than looping all
lights. This is a modern technique the C renderer does not have.
[LearningPlan](#ref-learningplan)

### C reference

C Ironwail does not have cluster-forward lighting. It uses a simpler
dynamic light model (OpenGL point lights via `R_AddLights`).

### GoGPU reality

- **Cluster compute pipeline**: `createWorldClusterComputePipeline()` at
  `world_cluster_compute_gogpu.go:13`.
- **Compute shader**: `worldClusterComputeShaderWGSL` at
  `world_compute_shaders_gogpu.go:5`.
- **Dispatch + light upload**: `dispatchWorldClusterCompute()` at
  `world_cluster_compute_gogpu.go:75`, called from `renderWorldInternal`
  at `world_render_gogpu.go:99` — before the world render pass begins.
- **Dynamic light gathering**: `internal/renderer/dynamic_light.go` and
  `dynamic_light_pool.go`.
- **Log-depth setup**: `Core.SetupFrameData()` at `core_gogpu.go:158`
  computes the z-scale/bias for cluster z-slicing.
- **Fragment shader light loop**: `buildWorldFragmentShaderWGSL()` reads
  the cluster bitmask from `lightClusters` (a `texture_3d<u32>`) and
  iterates the assigned lights from the `dynamicLights` storage buffer.

---

## Stage 10: Entities — brush, alias, sprite, decal, viewmodel

### Purpose

Draw everything that isn't the static BSP world: doors and platforms (brush
entities), monsters and items (alias models), explosions and pickups
(sprites), bullet holes (decals), and the first-person weapon (viewmodel).
[LearningPlan](#ref-learningplan)

### C reference

`R_DrawEntitiesOnList` (`gl_rmain.c:1108`) dispatches by model type:
`R_DrawBrushModels` (`r_world.c:660`), `R_DrawAliasModels`
(`gl_mesh.c`), sprites, etc.

### GoGPU reality

Four sub-pipelines, each with its own shader and pipeline:

| Entity type | Pipeline setup | Render fn | Shader |
| --- | --- | --- | --- |
| Brush entity | `world_gogpu_brush_render.go` | `renderOpaqueBrushEntitiesHAL` | reuses world shaders |
| Alias (MDL) | `world_gogpu_alias.go` | `renderAliasEntitiesHAL` | `AliasVertexShaderWGSL` at `world/gogpu/shaders.go:3` |
| Sprite | `world_gogpu_sprite.go` | `renderSpriteEntitiesHAL` | `SpriteVertexShaderWGSL` at `world/gogpu/shaders.go:82` |
| Decal | `world_gogpu_decal.go` | `renderDecalMarksHAL` | `DecalVertexShaderWGSL` at `world/gogpu/shaders.go:157` |

The **viewmodel** (`renderViewModelHAL` at `world_gogpu_alias.go:593`) is
a special alias-model render with its own depth handling — it draws on top
of the world without depth-testing against it. All of these are orchestrated
in `renderEntities()` at `renderer_gogpu_frame.go:586`, ordered into opaque
→ sky → translucent passes. [LearningPlan](#ref-learningplan)

### Bugs/lessons

- **Back-face culling**: alias models needed `CullModeFront` (not
  `CullModeBack`) to match OpenGL's back-face culling convention
  (commits `7505c81`, `78a272d`).
- **Alias skin sampler**: needed `REPEAT` wrap mode, not
  `CLAMP_TO_EDGE` (commit `7911202`). Palette index 255 needed to be
  treated as opaque (commit `f0fb2af`).
- **Brush entity cutout faces**: needed alpha-test pipeline with nearest
  sampler (commits `e68aa0c`, `4f5e03b`, `6dfda87`).
- **Pressed button textures**: the frame-1 materials buffer was missing
  entirely — pressed buttons showed their unpressed texture (commit
  `aa17df6`). [MaterialsDiag](#ref-materialsdiag)

---

## Stage 11: Particles

### Purpose

Particles are camera-facing billboards with a procedural soft-circle
fragment shader. Simulated on the CPU (gravity, decay), uploaded each frame.
[LearningPlan](#ref-learningplan)

### C reference

`r_part.c` — `R_RunParticle` and `R_DrawParticles`.

### GoGPU reality

- **CPU-side simulation + vertex generation**:
  `internal/renderer/particle.go`.
- **GPU pipeline + shaders**: `particle_gogpu.go`
  (`particleVertexShaderWGSL` at `:20`, `particleFragmentShaderWGSL` at
  `:75`, `ensureParticleResourcesLocked` at `:148`,
  `renderParticlesHAL` at `:354`).
- **Batch capacity**: 512 particles per batch (`particleBatchCapacity`).
- The fragment shader draws a soft circle (radial alpha falloff).
- Particle instances use `@location(0) position` and `@location(1) color`
  per-instance attributes.

---

## Stage 12: Post-processing — scene composite, polyblend, overlay

### Purpose

Render the 3D scene to an offscreen texture, then draw that texture to the
screen with a fullscreen shader that can distort it (underwater warp), tint
it (polyblend), and finally draw the 2D UI on top. [LearningPlan](#ref-learningplan)

### C reference

C uses OpenGL FBOs for post-processing. The underwater warp is applied via
viewport/scissor tricks. The polyblend is `V_CalcBlend` → `R_SetupView`.
2D overlay is `SCR_UpdateScreen` → `Draw_Console` etc.

### GoGPU reality

Three post passes, in order:

1. **Scene composite** (`compositeSceneRenderTarget()` at
   `warpscale_gogpu.go:472`): blits the offscreen scene render target to
   the swapchain surface, applying the underwater warp if the camera is in
   water. Shaders: `sceneCompositeVertexShaderWGSL` (`:16`),
   `sceneCompositeFragmentShaderWGSL` (`:45`).
2. **PolyBlend** (`renderPolyBlendHAL()` at `polyblend_gogpu.go:224`):
   fullscreen tint. Shaders: `polyBlendVertexShaderWGSL` (`:15`),
   `polyBlendFragmentShaderWGSL` (`:34`).
3. **2D overlay** (`flush2DOverlay()` at
   `renderer_gogpu_overlay.go:32`): HUD/menu/console composited CPU-side
   into a single texture and blitted. Pipeline: `overlay_composite_gogpu.go`
   (`overlayCompositeVertexShaderWGSL` at `:11`,
   `overlayCompositeFragmentShaderWGSL` at `:37`).

All three use the same fullscreen-triangle pattern (vertex positions baked
into the shader via `@builtin(vertex_index)`).

---

## Stage 13: The full frame — `RenderFrame()` top to bottom

### Purpose

Combine all stages into one frame loop. [LearningPlan](#ref-learningplan)

### GoGPU reality

Reading `RenderFrame()` at `renderer_gogpu_frame.go:82` end to end:

| Frame phase | Code | Stage |
| --- | --- | --- |
| Clear | `:113-129` | Stage 0 |
| Cluster compute dispatch | `world_render_gogpu.go:99` | Stage 9 |
| World BSP render | `renderWorldInternal` `world_render_gogpu.go:16` | Stages 3-8 |
| Opaque brush/alias/sprite/particle entities | `renderEntities` `:586` | Stages 10-11 |
| Translucent water + entities (sorted) | `renderGoGPUSortedTranslucentFaceRendersHAL` | Stage 7 |
| Viewmodel | `renderViewModelHAL` `world_gogpu_alias.go:593` | Stage 10 |
| Scene composite (warp) | `compositeSceneRenderTarget` `warpscale_gogpu.go:472` | Stage 12 |
| PolyBlend | `renderPolyBlendHAL` `polyblend_gogpu.go:224` | Stage 12 |
| 2D overlay | `flush2DOverlay` `renderer_gogpu_overlay.go:32` | Stage 12 |

The `host_speeds 1` cvar enables per-phase timing (`clear_ms`,
`world_ms`, `entities_ms`, `viewmodel_ms`, `scene_composite_ms`,
`polyblend_ms`, `overlay_ms`, `total_ms`) logged each frame. [README](#ref-readme)

The depth-stencil is cleared before the entities phase
(`:177-188`) so entities can depth-test against the world without
re-rendering the world into the entity pass.

---

## Stage 14 (optional): Order-Independent Transparency

### Purpose

Replace the sorted-translucent pass with weighted-blended transparency
(McGuire & Bavoil 2013), avoiding the back-to-front sort. Enabled by a cvar.
[LearningPlan](#ref-learningplan)

### C reference

C Ironwail's `ALPHAMODE_OIT` path in `R_BeginTranslucency` — uses an OIT
framebuffer with accumulation and revealage textures, stencil state, and a
final resolve pass.

### GoGPU reality

- **Mode selection**: `internal/renderer/oit_mode.go`.
- **Render path**: `internal/renderer/oit_render_path.go`.
- **Stub**: `internal/renderer/oit_stub.go`.
- **Shared helpers**: `internal/renderer/oit/`.

When enabled, translucent objects render to an accumulation texture +
revealage texture, then composite. This avoids the sort but is optional —
the default path is sorted translucency.

---

## References

<a id="ref-gogpuissues"></a>[GogpuIssues] `article/gogpu_issues.md` (transcript of gogpu/gogpu issues).

<a id="ref-learningplan"></a>[LearningPlan] [`docs/RENDERER_LEARNING_PLAN.md`](../../docs/RENDERER_LEARNING_PLAN.md), ironwail-go repository.

<a id="ref-materialsdiag"></a>[MaterialsDiag] [`docs/diagnoses/qbj2_materials.md`](../../docs/diagnoses/qbj2_materials.md), ironwail-go repository.

<a id="ref-parity"></a>[Parity] [`docs/PARITY.md`](../../docs/PARITY.md), ironwail-go repository.

<a id="ref-readme"></a>[README] [`README.md`](../../README.md), ironwail-go repository.

<a id="ref-vertexlayout"></a>[VertexLayout] [`docs/VERTEX_LAYOUT.md`](../../docs/VERTEX_LAYOUT.md), ironwail-go repository.

<a id="ref-waterdiag"></a>[WaterDiag] [`docs/diagnoses/qbj2_water.md`](../../docs/diagnoses/qbj2_water.md), ironwail-go repository.


[ironwail]: https://github.com/andrei-drexler/ironwail
[gogpu]: https://github.com/gogpu/gogpu
[scratchapixel]: https://www.scratchapixel.com/
[webgpufundamentals]: https://webgpufundamentals.org/
[oto]: https://github.com/ebitengine/oto
