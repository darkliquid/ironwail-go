# Internals

## Logic

`StatusBar` eagerly resolves the HUD art it needs from `draw.Manager` (bars, weapons, ammo, armor, face variants, score graphics, and expansion-pack assets). Rendering then uses Quake's 320-wide logical layout to place bars, numerals, icons, inventory strips, and scoreboards. Pickup flashes are stateful across frames: `trackPickups` compares the current item bitmask with the previous one, records acquisition times, and lets `weaponFlashIndex` cycle through flash frames for one second at 10 fps. QuakeWorld mode reuses much of the same icon logic but redistributes pieces across dedicated canvases.

## Constraints

- Viewsize thresholds are critical: classic inventory is shown only below 110, classic/QW main bars only below 120, but multiplayer scoreboards can still appear when the main bar is hidden.
- Scoreboard rows sort by frags descending, then by lower `ClientIndex`, and empty names are filtered out.
- Invulnerability forces the displayed armor value to `666` and changes the armor icon to the disc when present.
- Expansion-pack weapon/item bits and slot-sharing rules (especially Hipnotic grenade/proximity behavior and Rogue weapon/ammo variants) are part of the module's correctness contract.

## Decisions

### Preserve Quake HUD art/layout behavior with asset-driven fallback paths

Observed decision:
- The status-bar code prefers picture assets for bars, numerals, and inventory artwork, but falls back to simpler character/fill rendering when some assets are unavailable.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- The status bar can still render in reduced form during tests or partial asset availability, while full runtime behavior preserves the richer Quake-specific HUD art path.
