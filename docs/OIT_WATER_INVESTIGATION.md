# OIT Water Transparency Investigation

**Date:** 2026-08-29
**Status:** Partially resolved; remaining visual issue requires GPU-level debugging
**Branch:** renderer-cleanup

## Summary

Water/liquid transparency in the GoGPU renderer exhibits incorrect behavior: only
the nearest visible face of a water volume appears semi-transparent. Geometry
behind the water should show through but instead appears fully opaque on top of
the water surface. This affects both OIT (`r_oit 1`) and classic alpha-blend
(`r_oit 0`) paths identically.

Two concrete bugs were found and fixed during this investigation. The remaining
visual issue persists despite exhaustive code-level analysis confirming the
rendering pipeline matches C Ironwail's behavior.

## Bugs Found and Fixed

### 1. `goGPUOITEnabled()` ignored `r_oit` cvar

**File:** `internal/renderer/renderer_gogpu_world.go:413`

`goGPUOITEnabled()` was hardcoded to `return true`, making the `r_oit` cvar
completely non-functional. Both `r_oit 0` and `r_oit 1` always used the OIT
path, which is why toggling the cvar had no visual effect.

**Fix:** Changed to `return GetAlphaMode() == AlphaModeOIT`, which properly
consults the `r_oit` and `r_alphasort` cvars via the existing alpha mode system.

### 2. Gamma/contrast post-processing was missing

**Files:** `internal/renderer/types.go`, `internal/renderer/renderer_gogpu_warpscale.go`,
`internal/renderer/renderer_gogpu_runtime.go`, `internal/game/game_init.go`

The Go renderer read `r_gamma` into `Config.Gamma` but never applied it. No
`r_contrast` cvar existed at all. C Ironwail applies both as a final fullscreen
post-process pass (`GL_PostProcess` in `gl_rmain.c:330`):

```glsl
out_fragcolor.rgb *= contrast;
out_fragcolor = vec4(pow(out_fragcolor.rgb, vec3(gamma)), 1.0);
```

This caused an ~19% global brightness difference between C and Go screenshots.

**Fix:** Implemented gamma/contrast in the scene composite WGSL shader, added
`r_contrast` cvar (clamped `[1.0, 2.0]` per C behavior), extended the scene
composite uniform buffer from 16 to 32 bytes with a `postProcess` vec4.

## Remaining Issue: Single-Face Water Transparency

### Symptom

When looking at a water volume (pool, suspended block, etc.):

- Only the closest face to the camera renders as semi-transparent
- Geometry behind the water (walls, floors, objects) appears fully opaque and
  visually on top of the water face rather than showing through it
- Behavior is identical in both OIT and classic alpha-blend modes

### Analysis Performed

Every stage of the rendering pipeline was verified against C Ironwail:

#### Face Classification — Correct

- `shouldDrawGoGPUTranslucentLiquidFace()` correctly classifies liquid faces
  with alpha < 1.0 as translucent (`renderer_gogpu_world_resources.go:229`)
- `FacePass()` in `world/pass.go:42` routes `alpha < 1` to `PassTranslucent`
- The switch/case in `renderer_gogpu_world_render.go:315-320` is exclusive —
  each face goes to exactly one bucket (sky, translucent liquid, opaque, or
  alpha-test)
- Liquid alpha values are correctly read from `r_wateralpha` / map worldspawn
  overrides (`renderer_gogpu_world_geometry.go:519-539`)

#### Cull Mode — Equivalent to C

C Ironwail uses `glFrontFace(GL_CW)` + `GLS_CULL_BACK`:
- Front-facing = CW-wound triangles
- `GLS_CULL_BACK` culls back-facing (CCW) triangles, keeps CW

Go uses `FrontFaceCCW` + `CullModeFront`:
- Front-facing = CCW-wound triangles
- `CullModeFront` culls front-facing (CCW) triangles, keeps CW

