# Interface

## Main consumers

- runtime code that builds one HUD instance and feeds it per-frame state snapshots.
- game-visual integration that wants a single overlay entrypoint while still honoring Quake-style cvars and layout rules.

## Main surface

- `HUD`, `State`, `ScoreEntry`, `HUDStyle`
- `NewHUD`, `SetScreenSize`, `SetState`, `Style`, `Draw`
- `UpdateCrosshair`, `SetCenterprint`, `ClearCenterprint`, `IsActive`
- subordinate renderers: `StatusBar`, `Centerprint`, `CompactHUD`, `Crosshair`

## Contracts

- Callers provide a flattened `State`; the package does not reach back into client/server runtime objects.
- Rendering behavior is cvar-sensitive each frame, especially for HUD style, viewsize thresholds, scaling, and centerprint timing.
- The package draws through renderer canvases rather than backend-specific APIs.
