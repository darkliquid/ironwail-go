# Internals

## Logic

These helpers centralize repeated menu rendering patterns. The text box routine assembles Quake's box art as a simple 9-patch, the plaque/title helper lays down the standard frame used by many pages, cursor helpers animate either the spinning menu dot or a blinking arrow, and text rendering writes menu-space glyphs character-by-character while selecting bright or normal charset variants. `translateSetupPlayerPic` builds a new `QPic` with shirt/pants ranges recolored via the renderer's player-skin translation helper.

## Constraints

- Box widths are specified in 16-pixel columns while line counts are 8-pixel text rows.
- Cursor animation timing is wall-clock based rather than frame-count based.
- Text rendering assumes Quake's 0–255 glyph space; non-ASCII/unmapped runes are lossy.
- `translateSetupPlayerPic` allocates a fresh pixel buffer for each translated preview.

## Decisions

### Factor repeated Quake menu rendering motifs into shared helpers

Observed decision:
- The Go package extracts plaque/cursor/text-box/text helpers instead of duplicating C-style draw snippets across every menu screen.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Screen draw functions stay shorter and more declarative, while common menu art conventions remain consistent across pages.