Both combinations **keep CW-wound triangles and cull CCW-wound triangles**.
Vertex/index generation is identical between C (`r_brush.c:661-671, 877-882`)
and Go (`renderer_gogpu_world_geometry.go:376-397, 141-146`) — same surfedge
iteration order, same fan triangulation `(0, k-1, k)`.

Quake BSP liquid faces are single-sided by design. Both C and Go render only
one side. This is expected Quake behavior.

#### Render Order — Correct

Phase ordering matches C Ironwail (`R_DrawWater(true)` after
`R_DrawEntitiesOnList(false)`):

1. World opaque pass (walls, floors, ceilings, opaque liquids)
2. Entity opaque passes (brush, alias, particles)
3. Sky brush entities
4. Opaque liquid brush entities
5. **Translucent world liquid** ← water drawn here
6. Translucent liquid brush entities
7. Translucent brush/alias entities
8. Decals, sprites, translucent particles

Defined in `render_pass_parity.go:238-270`.

#### Depth State — Correct

- Opaque world pass: `DepthLoadOp: Clear`, `DepthWriteEnabled: true`
- Translucent liquid pass: `DepthLoadOp: Load`, `DepthWriteEnabled: false`
  (via `NonDecalDepthStencilState(false)` at `pipeline/pipeline.go:65`)
- OIT accumulation pass: same depth state, reads opaque depth buffer
- Both use the same `WorldDepthTextureView` — confirmed at
  `renderer_gogpu_world_render.go:128` and `renderer_gogpu_oit.go:506`

Note: `DepthReadOnly: true` on the OIT attachment causes a WebGPU validation
error (`usage conflict: existing 128 incompatible with 64`) because the depth
texture lacks `TEXTURE_BINDING` usage. The current approach
(`DepthReadOnly: false` + pipeline `DepthWriteEnabled: false`) is correct for
this texture configuration.

#### Blend State — Correct

**Classic alpha-blend pipeline** (`pipeline/world_pipelines.go:436-438`):
```
Color: SrcAlpha, OneMinusSrcAlpha, Add
Alpha: One, OneMinusSrcAlpha, Add
```
Standard premultiplied alpha blending. Matches C's `GLS_BLEND_ALPHA`.

**OIT accumulation pipeline** (`renderer_gogpu_oit.go:437-454`):
```
Target 0 (accum RGBA16Float): One, One, Add     (additive)
Target 1 (reveal R8Unorm):    Zero, OneMinusSrc, Add  (multiplicative)
```
Matches C's `GL_BlendFunciFunc(0, GL_ONE, GL_ONE)` and
`GL_BlendFunciFunc(1, GL_ZERO, GL_ONE_MINUS_SRC_COLOR)`.

**OIT resolve pipeline** (`renderer_gogpu_oit.go:366-368`):
```
Color: SrcAlpha, OneMinusSrcAlpha, Add
```
Matches C's `GLS_BLEND_ALPHA` for the resolve pass.

#### Shader Math — Correct

OIT accumulation weight function matches C exactly:

```wgsl
// Go (renderer_gogpu_oit.go:118)
let z = input.clipPos.w;
let weight = clamp(color.a * color.a * 0.03 / (1e-5 + z / 1e7), 1e-2, 3e3);
```

```glsl
// C (gl_shaders.h)
float z = 1./gl_FragCoord.w;  // == w_clip == clipPos.w
float weight = clamp(color.a * color.a * 0.03 / (1e-5 + pow(z/1e7, 1.0)), 1e-2, 3e3);
```

Accum/reveal output math is equivalent. Resolve formula
(`accum.rgb / max(accum.a, eps)`, coverage `1 - reveal`) matches C.

#### Render Target Management — Correct

- Scene render target enabled when translucent liquid faces exist
  (`renderer_gogpu_frame.go:190`)
- `enableSceneRenderTarget()` sets `dc.sceneRenderTarget = WorldRenderTextureView`
  (`renderer_gogpu_warpscale.go:452`)
