# RESEARCH-0005: Upstream Feedback for gogpu Discussion #468 — Ironwail-Go BYO-Kit Validation

- **Status:** Integrated
- **Owner:** research (experiment findings from the gogpu/ui UI rewrite),
  2026-08-19
- **Date:** 2026-08-19
- **Related:** IRONWAILL-SPEC-001, ADRs 0001-0005, research 0002 §10,
  discussion #468 (https://github.com/orgs/gogpu/discussions/468)

---

## Purpose

Collate the Canvas-gap, Painter-pattern, GPUView, and BYO-kit authoring
findings from the ironwail-go gogpu/ui rewrite experiment, as the reference
validation case the gogpu org invited in discussion #468. This is the summary
document; the post itself is a condensed version of this.

## 1. Canvas gap findings (what was missing and how we worked around it)

The experiment validated the #468 Canvas-gap table against a real game UI:

| Gap | Finding | Workaround used |
| --- | --- | --- |
| Image regions / sprite atlas (`ImageRegionDrawer` absent) | Confirmed absent in v0.1.54 (main + releases). WAD pics (status bar faces/weapons, 9-patch boxes) are sub-rects of composite atlases. | `image.RGBA.SubImage` slicing: convert the whole atlas to RGBA once, `SubImage` each region, `canvas.DrawImage`. Zero-copy (shares backing pixels), one GPU upload per atlas. |
| Gradients / arbitrary paths | Not needed for the Quake MVP (flat palette fills + bitmap text), but would be needed for a modern HUD. | Not used; the gg escape hatch (`Context() *gg.Context`) is available. |
| Rotation / scale transforms | Not needed for the MVP (integer 320x200 menu geometry). | Widget layout + `scr_*` scales as widget transforms. |
| Blend modes | Not needed for the MVP. | Not used. |

**Key takeaway for upstream:** `ImageRegionDrawer` (or a `DrawImageSrcRect`
variant) would remove the `SubImage` boilerplate and is the single most
valuable Canvas addition for game UIs. The `SubImage` workaround is
acceptable but forces callers to manage atlas lifecycle themselves.

## 2. Painter-pattern fit for Quake visuals

The Painter pattern (ADR-034) maps well to Quake's visual style: Quake
widgets are flat palette fills + bitmap glyphs + WAD pics, all expressible as
custom painters. The experiment did not need stock `core/*` widgets at all
(BYO-kit validated): the menu, console, and HUD are custom widgets on
`widget.WidgetBase` + the conchars text widget.

- **Positive:** no stock theme needed; `theme.DefaultDark()` + Quake palette
  tokens + a `ThemeExtension` carried the glyph/alpha conventions.
- **Negative:** the Painter pattern's per-widget `ColorScheme` structs assume
  semantic colors; Quake's palette-index colors are awkward to map. A
  `palette-index` color type or raw-palette access would help game theming.

## 3. GPUView 3D-viewport findings

Feasible but deferred as a stretch milestone (see `notes/gpuview-spike.md`).
The engine already has offscreen scene-target machinery (waterwarp world
texture), proving world-into-texture is possible. The blocker for the
engine-owned render loop is that GPUView is only blitted by the `desktop`
render loop; a standalone engine must blit the widget's texture itself
(`DrawGPUTexture`). **Upstream ask:** document/standardize the
non-`desktop.Run` GPUView blit path so engines that keep their own loop can
composite a GPUView texture.

## 4. BYO-kit authoring experience (the validation the org wanted)

Overall the BYO-kit path works: `app` + `widget` + `render` (public
`render.NewCanvas`) have zero deps on `core/*`, and custom widgets are
straightforward on `widget.WidgetBase`.

**Pain points worth upstream attention:**

1. **Font registration is awkward for bitmap fonts.** ADR-0004 chose a
   `QuakeTextWidget` backed by the conchars atlas (8px advance) over building
   a pixel TTF, partly because `plugin.AssetLoader.LoadFont`'s registerer
   cannot reach the internal `GlobalFontRegistry`. A public font-registration
   path (or a bitmap-strike API) would let game engines use their own fonts
   without reaching into internals.
2. **`render.NewCanvas` is public but the concrete canvas's `Context()` is
   the only escape hatch.** It works, but a first-class
   `AdvancedCanvas.GGContext()` (the #468 Tier-2 proposal) would be cleaner
   than relying on the concrete type.
3. **The `desktop.Run` vs engine-owned-loop split is the biggest integration
   decision.** The engine-owned path (Architecture A) works, but the GPUView
   blit caveat and the single-slot EventSource/OnDraw conflict are the sharp
   edges a game engine hits first.

## 5. Recommended next steps for the org

- Add `ImageRegionDrawer` (or `DrawImageSrcRect`) to the Canvas — highest
  value for game UIs.
- Document the standalone-engine GPUView blit path.
- Expose a public font-registration path so bitmap/pixel fonts are usable.
- Keep the Painter pattern; consider a raw-palette color escape for
  palette-indexed game art.

## Source Index

- Experiment commits on `experiment/ui-rewrite` (internal/quakeui/*):
  theme, widgets, menu, console, hud, host, gateway, stack.
- `internal/quakeui/notes/{text-spike,sprite-atlas-spike,gpuview-spike}.md`
- ADRs 0001-0005; research 0002 §10; spec §9.
