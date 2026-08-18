# Spike T1: pixel-TTF vs QuakeTextWidget for conchars text

**Status:** RESOLVED (2026-08-18)
**Bead:** ironwail-go-teh.4 (M1.1)
**ADR:** 0004 (TTF text from rasterized conchars)
**Gap:** 17

## Question

Quake text is 8x8 bitmap glyphs from the `conchars` WAD lump (128x128,
16x16 grid), drawn at 8px per char with a high-bit color convention
(char +128 = bright glyph row). gogpu/ui renders text via TrueType fonts.
Two options: (a) build a pixel TTF from conchars with a PUA rune mapping,
or (b) implement a `QuakeTextWidget` backed by the conchars atlas with an
8px advance table.

## Findings

1. **Pixel-TTF build is heavy.** A real TTF needs font tables, cmap,
   glyf/loca, and a header — hundreds of lines of binary authoring, or an
   external font-authoring dependency. No such dependency exists in the
   module graph, and adding one for an 8px bitmap font is disproportionate.
2. **`ui/internal/render` is unimportable.** ADR-0004's review found
   `render.GlobalFontRegistry` lives in an internal package; the public path
   is `plugin.AssetLoader.LoadFont` → `SetFontRegisterer`, but the registerer
   callback itself still needs the internal registry to do anything useful.
   So even option (a) cannot register the font without either reaching into
   internals or relying on a registerer the engine cannot provide.
3. **The conchars atlas is already engine-owned.** `draw.Manager.ConcharsData()`
   returns the raw 128x128 indexed pixels. The renderer already extracts 8x8
   glyphs via `getCharPic`/`SubPic`. A widget can do the same and draw each
   glyph as an `image.RGBA.SubImage` slice (or the gg escape hatch).
4. **Layout stays in gogpu/ui.** The QuakeTextWidget measures with a static
   8px/char advance table and draws each rune from the atlas; all Box/align/
   wrap layout still comes from gogpu/ui primitives, so the engine never
   re-implements layout (ADR-0004's core requirement).

## Decision

**Option (b): QuakeTextWidget backed by the conchars atlas** with a static
8px advance table, per-glyph `image.RGBA.SubImage` draws. This avoids the
TTF-authoring cost and the unimportable-registry problem, keeps layout in
gogpu/ui, and preserves the retro 8x8 look. The bright row (char +128) is
handled by mapping to the alternate glyph row and/or a bright fill color via
the quakeui ThemeExtension (`BrightRow`).

The pixel-TTF variant (a) remains a documented alternative if upstream later
exposes a public font-registration path (the `plugin.AssetLoader` bridge
matures) — at which point the atlas widget can be swapped for a TTF family
without touching layout.

## Acceptance

- Decision recorded: DONE (this note).
- Approach pinned before M2.2: DONE — M2.2 implements `QuakeTextWidget`.
