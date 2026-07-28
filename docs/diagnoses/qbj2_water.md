# Water Translucency — Diagnosis and Resolution

> **Status:** Resolved (commit `6802fc5`, 2026-07-27). This document consolidates
> the five prior diagnosis docs into a single record of what was wrong, what
> was tried, and what the fix was.

## Problem

Water on the qbj2 (BSP2 large map) start area appeared opaque instead of
translucent. The worldspawn `wateralpha=0.6` setting was not taking effect
visually — water rendered at 100% opacity.

## Root Causes (3, all fixed)

### 1. Worldspawn `wateralpha` override bypassed

`ResolveLiquidAlphaSettings` only applied the worldspawn `wateralpha` override
when the `r_wateralpha` cvar was exactly `1.0`. A stale user config value
(`0.35`) from `~/.ironwail/` prevented the map's `wateralpha=0.6` from taking
effect.

**Fix:** `ResolveLiquidAlphaSettings` now always applies worldspawn overrides,
matching C Ironwail's `R_ParseWorldspawn` behavior. File:
`internal/renderer/world/liquid_alpha.go`.

### 2. Vulkan swapchain discard between command buffer submits

The original architecture split the frame into multiple `queue.Submit()`
calls. The translucent water pass opened a new render pass with `LoadOpLoad`
on the render target after the world pass had already submitted. Vulkan
drivers may discard the framebuffer contents between separate submits, so the
translucent water was blending over a black destination instead of the opaque
geometry already drawn.

**Fix:** Translucent liquid faces are now drawn **within the world render pass
itself** — after opaque geometry, switching to the translucent turbulent
pipeline — matching C Ironwail's `R_DrawWater(true)` running in the same
framebuffer as `R_DrawWater(false)`. File:
`internal/renderer/world_render_gogpu.go:553+`.

### 3. Uniform buffer offset collision

The translucent water uniform (`alpha=0.6`) was overwriting the opaque uniform
(`alpha=1.0`) at offset 0 because both used the same fixed uniform buffer slot.

**Fix:** Dynamic uniform buffer offsets via `allocateUniformBuffer`. The
dynamic range is uploaded to GPU before encoding draw commands, and the offset
is passed via `SetBindGroup` dynamic offsets. File:
`internal/renderer/world_render_gogpu.go:558-602`.

## Earlier Root Causes (also fixed in prior commits)

These were discovered and fixed during the investigation before the final fix:

- **Fallback lightmap dimension mismatch:** Created as `TextureViewDimension2D`
  but the shader declared `texture_2d_array<f32>`. WebGPU rejected it, silently
  defaulting to fullbright white → `×2.0` overbright. Fixed by using
  `createWorldSolidTextureArray` (`TextureViewDimension2DArray`).
- **`r_telealpha` default=1 with `FlagArchive`:** Forced teleporter textures to
  alpha=1.0 (opaque). Fixed: default=0, `FlagNone`.
