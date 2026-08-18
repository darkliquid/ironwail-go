# Spike R1.4: sprite-atlas sub-rect drawing path

**Status:** RESOLVED (2026-08-18)
**Bead:** ironwail-go-teh.6 (M1.3)
**Gap:** 15

## Question

Status bar faces/weapons and 9-patch boxes draw sub-rects from composite WAD
pics (sprite atlases). gogpu/ui v0.1.54 has no `ImageRegionDrawer` (verified
absent in main and releases). Two paths: `image.RGBA.SubImage` slicing, or the
gg escape hatch (`widget.Canvas` concrete impl exposes `Context() *gg.Context`).

## Findings

1. **No `ImageRegionDrawer` in v0.1.54** (research 0002 §3, confirmed by
   search). The Canvas has `DrawImage(img image.Image, at geometry.Point)`
   which draws whole images.
2. **`image.RGBA.SubImage` is the natural fit.** The engine already converts
   palette-indexed QPic pixels to RGBA (`theme.QPicToImage`, renderer
   `ConvertPaletteToRGBA`). `(*image.RGBA).SubImage(r)` returns a view sharing
   the backing pixel buffer (no copy) that satisfies `image.Image`, so
   `canvas.DrawImage(sub, at)` draws exactly the sub-rect. Zero allocation
   beyond the sub-image header; the GPU upload happens once per whole atlas.
3. **The gg escape hatch** (`Context() *gg.Context` on the concrete canvas)
   supports `DrawImageEx`/sub-rect draws and gradients, but it bypasses the
   Canvas state (clip/transform) and is documented "advanced use". It remains
   the fallback for precise draws the Canvas cannot express.
4. **QPic.SubPic already exists** (`internal/image/wad.go:255`) for CPU-side
   slicing of palette-indexed pics; combining `SubPic` + `QPicToImage` gives a
   clean palette→RGBA sub-image for any atlas region.

## Decision

**Use `image.RGBA.SubImage` slicing** for WAD pic sub-rects (status bar
faces/weapons, 9-patch boxes): convert the whole atlas QPic to RGBA once via
`theme.QPicToImage`, then `SubImage` each region and `canvas.DrawImage`.
The gg escape hatch is reserved for cases the Canvas cannot express (gradients,
paths, precise transforms) and is not needed for the MVP HUD/menu pic draws.

## Acceptance

- Approach chosen: DONE (this note).
- No `ImageRegionDrawer` dependency: DONE — none exists in v0.1.54.
