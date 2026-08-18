# ADR-0004: TTF Text Rendering from Rasterized Conchars

**Status:** Accepted (2026-08-18)
**Deciders:** darkliquid (Stage 1: full TTF), lifecycle driver
**Date:** 2026-08-18
**Related:** IRONWAIL-SPEC-001 §1.4/§2/§4.3; research 0002 §3;
gap log row 8

## Context and Problem Statement

Quake text is 8x8 bitmap glyphs from the `conchars` WAD lump (128x128,
16x16 grid), drawn at 8px per char with a high-bit color convention (char
+128 = bright glyph row). gogpu/ui renders text via TrueType fonts (embedded
Inter; custom TTF via `GlobalFontRegistry`); there is no bitmap-glyph API.
The Canvas gap analysis shows no `ImageRegionDrawer` in v0.1.54 either — so
per-glyph draws from the conchars atlas would need image slicing per char.

## Decision Drivers

- gogpu/ui's text pipeline is TTF; `DrawStyledText`/`TextStyle` and
  `MeasureStyledText` all resolve TTF faces.
- Retro per-glyph draws mean hundreds of `DrawImage` calls per frame and
  re-implementing text layout (kerning, wrapping, measure) by hand.
- Exact pixel parity is explicitly NOT required for the experiment
  (DoD #5: "behavioral, not pixel-exact").

## Considered Options

1. **Per-glyph image draws from conchars atlas** — pros: zero text-sys
   changes, exact retro look, reuse `getCharPic` logic. Cons: hand-rolled
   layout/measure; no Search order; hundreds of image draws; keeps the
   engine-specific text path alive inside quakeui (contradicts BYO-kit
   spirit: `widget` + std text).
2. **Hybrid (conchars for menu/console, TTF elsewhere)** — pros: retro text
   where it matters most (parity-wise) and TTF elsewhere. Cons: two text
   systems, two measure paths, more code; parity is still broken for
   console/menu text anyway (8px integer scaling disappears).
3. **Full TTF: rasterize conchars into a pixel TTF** — chosen. Map each
   conchars glyph (0-255, both color rows) to a Private-Use-Area (PUA)
   codepoint (e.g. U+E000 + num, and U+E100 + num for the bright row);
   build a 16x16 grid TTF (or use bitmap strikes / embedded pixel font)
   at 8px; register via `GlobalFontRegistry.Register("quake-conchars",
   Regular, Normal, ttfBytes)`. `DrawStyledText` with
   `FontFamily: "quake-conchars"` then renders any Quake string. High-bit
   color row becomes a pre-transform (map char to PUA pair on write) or a
   dual-color glyph set.
   - Implementation note: building a TTF from scratch is heavy (font
     tables, cmap, glyf/loca). Alternative accepted variant: keep conchars
     **pixel data** and expose it via a custom `TextModeBitmap` path or a
     `QuakeTextWidget` that measures with a width table (8px/char) and
     draws each rune from the atlas into the gg canvas — i.e. a widget-
     local renderer, but *only for conchars text* (menu/console/HUD text);
     all *layout* (Box, alignment, wrap) still comes from gogpu/ui
     primitives/widgets so the engine never re-implements layout.

## Decision Outcome

Full-TTF direction with the pragmatic variant: **rasterize conchars to a
TTF-compatible pixel font OR implement a `QuakeTextWidget` backed by the
conchars atlas with a static 8px advance table** — the plan will spike
option (a) (real pixel-TTF build, using e.g. a TTF authoring lib or
embedding a pre-built 8px bitmap TTF) and fall back to (b)
(`QuakeTextWidget` + atlas slicing via `image.RGBA.SubImage`) if the TTF
build proves too costly. Either way:
- All engine text draws route through `widget`-level text (never raw
  canvas per-char loops outside the widget).
- **Font registration uses the public `plugin.AssetLoader.LoadFont`
  path** (`plugin.MemoryAssetLoader{}.LoadFont("quake-conchars", ttfBytes)`
  → `SetFontRegisterer` → global registry) — `ui/internal/render`
  `GlobalFontRegistry` is unimportable (Review 2 fix).
- The high-bit color row maps to the PUA bright set (or a second fill
  color via ThemeExtension tokens).
- `MeasureStyledText` gives correct widths (8px/char via advance table).

- **Positive:** one text system; scavenges gogpu/ui layout; parity target
  is behavioral; validates the Canvas-gap feedback upstream wants
  (sprite/atlas handling).
- **Negative:** loses exact pixel parity (accepted); pixel-TTF build is a
  spike risk; bright-row handling needs care (dual color sets).

**T1 spike outcome (2026-08-18): option (b) chosen.** The pixel-TTF build
requires authoring a full TTF (font tables, cmap, glyf/loca) with no
in-repo dependency, and the `plugin.AssetLoader` registerer still cannot
reach the internal `GlobalFontRegistry` to make the font usable. The
`QuakeTextWidget` (conchars atlas + 8px advance + `image.RGBA.SubImage`
glyph draws) avoids both, keeps layout in gogpu/ui, and preserves the
retro 8x8 look. The pixel-TTF variant stays a documented alternative if
upstream later exposes a public font-registration path.

## Links

- IRONWAIL-SPEC-001 §1.4, §2, §4.3; research 0002 §3; ADR-0001 (gate);
  gap log 8

## Review Log

### Stage 5 — Review 2 (2026-08-18)

Verdict: APPROVED WITH FIX. Review found the font-registration mechanism was
underspecified: `render.GlobalFontRegistry` is in an **internal** package
(`ui/internal/render/fontregistry.go`) and cannot be imported by the engine.
Resolution: register the pixel TTF via the public `plugin.AssetLoader`
(`plugin.MemoryAssetLoader{}.LoadFont("quake-conchars", ttfBytes)`,
plugin/assets.go:141, wired to the global registry via
`SetFontRegisterer`, plugin/assets.go:124). Both option variants (TTF build
vs QuakeTextWidget atlas) keep this registration path. Also note: bitmap
text via TextModeBitmap needs the canvas text-mode path (gg supports
bitmap strikes); the spike confirms before committing.
