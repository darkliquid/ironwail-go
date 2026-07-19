# qbj2 Start — Water Lighting & Translucency Diagnosis

## Symptom

Water on qbj2 `start.bsp` renders **too bright** and **fully opaque** in ironwail-go,
while C Ironwail renders it correctly (translucent at 60% opacity with proper lighting).

## Prior Fix Attempt (commit a893941)

The previous commit `a893941` ("fix(renderer): align water translucency, lit water
fallback, and bspdiag tooling with C Ironwail") made several changes:

1. **`DeriveFaceFlags`** (`world/texture.go`): Removed `SurfDrawTiled` from liquid
   texture types (water/lava/slime/tele), keeping it only when `TEX_SPECIAL` is set
   on the texinfo. This matches C `gl_model.c:1362-1363`.

2. **`worldFaceHasLitWater`** (`world.go:359`): Added `SurfDrawTiled == 0` check,
   matching C's condition that lit water requires non-tiled surfaces.

3. **`gogpuWorldLightmapArrayBindGroupForFace`** (`world.go:381`): Added `useLitWater`
   flag and returns `litWater=1` with the fallback (black) lightmap when
   `LightmapIndex < 0` but the face is a liquid in a lit-water map.

4. **Shader** (`world_shaders_gogpu.go:573`): Rewrote the turbulent fragment shader
   to branch on `uniforms.litWater`: unlit path uses only texture + fog; lit path
   samples lightmap + dynamic lights + fullbright, matching C's `WORLDSHADER_WATER`.

5. **`hasValidLighting`** field: Added `lightmapSamplesHaveVariation` to detect
   degenerate (uniform) lightmap data and a `hasValidLighting` field on
   `faceLightmapSurface`. **However, this field is never used in any decision —
   it is only logged.** It is dead code.

6. **Fog uniform density**: Changed `fillWorldSceneUniformBytes` to stop
   double-applying `worldFogUniformDensity` (the conversion from cvar density to
   shader exp2 density).

7. **Fog state**: Added float-precision fog state tracking in `client_environment.go`
   and underscore-prefix handling in entity field parsing.

**Despite these changes, the water is still too bright and fully opaque.**

---

## Map Data Analysis (bspdiag + direct BSP inspection)

### Worldspawn entity fields

```
wateralpha    = ".6"       ← water should be 60% opaque (translucent)
fog_color     = ".3 .3 .32"
fog_density   = ".025"
```

### Texture table (liquid textures only)

| Miptex | Name            | Texinfo flags | TEX_SPECIAL? | Type      |
|--------|-----------------|---------------|--------------|-----------|
| 9      | `*waterskip`    | (unused)      | N/A          | Water     |
| 10     | `*watermurk3`   | **0**         | **NO**       | Water (6) |
| 18     | `*tele128_blu1` | **0**         | **NO**       | Tele (5)  |

**Critical**: Both used liquid textures have `TEX_SPECIAL = 0` in the BSP texinfo.
Per C `gl_model.c:1362-1363`, this means `SURF_DRAWTILED` is **NOT** set, and the
faces are candidates for **lit water** (lightmapped water).

### Liquid face census (520 total, with .lit sidecar applied)

| Texture         | VARIED lightmap | UNIFORM lightmap | NONE (LightOfs=-1) | Total |
|-----------------|-----------------|------------------|-------------------- |-------|
| `*watermurk3`   | 378             | 10               | 82                  | 470   |
| `*tele128_blu1` | 38              | 0                | 12                  | 50    |
| **Total**       | **416**         | **10**           | **94**              | **520**|

- **416 faces** have meaningful (varied) RGB lightmap data → properly lit water
- **10 faces** have uniform lightmap data (all 0, or all 1) → effectively black
- **94 faces** have `LightOfs = -1` → no lightmap data → black lightmap in C

### Liquid alpha resolution (verified via test)

```
HasWater=true Water=0.600
TransparentWaterSafe=true
Resolved: water=0.600 lava=0.600 slime=0.600 tele=1.000
```

The worldspawn `wateralpha=0.6` override IS parsed correctly.
`TransparentWaterSafe` IS true (map is water-vised).

---

## C Ironwail Reference (the correct implementation)

### Face flag derivation (`gl_model.c:1359-1368`)

```c
else if (TEXTYPE_ISLIQUID (texture->type))
{
    out->flags |= SURF_DRAWTURB;
    if (out->texinfo->flags & TEX_SPECIAL)
        out->flags |= SURF_DRAWTILED;          // tiled = unlit, no lightmap
    else if (out->samples && !loadmodel->haslitwater)
    {
        Con_DPrintf ("Map has lit water\n");
        loadmodel->haslitwater = true;          // mark model as having lit water
    }
    // ... set DRAWLAVA/DRAWSLIME/DRAWTELE/DRAWWATER
}
```

Key: `TEX_SPECIAL` is read **directly from the BSP texinfo flags** (`gl_model.c:1114`:
`out->flags = LittleLong(in->flags)`). It is NOT derived from the texture name.

### Lightmap allocation (`r_brush.c:GL_PackLitSurfaces`)

```c
if (surf->flags & SURF_DRAWTILED)
    continue;                                    // tiled = skip lightmap entirely
// ...
if (surf->samples)
    surf->lightmaptexturenum = AllocBlock(...);  // real lightmap block
else
    surf->lightmaptexturenum = blacklm;          // reserved black block
```

- `SURF_DRAWTILED` faces: **no lightmap at all** → standalone water shader (fullbright)
- Non-tiled liquid WITH samples: **real lightmap** → WORLDSHADER_WATER (lit)
- Non-tiled liquid WITHOUT samples (LightOfs=-1): **black lightmap block** →
  WORLDSHADER_WATER (samples black → black water × 2.0 = black)

### Program selection (`r_world.c:556-559`)

```c
if (cl.worldmodel->haslitwater && r_litwater.value)
    program = glprogs.world[...][WORLDSHADER_WATER];
else
    program = glprogs.water[...];
```

**Global choice**: if the model has ANY lit water AND `r_litwater` is on, ALL water
surfaces use the `WORLDSHADER_WATER` program. No per-face selection.

### WORLDSHADER_WATER fragment shader (`gl_shaders.h:604-726`)

```glsl
uv = uv * 2.0 + 0.125 * sin(uv.yx * (3.14159265 * 2.0) + Time);  // warp UV
vec4 result = texture(Tex, uv);                                    // sample texture
// ... sample lightmap, apply lightstyles ...
total_light *= 2.0;                                                // overbright
result.rgb = mix(result.rgb, result.rgb * total_light, result.a); // lit/unlit mix
result.rgb += fullbright;
result.rgb = ApplyFog(result.rgb, in_pos - EyePos);
result.a = in_alpha;  // water alpha REPLACES texture alpha
```

Key: `result.a = in_alpha` (not `result.a *= in_alpha`). The water alpha (0.6)
**replaces** the texture alpha in lit water mode.

### Standalone water shader (`gl_shaders.h:783-822`)

```glsl
vec2 uv = in_uv * 2.0 + 0.125 * sin(...);
vec4 result = texture(Tex, uv);
result.rgb = ApplyFog(result.rgb, in_pos);
result.a *= in_alpha;  // water alpha MULTIPLIES texture alpha
```

Key: `result.a *= in_alpha`. The water alpha **multiplies** the texture alpha.

### Water alpha computation (`gl_rmisc.c:268-278, 452-498`)

```c
map_wateralpha = (cl.worldmodel->contentstransparent & SURF_DRAWWATER)
                 ? r_wateralpha.value : 1;
// ... then worldspawn "wateralpha" key overrides map_wateralpha
```

`contentstransparent` is computed from PVS visibility analysis (`gl_model.c:1750-1841`):
water leaves that see non-water leaves → transparent. For qbj2, the map IS
water-vised, so `contentstransparent & SURF_DRAWWATER` is set, and
`map_wateralpha = r_wateralpha.value = 1`, then overridden by worldspawn to 0.6.

---

## Go Implementation Analysis

### What IS correct

1. **`DeriveFaceFlags`** (`world/texture.go:86`): Correctly sets `SurfDrawTiled`
   only when `TEX_SPECIAL` is set. Liquid types get `SurfDrawTurb` but NOT
   `SurfDrawTiled` when `TEX_SPECIAL=0`. ✓

2. **Liquid alpha resolution** (`world/liquid_alpha.go`): Correctly parses
   worldspawn `wateralpha` override (with `_` prefix support) and resolves to 0.6.
   `TransparentWaterSafe` is correctly computed. ✓

3. **Face classification** (`world/pass.go`): `FaceAlpha` returns 0.6 for water,
   `FacePass` returns `PassTranslucent` for alpha < 1. ✓

4. **Translucent liquid render path**: Translucent liquid faces are collected
   separately and rendered via `renderGoGPUSortedTranslucentFaceRendersHAL` with
   the `worldTranslucentTurbulentPipeline` (correct alpha blending). ✓

5. **Lightmap texture layout**: Pages stacked vertically with 1px padding.
   Vertex `LightmapLayer` and `LightmapCoord[1]` are post-processed to global
   coordinates (`world_geometry_gogpu.go:309-320`). ✓

6. **Lit water uniform**: `gogpuWorldLightmapArrayBindGroupForFace` returns
   `litWater=1` for liquid faces in a lit-water map, using the black fallback
   when `LightmapIndex < 0`. ✓

### What is SUSPECT / potentially wrong

#### Issue 1: `hasValidLighting` is dead code

`lightmapSamplesHaveVariation` and `faceLightmapSurface.hasValidLighting` are
computed but **never used in any decision**. `worldFaceHasLitWater` does not check
`hasValidLighting`. The previous fix added this field but forgot to wire it in.
Currently, faces with uniform/degenerate lightmap data (all 0, all 128) are still
treated as lit water, sampling a uniform lightmap → uniform darkness. This may or
may not be correct vs C (C also treats them as lit water with uniform lightmap →
uniform darkness, so this is actually parity-correct).

**Verdict**: Dead code, but not the cause of the bug. Should be removed or wired in
depending on intent. C does NOT check for variation — it just checks
`out->samples != NULL`.

#### Issue 2: Shader alpha formula differs from C lit water mode

Go shader (`world_shaders_gogpu.go:596`):
```wgsl
return vec4<f32>(fogged, sampled.a * uniforms.alpha);
```

C lit water (`gl_shaders.h:725`):
```glsl
result.a = in_alpha;  // REPLACES texture alpha
```

C unlit water (`gl_shaders.h:810`):
```glsl
result.a *= in_alpha;  // MULTIPLIES texture alpha
```

For liquid textures, `sampled.a = 1.0` (opaque, per `BuildMaterialTextureRGBA`
line 121-125), so `1.0 * 0.6 = 0.6` in both formulas. **This is not the cause**
for standard liquid textures. However, if the texture upload somehow produces
`sampled.a != 1.0` for liquid textures (e.g., a bug in `BuildMaterialTextureRGBA`
or atlas bleeding), the Go shader would produce wrong alpha while C would still
use 0.6.

**Verdict**: Likely not the cause, but the Go shader should use `uniforms.alpha`
(not `sampled.a * uniforms.alpha`) in the lit water path to exactly match C.

#### Issue 3: Shader lit water lighting formula

Go shader lit path (`world_shaders_gogpu.go:585-591`):
```wgsl
var totalLight = textureSample(worldLightmap, ...).rgb;
let dynamicLight = accumulateDynamicLights(...);
totalLight += max(min(dynamicLight, vec3<f32>(1.0) - totalLight), vec3<f32>(0.0));
let fullbright = textureSampleLevel(worldFullbrightTexture, ...);
let fullbrightColor = fullbright.rgb * fullbright.a;
lit = mix(sampled.rgb, sampled.rgb * totalLight * 2.0, sampled.a) + fullbrightColor;
```

C lit water (`gl_shaders.h:641-721`):
```glsl
vec4 lm0 = textureLod(LMTex, lmuv, 0.);
// ... lightstyles mixing (single/double/quad style) ...
total_light *= 2.0;
result.rgb = mix(result.rgb, result.rgb * total_light, result.a);
result.rgb += fullbright;
```

**Differences**:
1. Go does NOT apply lightstyles in the shader. The lightmap texture is pre-composited
   with lightstyle values on the CPU (`compositeWorldLightmapSurfaceRGBA`). C applies
   lightstyles in the shader via `in_styles` and multiple lightmap taps. This is a
   design difference, not a bug — both should produce the same result if the CPU
   compositing matches the GPU lightstyle mixing.

2. Go uses `fullbright.rgb * fullbright.a` while C uses just `fullbright.rgb` (the
   fullbright texture's alpha is not used in C's WATER mode — C just does
   `result.rgb += fullbright` where `fullbright = texture(FullbrightTex, uv).rgb`).
   For liquid textures, `BuildMaterialTextureRGBA` does NOT create a separate
   fullbright texture (`hasSeparateFullbright = false` for liquids, line 121-125),
   so `fullbrightTextures` should be nil/transparent. This means `fullbrightColor`
   should be 0. **Not the cause.**

3. Go's `mix(sampled.rgb, sampled.rgb * totalLight * 2.0, sampled.a)` with
   `sampled.a = 1` gives `sampled.rgb * totalLight * 2.0`. C's
   `mix(result.rgb, result.rgb * total_light, result.a)` with `result.a = 1` (from
   `texture(Tex, uv)` where liquid textures are opaque) gives
   `result.rgb * total_light`. **But C already did `total_light *= 2.0` at line 714**,
   so the effective formula is `result.rgb * (total_light * 2.0)` — same as Go. ✓

**Verdict**: The lighting formula matches C. Not the cause.

#### Issue 4: Opaque liquid pass alpha is hardcoded to 1

`world_render_gogpu.go:474`:
```go
writeWorldUniform(1, batch.key.litWater)  // alpha hardcoded to 1
```

This is for **opaque liquid batches** — faces where `worldFaceAlpha = 1` (alpha = 1,
fully opaque). For qbj2 with `wateralpha=0.6`, water faces should have alpha=0.6
and be classified as `PassTranslucent`, NOT `PassOpaque`. So they should NOT appear
in `opaqueLiquidBatches`.

**But**: If for some reason `liquidAlpha.water` is 1 at classification time (e.g.,
the worldspawn override is not applied, or `TransparentWaterSafe` is false), the
faces would be classified as opaque and rendered with alpha=1 in the opaque pass.
This would produce "fully opaque" water.

**Verdict**: This is the most likely cause IF the alpha resolution is failing at
classification time. Needs runtime verification.

#### Issue 5: Potential timing issue with `LiquidAlphaOverrides`

`worldLiquidAlphaSettingsForGeometry` reads `geom.LiquidAlphaOverrides` and
`geom.TransparentWaterSafe`, which are set at geometry build time
(`world_geometry_gogpu.go:55-56`). These are set once and don't change. The
classification (`shouldDrawGoGPUTranslucentLiquidFace`) uses the same settings.
So there should be no timing issue.

**However**: The batch cache (`storeGoGPUWorldBatchCacheEntry`) stores the
`translucentLiquidFaces` list. If the cache is populated on the first frame when
the alpha settings might be different (e.g., before cvars are loaded from config),
the cached classification could be wrong. The cache is keyed by `cameraLeafIndex`
and does NOT invalidate when cvar values change.

**Verdict**: Possible cause if the batch cache is populated before the worldspawn
override is applied. Needs investigation of cache invalidation.

#### Issue 6: Batch cache does not track liquid alpha settings

`gogpuWorldBatchCacheEntry` stores `translucentLiquid []WorldFace` but does NOT
store the liquid alpha settings used to classify them. If `r_wateralpha` changes
at runtime (or is loaded from config after the first frame), the cached
classification is stale.

Looking at `world_render_gogpu.go:229-236`: when `cacheHit`, the
`translucentLiquidFaces` are read directly from the cache without reclassification.
If the cache was built with `water=1` (before worldspawn override), the faces would
be in `opaqueLiquidBatches` (opaque) instead of `translucentLiquidFaces`.

**Verdict**: **HIGH PROBABILITY CAUSE.** The batch cache may be locking in the
wrong classification from the first frame.

---

## Proposed Fix Strategy

### Phase 1: Verify the root cause (runtime telemetry)

Before changing code, add temporary logging to verify which path the water faces
actually take at runtime:

1. Log `liquidAlpha` values at the start of `renderWorldInternal` and
   `collectGoGPUWorldTranslucentLiquidFaceRenders`.
2. Log the number of translucent vs opaque liquid faces classified.
3. Log whether the batch cache hit returns the correct translucent liquid count.
4. Log `uniforms.alpha` and `uniforms.litWater` values written for liquid batches.

### Phase 2: Fix the batch cache invalidation

If the cache is the problem, either:
- **Option A**: Include a hash/version of the liquid alpha settings in the cache
  key, so changing `r_wateralpha` or loading a new map invalidates the cache.
- **Option B**: Always reclassify liquid faces on cache hit (cheap — just iterate
  visible faces and check alpha).
- **Option C**: Remove the batch cache for liquid faces entirely (they're a small
  subset of faces).

### Phase 3: Align shader alpha with C lit water mode

In the turbulent fragment shader, change the lit water return to use
`uniforms.alpha` directly instead of `sampled.a * uniforms.alpha`, matching C's
`result.a = in_alpha`:

```wgsl
// Lit water: alpha = water alpha (replaces texture alpha), matching C
// Unlit water: alpha = texture alpha * water alpha, matching C
var finalAlpha = sampled.a * uniforms.alpha;
if (uniforms.litWater > 0.5) {
    finalAlpha = uniforms.alpha;
}
return vec4<f32>(fogged, finalAlpha);
```

### Phase 4: Remove dead code

Remove `hasValidLighting` field and `lightmapSamplesHaveVariation` function, OR
wire them in if a use case is found. C does NOT check for lightmap variation —
it treats all faces with `samples != NULL` as lit water. The Go code should match.

### Phase 5: Verify lit water lightstyle compositing

Verify that `compositeWorldLightmapSurfaceRGBA` produces the same result as C's
in-shader lightstyle mixing. The CPU compositing applies `worldLightstyleScale`
per style and sums them. C's shader does:
```glsl
total_light = in_styles.x * lm0.xyz;  // single style
// or
total_light = in_styles.x * lm0.xyz + in_styles.y * lm1.xyz;  // multi style
```
The Go approach pre-composites to a single RGB value per texel. This should match
if `worldLightstyleScale` returns the same values as C's `GetLightStyle`.

---

## Summary

| Finding | Impact | Action |
|---------|--------|--------|
| `hasValidLighting` is dead code | Low (not the cause) | Remove or wire in |
| Shader alpha formula differs from C lit water | Low (same result for opaque textures) | Align with C for safety |
| Batch cache may lock in wrong alpha classification | **High (likely cause)** | Investigate and fix cache invalidation |
| Opaque liquid pass hardcodes alpha=1 | Medium (correct for opaque, wrong if misclassified) | Verify classification is correct |
| Lit water lighting formula matches C | None | No action needed |
| Lightmap coordinate system is correct | None | No action needed |

### Issue 7: Batch cache is reset on map load (not the cause)

`resetGoGPUWorldBatchCache` is called in `UploadWorld` (`world_upload_gogpu.go:26`),
which runs when a new map loads. The `LiquidAlphaOverrides` and
`TransparentWaterSafe` are set at geometry build time (`BuildModelGeometry` line 55-56),
before the first frame. So the cache starts fresh with correct alpha settings.

**Verdict**: The batch cache is NOT the cause. The alpha settings are correct from
the first frame.

### Issue 8: Depth buffer state of opaque vs translucent liquid pipelines

- **Opaque turbulent pipeline** (`world_pipelines_gogpu.go:377`):
  `gogpuNonDecalDepthStencilState(true)` — **depth write ENABLED**, opaque blend
  (`One, Zero`).
- **Translucent turbulent pipeline** (`world_pipelines_gogpu.go:493`):
  `gogpuNonDecalDepthStencilState(false)` — **depth write DISABLED**, alpha blend
  (`SrcAlpha, OneMinusSrcAlpha`).

If water faces are incorrectly classified as opaque (alpha=1), they:
1. Render with opaque blend (alpha=1 → fully opaque) ✓ matches symptom
2. Write to depth buffer (blocking everything behind) ✓ matches symptom
3. Use `writeWorldUniform(1, batch.key.litWater)` — alpha hardcoded to 1 ✓ matches symptom

If `litWater` is also 0 for these faces (because the lightmap bind group is the
fallback and `useLitWater` evaluates to false for some reason), the shader does
`lit = sampled.rgb` (fullbright) → "too bright" ✓ matches symptom.

**Verdict**: The symptoms are fully consistent with water faces being classified
as `PassOpaque` instead of `PassTranslucent`. The question is WHY this happens
when the alpha resolution appears correct.

### Issue 9: Possible material/texture alpha issue

`BuildMaterialTextureRGBA` (`world/texture.go:121-125`) sets liquid textures to
`diffuse[base+3] = 255` (fully opaque). But what if the texture atlas or sampler
introduces alpha < 1 at edges due to linear filtering? The shader uses
`sampled.a * uniforms.alpha` for the final alpha. If `sampled.a` drops below 1 at
texture edges due to filtering, the alpha would be < 0.6, making the water MORE
transparent, not less. So this is NOT the cause of "fully opaque".

However, there's a related concern: the `worldTurbulentPipeline` (opaque) uses
`CullModeFront`. If the water faces are wound the wrong way, they could be
back-face culled and not render at all. But this would make water invisible, not
opaque. Not the cause.

---

## Refined Root Cause Hypothesis

After thorough analysis, the logic appears correct on paper:
- Alpha resolution: water=0.6 ✓
- Face classification: alpha < 1 → PassTranslucent ✓
- Translucent pipeline: alpha blend, no depth write ✓
- Shader: litWater=1 for lit-water maps, lightmap sampled ✓

**The most likely remaining cause is that the water faces are being classified as
opaque at runtime despite the alpha settings appearing correct.** This could be
due to:

1. **A subtle bug in `worldLiquidAlphaSettingsForGeometry`** where the cvar read
   or override application produces water=1 instead of 0.6 under certain
   conditions (e.g., cvar not yet registered, pkgCVars nil).

2. **A bug in the batch cache interaction** where the opaque liquid batches from
   a cache entry built with different settings are being used.

3. **A render pass ordering issue** where the translucent liquid faces are
   rendered but then overwritten by a subsequent opaque pass (e.g., brush entity
   water rendered opaquely on top).

4. **The `worldTurbulentPipeline` (opaque) being used for translucent faces**
   due to a pipeline selection bug in `renderGoGPUSortedTranslucentFaceRendersHAL`.

**The analysis strongly suggests adding runtime telemetry is necessary before
making further code changes**, as the static analysis shows the logic should be
correct but the symptom persists.

**Next step**: Add runtime telemetry (Phase 1) to confirm which render path and
alpha values the water faces actually take at runtime.