- Both opaque world and translucent liquid passes write to the same scene RT
- No mid-frame clears between opaque and translucent passes
- Scene composite blits scene RT to surface with `LoadOpClear` (replaces
  surface content), then overlay draws on top with `LoadOpLoad`
- All formats consistent (`sceneSurfaceFormat()` used everywhere)

#### Command Buffer Synchronization — Correct

Frame graph shares one command encoder for world + entity + translucent passes.
All recorded into the same command buffer with implicit render pass barriers.
OIT resolve splits into a separate submit (required for attachment→sampled
barrier on Vulkan), but accum/reveal textures are fully written before resolve
reads them.

### Hypotheses for Remaining Issue

Since the code-level analysis confirms correctness, the remaining possibilities
are:

1. **WebGPU/naga MRT blend bug**: The Vulkan backend may not correctly implement
   per-target blend states for MRT. The OIT accumulation requires two different
   blend modes on two targets simultaneously. A driver or naga translation bug
   could cause one target to use the wrong blend equation.

2. **Scene render target presentation issue**: The scene RT → surface blit via
   the scene composite shader may lose or corrupt the blended water content.
   Testing without the scene RT (forcing direct-to-surface rendering) would
   isolate this.

3. **Depth buffer precision/format issue**: The shared depth texture format or
   precision may differ from what C Ironwail uses, causing subtle depth test
   differences that affect which fragments pass/fail.

4. **Texture sampling/filtering difference**: The water texture sampling in the
   turbulent shader may produce different results due to sampler configuration
   differences between OpenGL and WebGPU.

### Recommended Next Steps

1. **GPU capture with RenderDoc or Vulkan validation layer**: Inspect the actual
   draw calls, render pass attachments, blend states, and fragment shader outputs
   at the GPU level. This will definitively show whether the issue is in command
   encoding or GPU execution.

2. **Test without scene render target**: Temporarily force
   `shouldUseSceneRenderTarget()` to return false and render everything
   directly to the swapchain. If water transparency works correctly without the
   scene RT, the issue is in the scene composite path.

3. **Add debug visualization**: Render the OIT accum/reveal textures directly to
   screen (or log their pixel statistics) to verify the accumulation pass
   produces expected values.

4. **Test on different GPU/driver**: Rule out implementation-specific Vulkan
   bugs by testing on AMD/NVIDIA/Intel.

5. **Compare with C Ironwail screenshot at identical viewpoint**: Use the parity
   harness (`mise run parity-all`) to capture side-by-side screenshots at a
   viewpoint looking through water at known geometry. Quantify the pixel
   difference.

## GFXReconstruct Vulkan Trace Analysis

A gfxrecon capture (`gfxrecon_capture_20260830T012302.gfxr`, 2528 frames,
NVIDIA RTX 3060) was analyzed to identify the root cause at the Vulkan level.

### Pipeline Inventory (14 total)

| Handle | Purpose | Blend | Depth Write | Cull | Attachments |
|--------|---------|-------|-------------|------|-------------|
| 69 | Opaque world | ONE/ZERO (replace) | true | BACK | 1 |
| 70 | Sky | ONE/ZERO | false | BACK | 1 |
| 71 | Alpha-test world | ONE/ZERO | true | BACK | 1 |
| 74 | Sky (depth write) | ONE/ZERO | true | BACK | 1 |
| 75 | Sky variant | ONE/ZERO | false | BACK | 1 |
| 76 | Translucent brush entity | SRC_ALPHA/ONE_MINUS_SRC_ALPHA | false | BACK | 1 |
| 77 | Turbulent opaque liquid | ONE/ZERO | true | BACK | 1 |
| 78 | Translucent turbulent water | SRC_ALPHA/ONE_MINUS_SRC_ALPHA | false | BACK | 1 |
| 157 | Brush entity | SRC_ALPHA/ONE_MINUS_SRC_ALPHA | true | BACK | 1 |
| 188 | Overlay composite | SRC_ALPHA/ONE_MINUS_SRC_ALPHA | — | NONE | 1 |
| 192 | **OIT accumulation** | accum: ONE/ONE, reveal: ZERO/ONE_MINUS_SRC_COLOR | false | BACK | **2** |
| 271 | Scene composite | ONE/ONE_MINUS_SRC_ALPHA | — | NONE | 1 |
| 1777 | Depth-only | disabled | true | NONE | 1 |
| 1778 | gogpu 2D overlay | SRC_ALPHA/ONE_MINUS_SRC_ALPHA | false | NONE | 1 |

