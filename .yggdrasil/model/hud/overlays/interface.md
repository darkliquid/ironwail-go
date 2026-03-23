# Interface

## Main consumers

- `hud/runtime`, which delegates crosshair and centerprint drawing to these helpers and uses `CompactHUD` for the alternate HUD style.
- tests that validate glyph selection, fade behavior, and helper rendering contracts.

## Main surface

- `NewCenterprint`, `SetMessage`, `Clear`, `IsActive`, `Draw`
- `Crosshair.UpdateCvar`, `Crosshair.Draw`
- `NewCompactHUD`, `CompactHUD.Draw`
- `DrawNumber`, `DrawString`

## Contracts

- Crosshair selection mirrors Ironwail's cvar semantics: `0` disables, `1` is `'+'`, values greater than `1` use the dot glyph, and negative values select custom glyph indices.
- Normal centerprint respects hold/fade timing, but finale intermissions use the typewriter reveal path instead.
- Compact HUD only presents a minimal subset of stats and accepts both impulse-style and bitmask-style weapon identifiers.
