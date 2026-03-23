# Responsibility

## Purpose

`hud/status-bar` owns the Quake-style bottom-of-screen HUD presentations: the classic status bar, QuakeWorld layout, inventory strips, multiplayer scoreboards, and expansion-pack-specific item/weapon variants.

## Owns

- `StatusBar` asset loading and cached HUD picture state.
- Classic and QuakeWorld status-bar rendering.
- Inventory, ammo, armor, face, item, and scoreboard drawing.
- Pickup flash tracking and weapon-icon flash selection.
- Hipnotic and Rogue-specific status/inventory branches.

## Does not own

- HUD style selection between classic, compact, and QuakeWorld modes.
- Manual centerprint timing or crosshair rules.
- Client/gameplay state collection beyond reading the supplied `State`.
