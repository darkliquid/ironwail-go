# Responsibility

## Purpose

`menu/rendering-primitives` owns the shared drawing helpers used by multiple menu pages: menu text boxes, plaque/title framing, animated cursors, text glyph emission, and setup-player color translation.

## Owns

- 9-patch menu text box drawing with `box_*.lmp` assets.
- Plaque/title framing (`gfx/qplaque.lmp` plus optional banner pics).
- Animated sprite/text cursors.
- Shared menu text rendering with bright/normal glyph variants.
- Player preview recoloring for the setup screen.

## Does not own

- Which pages use these helpers or when they are invoked.
- Menu-state transitions or cvar/command side effects.