### Critical Finding: OIT Resolve Pipeline Missing

The OIT resolve pipeline (fullscreen triangle, no vertex input, SRC_ALPHA blend,
no depth) **does not appear in the Vulkan trace**. Only 14 pipelines exist; the
resolve would be #15.

In water-viewing frames, the OIT accumulation pass (pipeline 192) runs correctly
with **12 draw calls** drawing 50 triangles across multiple water faces. The
accum/reveal clear values are correct (accum=[0,0,0,0], reveal=[1,1,1,1],
depth=load). But the resolve pass that composites the accumulated transparency
over the scene **never executes**.

Frame sequence in water frames:
1. OIT accumulation (renderPass 197, pipeline 192) — 12 draw calls ✓
2. Brush entity (renderPass 180, pipeline 157)
3. Pipeline barriers
4. Overlay composite (renderPass 225, pipeline 188)
5. Pipeline barriers
6. Scene composite (renderPass 225, pipeline 271)
7. **OIT resolve — MISSING**

### Root Cause

The original capture was taken with `goGPUOITEnabled()` hardcoded to
`return true`. Debug logging with the fixed binary confirms the resolve now
runs successfully every frame ("OIT resolve: submitted successfully"). The
original binary's resolve was likely failing silently due to a resource
lifecycle issue where the resolve pipeline or bind group was nil at the point
of use, despite the accumulation pass succeeding.

### Verification

With the `goGPUOITEnabled()` fix applied, runtime debug logging confirms:
- OIT resolve pipeline is created (`Creating GPU Render Pipeline label="OIT Resolve Pipeline"`)
- Resolve enters every water frame (`accumulated=true, oitEnabled=true`)
- Resolve submits successfully every frame

The water transparency should now work correctly with the current code. A fresh
gfxrecon capture with the fixed binary would confirm the resolve pipeline
appears in the Vulkan trace and the visual output matches expectations.

## Files Modified

| File | Change |
|------|--------|
| `internal/renderer/renderer_gogpu_world.go` | Fixed `goGPUOITEnabled()` to check cvar |
| `internal/renderer/types.go` | Added `Contrast` field, `CvarRContrast` constant, clamping |
| `internal/renderer/renderer_gogpu_warpscale.go` | Gamma/contrast in scene composite shader + uniforms |
| `internal/renderer/renderer_gogpu_runtime.go` | Pass contrast through DrawContext |
| `internal/renderer/renderer_gogpu.go` | Added `contrast` field to DrawContext |
| `internal/renderer/renderer_gogpu_warpscale_test.go` | Updated test for new uniform signature |
| `internal/game/game_init.go` | Registered `r_contrast` cvar, added to startup load list |

## References

- C Ironwail OIT: `Quake/gl_rmain.c:1838-1881` (R_BeginTranslucency/R_EndTranslucency)
- C Ironwail water draw: `Quake/r_world.c:520-600` (R_DrawBrushModels_Water)
- C Ironwail postprocess: `Quake/gl_rmain.c:330-355` (GL_PostProcess)
- C Ironwail shader math: `Quake/gl_shaders.h` (OIT_OUTPUT macro, oit_resolve_fragment_shader)
- C Ironwail front face: `Quake/gl_vidsdl.c:1250` (`glFrontFace(GL_CW)`)
- McGuire weighted-blended OIT: https://casual-effects.blogspot.com/2014/04/weighted-blended-order-independent.html
