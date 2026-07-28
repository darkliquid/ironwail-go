# Materials System — Known Issues and Diagnostics

> **Status:** The button texture frame bug is **fixed** (commit `aa17df6`).
> The texture atlas overflow (>256 textures on BSP2 maps) is **open**. The
> liquid lighting question is **partially addressed** (lightmap fallback
> fixed, but lit water on large maps is not fully verified). This document
> consolidates `TEXTURE_ATLAS_DIAGNOSTIC_PLAN.md` and
> `qbj2_render_issues.md`.

## Materials Buffer Architecture

The materials buffer (`r.worldMaterialsBuffer`) is a GPU **uniform buffer**
that the WGSL fragment shader reads as `materials[MaterialID]`. Each
`MaterialData` struct is 32 bytes (atlas bounds + layer + flags). The buffer is
created at a fixed size of `256 * 32 = 8192` bytes
(`world_upload_gogpu.go:323`). The WGSL shader declares it as
`array<MaterialData, 256>` (`world_shaders_gogpu.go:128`).

For world faces, `updateWorldMaterialsBuffer` rewrites it each frame with
frame-0 animations. A separate frame-1 buffer (`worldMaterialsBufferFrame1`)
handles pressed button textures (see below).

The shader indexes into it using the per-vertex `materialID` attribute to look
up `atlasBounds` and `layer` for texture sampling.

### Per-Entity Frame Animation (Fixed)

**Problem:** QuakeC sets `self.Frame = 1` when a button is pressed, but the
renderer hardcoded `frame=0` in `animateWorldMaterials` and never bound the
alternate texture chain (`+Abutton`/`+Bbutton`).

**Fix (commit `aa17df6`):** A separate frame-1 GPU materials buffer + bind
group is created at world upload (and for external BSP models). It is updated
each frame with `AlternateAnims` selected. All brush entity render loops
(opaque, alpha-test, translucent) now select the frame-1 bind group when the
entity's `frame != 0`, so pressed buttons and activated switches show their
alternate textures.

**Key files:**
- `internal/renderer/world_material_gogpu.go:24` — `animateWorldMaterials`
  (now takes `frame` parameter)
- `internal/renderer/world_upload_gogpu.go:323-365` — frame-0 and frame-1
  materials buffers
- `internal/renderer/world_gogpu_brush_render.go` — brush entity render loops
  (selects frame-1 bind group when `draw.frame != 0`)
- `internal/renderer/world_gogpu_translucent.go` — translucent brush loops
  (same treatment)

## Open Issue: Texture Atlas Overflow (>256 Textures)

### Problem

Small maps (id1 start) render correctly. Large BSP2 maps (qbj2 start) render
with incorrect textures. The materials uniform buffer is hardcoded to 256
entries, but `baseMaterials` is allocated as `textureCount + 2` with no
clamping. When a map has more than 254 textures:

1. `uploadWorldMaterialTextures` allocates `baseMaterials` with
   `textureCount+2` slots (`world_resources_gogpu.go:486`)
2. GPU buffer created with capacity for exactly 256 materials
   (`world_upload_gogpu.go:323`: `Size: 256 * 32`)
3. WGSL shader declares `array<MaterialData, 256>`
   (`world_shaders_gogpu.go:128`)
4. `queue.WriteBuffer` writes all `textureCount+2` entries into the 256-slot
   buffer — **silent buffer overflow**
5. Shader indexes `materials[materialID]` where `materialID` can be
   `textureCount+1` — **out-of-bounds GPU array access**

### Proposed Fix

Make the materials buffer size dynamic (sized to `textureCount + 2`), or use
a `var<storage, read>` buffer instead of `var<uniform>` to avoid the 256-entry
uniform buffer limit. The WGSL shader array size must match the buffer.

### Diagnostic Plan (Not Yet Implemented)

**Phase 1 (P0): Material buffer capacity audit.** Add WARN logs when
`len(baseMaterials) > 256` in:
- `uploadWorldMaterialTextures` (`world_resources_gogpu.go`)
- `world_upload_gogpu.go` before `queue.WriteBuffer`
- `updateWorldMaterialsBuffer` (`world_material_gogpu.go:61`)
- External brush path (`renderer_gogpu_worldstate.go`)

**Phase 2 (P0): MaterialID range validation.** Histogram of `materialID`
values in `BuildModelGeometry` (`world_geometry_gogpu.go`). WARN if
`max >= 256`.

**Phase 3 (P1): Atlas layer distribution telemetry.** Per-layer breakdown,
bounds cross-check, GPU limit verification.

**Phase 4 (P2): Animation chain integrity.** Verify animated texture
remapping doesn't produce out-of-range indices.

**Phase 5 (P1): Structured first-frame material dump.** CSV/JSON of every
material entry + atlas layer PNG exports. Gated by env var
`IRONWAIL_DEBUG_MATERIAL_DUMP=1`.

**Phase 6 (P2): Debug shader variants.** Fragment shader outputs `materialID`
as color, gated by `r_debug_materials` cvar.

### Key Files

| File | Role |
| --- | --- |
| `internal/renderer/world_resources_gogpu.go` | Atlas construction, material table |
| `internal/renderer/world_upload_gogpu.go:323` | GPU buffer creation (hardcoded 256) |
| `internal/renderer/world_material_gogpu.go` | Per-frame animation, buffer update |
| `internal/renderer/world_geometry_gogpu.go` | MaterialID assignment per vertex |
| `internal/renderer/world_shaders_gogpu.go:128` | WGSL shader (`array<MaterialData, 256>`) |
| `internal/renderer/renderer_gogpu_worldstate.go` | External brush material buffer |
| `internal/renderer/surface/surface.go` | Animation chain construction |

## Liquid Lighting

### Background

C (GLQuake) never allocates lightmaps for `SURF_DRAWTURB` surfaces — liquids
are always rendered fullbright. Ironwail added optional lit water via
`r_litwater`. The Go implementation samples the lightmap when `litWater > 0.5`
in the WGSL uniform, defaulting to `vec3<f32>(0.5)` (fullbright when
`×2.0 = 1.0`).

### Fixed Issues

- **Fallback lightmap dimension mismatch:** Created as
  `TextureViewDimension2D` but shader declared `texture_2d_array<f32>`.
  WebGPU rejected it, silently defaulting to fullbright white → `×2.0`
  overbright. Fixed by using `createWorldSolidTextureArray`
  (`TextureViewDimension2DArray`).
- **`r_telealpha` default:** Was `1` with `FlagArchive`, forcing teleporter
  textures to alpha=1.0 (opaque). Fixed: default=0, `FlagNone`.

### Open Question

Whether `r_litwater=1` produces correct lighting on all liquid faces in BSP2
maps with potentially invalid lightmap data is not fully verified. The
`bspdiag liquids` tool can inspect this:

```
bspdiag liquids <quake_dir> <map.bsp> [gamedir]
```

Reports per-face: texinfo flags, `LightOfs`, lightmap sample statistics
(`VARIED`/`UNIFORM`/`NONE`), and resolved per-liquid alpha settings.

## The Lift Teleport (Resolved)

The qbj2 lift issue was originally listed alongside the materials issues. It
was a trigger activation bug caused by the QCVM sync problem — not a
materials issue. The root cause and fix are documented in
`docs/QCVM_ENTITY_SYNC.md`. The lift is a `func_train` (not `func_plat`)
named `lift_main` following path_corners. The full activation chain:
trigger_multiple → func_button → SUB_UseTargets → func_train. The train
wasn't moving because `executeQCFunction` didn't sync pusher mutations back
to Go. Fixed in commit `fe9e43c`.
