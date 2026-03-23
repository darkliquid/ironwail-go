# Responsibility

## Purpose

`ironwailgo/debug-view-telemetry` owns the debug-only telemetry surface used to diagnose view/origin selection, relink behavior, and entity collection decisions at runtime.

## Owns

- `cl_debug_view` registration and level interpretation.
- Per-frame telemetry state, coalescing, and emission.
- Structured logging helpers for origin selection, relink phases, entity collection, and viewmodel-related diagnostics.
- Tests covering the telemetry behavior.

## Does not own

- The underlying view/camera/entity algorithms being observed.
- General-purpose logging infrastructure outside this targeted telemetry surface.
