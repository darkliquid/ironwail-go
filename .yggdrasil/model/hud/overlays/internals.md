# Internals

## Logic

The overlay helpers share one theme: they render smaller self-contained presentation rules without owning the full HUD pipeline. `DrawNumber` and `DrawString` emit raw Quake font glyphs. `Crosshair` translates the crosshair cvar into a stored glyph index and relies on the renderer's crosshair canvas transform for centering. `CompactHUD` draws a sparse corner overlay (health/armor, ammo, weapon abbreviation) and normalizes both classic weapon numbers and item-bit forms. `Centerprint` preloads optional art from `draw.Manager`, decides whether server or manual text is active, applies hold/fade rules, optionally reveals finale text progressively, and draws one of several background styles before emitting centered glyphs.

## Constraints

- Regular centerprint is suppressed while paused, but finale/intermission center text is still allowed.
- Newlines do not consume finale typewriter reveal budget.
- Centerprint background/text fade uses alpha-capable renderer interfaces when available and degrades to deterministic character dithering otherwise.
- Centerprint layout and compact HUD text assume fixed-width 8-pixel glyphs; rune width is measured with byte length rather than full Unicode display width.
- `compactScale` exists but is currently unused.

## Decisions

### Favor canvas and helper composition over specialized menu-only drawing APIs

Observed decision:
- Overlay helpers render through ordinary HUD/menu canvases and optional alpha-capable sub-interfaces instead of introducing a separate overlay renderer abstraction.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Overlay behavior remains backend-agnostic and integrates with existing renderer canvas transforms, but each helper must understand the relevant canvas/layout conventions directly.
