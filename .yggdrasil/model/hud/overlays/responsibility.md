# Responsibility

## Purpose

`hud/overlays` owns the transient and alternate overlay elements layered on top of the main status presentation: centerprint/intermission text, crosshair, compact HUD, and simple glyph-based drawing helpers.

## Owns

- `Centerprint`, `Crosshair`, and `CompactHUD`.
- Glyph-based `DrawNumber` and `DrawString` helpers.
- Centerprint backgrounds, fade/reveal behavior, and intermission/finale overlays.
- Crosshair glyph selection from cvar values.
- Compact corner-HUD rendering and weapon/ammo-name selection.

## Does not own

- Top-level HUD style dispatch.
- Classic/QuakeWorld status-bar asset orchestration.
- The actual source of gameplay state or cvar registration outside overlay-specific expectations.
