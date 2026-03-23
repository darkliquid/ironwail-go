# Responsibility

## Purpose

`renderer/canvas` owns the backend-neutral screen/canvas model and shared camera/screen orchestration helpers used by both 2D and 3D rendering paths.

## Owns

- renderer-facing config and interface types
- canvas types, transforms, and bounds calculations
- screen-space coordination and shared camera helpers

## Does not own

- backend event loops
- backend-specific draw calls
- world/entity submission details
