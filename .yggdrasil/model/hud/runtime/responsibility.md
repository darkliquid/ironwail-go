# Responsibility

## Purpose

`hud/runtime` owns the main `HUD` facade: the caller-facing API that stores overlay state, reads style-related cvars, configures canvas transforms, and dispatches drawing to child HUD components.

## Owns

- `HUD`, `State`, `ScoreEntry`, and `HUDStyle`.
- HUD construction and child component wiring.
- Screen-size and state snapshot storage.
- Style selection and top-level draw orchestration.
- Integration-facing helpers for crosshair and manual centerprint control.

## Does not own

- Detailed status-bar asset/layout rules.
- Centerprint timing/reveal/background rules.
- Backend-specific drawing implementation beyond `renderer.RenderContext` usage.
