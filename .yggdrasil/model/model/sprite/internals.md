# Internals

## Logic

`LoadSprite` reads the sprite header, validates the file, allocates the top-level `MSprite`, and then parses each logical frame descriptor. Single frames become `MSpriteFrame` values with origin-derived extents and raw pixels. Grouped and angled frames become `MSpriteGroup` values containing validated interval schedules and individual `MSpriteFrame` payloads. `loadSpriteFrame` also computes `SMax` and `TMax` by dividing the real dimensions by the next power-of-two padding, preserving Quake's padded texture-coordinate semantics.

## Constraints

- Sprite files must have positive width, height, and frame count.
- Individual frame dimensions and pixel buffer sizes must be valid and non-zero.
- Group intervals must all be greater than zero.
- `SpriteFrameAngled` groups are hard-required to have 8 directional frames.
- `padConditional` returns 1 for non-positive values and otherwise rounds up to the next power of two.

## Decisions

### Represent grouped sprite frames as a tagged union instead of splitting the top-level slice by type

Observed decision:
- Each logical sprite frame stores a `Type` plus an `interface{}` payload that is either a single frame or a group.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- The runtime shape mirrors Quake's mixed single/group frame model, but downstream code must type-switch carefully and tests need to lock in those payload expectations.
