# Responsibility

## Purpose

`input` owns the engine-wide abstraction for keyboard, mouse, and gamepad handling: key vocabulary, binding/routing semantics, per-frame input state, and the backend contract used by runtime event sources.

## Owns

- The package-level boundary between engine-facing input semantics and concrete platform/window backends.
- Separation between backend-neutral key/routing logic and executable- or renderer-owned backend implementations.
- Engine key naming/serialization conventions used by config files and bind commands.

## Does not own

- Gameplay/menu/console policy that decides what bound commands actually do.
- Renderer/window ownership beyond the backend interface boundary.
- Every platform adapter in the repo; renderer-coupled adapters remain outside this package.