- **FatPVS not used:** Single-leaf PVS culled underwater geometry. Fixed: `FatPVS`
  (from C's `SV_FatPVS`) triggered when camera leaf contains water faces.
- **BSP2 HeadNode traversal bug:** `FatPVS`/`PointInLeaf` started at node 0
  (submodel) instead of `Models[0].HeadNode[0]` — critical for BSP2 maps. Fixed
  in `internal/bsp/tree.go`.
- **`SetCVarSystem` forwarding bug:** `worldimpl.pkgCVars` was nil → `ReadAlphaCvar`
  always returned default. Fixed.
- **Unregistered liquid alpha cvars:** `r_wateralpha`, `r_telealpha`, etc. were
  not registered in `game_init.go`. Fixed.

## C Reference Architecture

C Ironwail (OpenGL) renders the entire frame to a single framebuffer within one
`R_RenderView` call. There are no intermediate command buffer submits. The
render order is:

1. `R_DrawEntitiesOnList(false)` — opaque world geometry + opaque entities
2. `R_DrawWater(false)` — **opaque water** (blend=OPAQUE, depth write=ON)
   — only draws water faces whose alpha is 1.0
3. `R_BeginTranslucency()` — set up translucent mode
4. `R_DrawWater(true)` — **translucent water** (blend=ALPHA, depth write=OFF)
   — only draws water faces whose alpha is < 1.0
5. `R_DrawEntitiesOnList(true)` — translucent entities
6. `R_EndTranslucency()`

Key principle: **no face is drawn both opaquely and translucently.** The split
is by alpha value, not by pass. Both passes use the same framebuffer.

### C's water shader

- **Lit water (`WORLDSHADER_WATER`):** `result.a = in_alpha` (replaces texture
  alpha). Used globally when `haslitwater && r_litwater` are both true.
- **Unlit water (standalone):** `result.a *= in_alpha` (multiplies).
- For opaque textures (alpha=1.0), both produce the same result.

### C's lightmap allocation for liquids

C (GLQuake) never allocates lightmaps for `SURF_DRAWTURB` surfaces — liquids
are always rendered fullbright. Ironwail added optional lit water via
`r_litwater`. Go's implementation samples the lightmap when `litWater > 0.5`
in the WGSL uniform, defaulting to `vec3<f32>(0.5)` (fullbright when
`×2.0 = 1.0`).

## Debugging Methodology (Key Learnings)

### The misleading screenshot stub

Early diagnosis was led astray by `CaptureScreenshot` being a stub writing
hardcoded `RGB(20,20,46)` instead of doing real GPU readback. **Lesson:**
always verify that diagnostic tools actually do what they claim. A stub
screenshot producing a plausible-looking color is worse than no screenshot at
all.

After implementing real WebGPU texture readback, the captured image showed
14,672 unique colors near-parity with C — confirming the renderer was working
correctly and the "opaque water" was real, not a screenshot artifact.

### The mathematical proof

When the stub was suspected, the proof was: `RGB(20,20,46)` = exactly
`0.6 × waterColor + 0.4 × black`. This meant the destination framebuffer was
black (discarded), not containing the opaque geometry. If water looked opaque
but the math says it should be dark, the framebuffer was being discarded.

### Automated raster test gap

The automated raster test (`TestQBJ2WaterTranslucencyRaster`) uses
`PARITY_RUN=1` which forces the scene render target, so it passes even when
in-game rendering fails. **Lesson:** test harness paths can diverge from
production paths. Always verify the test exercises the same code path.

### Telemetry design pattern

The `r_debug_water` cvar (`internal/renderer/water_debug.go`,
`internal/renderer/types.go`) provides per-frame liquid face telemetry gated
behind a cvar. Log lines prefixed `[rwater]` cover:

- Face classification (opaque/translucent)
- Alpha resolution (worldspawn override, cvar, `TransparentWaterSafe`)
- Pipeline selection (turbulent, translucent-turbulent)
- Per-batch alpha and lightmap settings
- Draw call counts

**Interpretation matrix:**

| Scenario | Symptom | Meaning |
| --- | --- | --- |
| A | Faces classified opaque | `worldFaceAlpha` returning 1.0; check worldspawn/cvar |
| B | Alpha resolution fails | `ResolveLiquidAlphaSettings` returning wrong value |
| C | Correct CPU, wrong GPU | Uniform buffer or pipeline mismatch |
| D | Lit water not enabled | `litWater` uniform or lightmap data issue |

### `bspdiag liquids` command

The `bspdiag liquids` tool (`cmd/bspdiag/liquids.go`) inspects liquid faces
offline without running the engine:

```
bspdiag liquids <quake_dir> <map.bsp> [gamedir]
```

Reports: per-face texinfo flags, `LightOfs`, lightmap sample statistics
(`VARIED`/`UNIFORM`/`NONE`), and resolved per-liquid alpha settings
(worldspawn overrides + `TransparentWaterSafe`).

## Files Changed (Key References)

| File | What changed |
| --- | --- |
| `internal/renderer/world/liquid_alpha.go` | Always apply worldspawn overrides |
| `internal/renderer/world_render_gogpu.go:553+` | Draw translucent water in world render pass |
| `internal/renderer/world_render_gogpu.go:558-602` | Dynamic uniform buffer offsets for translucent alpha |
| `internal/renderer/world_lightmap_gogpu.go` | Fixed lightmap array texture view dimension |
| `internal/renderer/cvars.go` | Liquid alpha cvar registration |
| `internal/game/game_init.go` | Registered `r_wateralpha`, `r_telealpha`, etc. |
| `internal/bsp/tree.go` | BSP2 HeadNode traversal fix for FatPVS/PointInLeaf |
| `internal/renderer/water_debug.go` | `r_debug_water` telemetry |
| `internal/renderer/types.go` | `r_debug_water` cvar constant |

## What Remains Open

- **Underwater warp:** The screen-space sinusoidal warp when the camera is
  underwater uses a separate scene composite pass (`warpscale_gogpu.go`). This
  is architecturally separate from the translucency fix and is not known to
  have issues.
- **Lit water on large maps:** Whether `r_litwater=1` produces correct lighting
  on all liquid faces in BSP2 maps with potentially invalid lightmap data is
  not fully verified. The `bspdiag liquids` tool can inspect this.
