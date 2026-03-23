# Responsibility

## Purpose

`hud` owns the 2D gameplay overlay layer that turns flattened client/gameplay state into Quake-style on-screen presentation.

## Owns

- The package-level boundary between gameplay state and overlay rendering.
- HUD style selection and the package's composition of status bar, compact HUD, crosshair, and centerprint behavior.
- The separation between orchestration, status-bar logic, and transient overlay components.

## Does not own

- Collecting live state from client/server/network subsystems.
- Render-backend implementation details beyond `renderer.RenderContext` contracts.
- Asset loading/parsing beyond consuming pictures from `draw.Manager`.
