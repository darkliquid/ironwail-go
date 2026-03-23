# Responsibility

## Purpose

`input` owns the engine-wide abstraction for keyboard, mouse, and gamepad handling: key vocabulary, binding/routing semantics, per-frame input state, and backend-specific event ingestion.

## Owns

- The package-level boundary between engine-facing input semantics and platform backends.
- Separation between backend-neutral key/routing logic and the SDL3 implementation.
- Engine key naming/serialization conventions used by config files and bind commands.

## Does not own

- Gameplay/menu/console policy that decides what bound commands actually do.
- Renderer/window ownership beyond the backend interface boundary.
- Every platform adapter in the repo; renderer-coupled adapters remain outside this package.
