# Interface

## Main consumers

- runtime setup code that creates the HUD with a `draw.Manager`.
- per-frame visual code that updates state, resize information, and invokes `Draw`.

## Main surface

- `NewHUD`
- `SetScreenSize`
- `SetState`, `State`
- `Style`
- `Draw`
- `UpdateCrosshair`
- `SetCenterprint`, `ClearCenterprint`, `IsActive`

## Contracts

- `HUD.Draw` is safe on a nil `RenderContext` and otherwise always updates canvas parameters when the context supports them.
- Exactly one HUD style renderer is chosen for the non-intermission status layer each frame.
- Crosshair and centerprint are drawn after status-layer selection on dedicated canvases.
- `State.HideIntermissionOverlay` lets callers preserve `Intermission` state for crosshair/policy decisions while suppressing the visible intermission/finale overlay when gameplay input focus is not active.
