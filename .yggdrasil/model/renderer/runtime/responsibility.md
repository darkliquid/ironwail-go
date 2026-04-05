# Responsibility

## Purpose

`renderer/runtime` owns the backend-agnostic renderer package surface, runtime adapter helpers, and render-pass classification around the canonical GoGPU runtime.

## Owns

- High-level renderer package intent and shared runtime surface.
- Adapter behavior that bridges renderer usage into the rest of the engine.
- Render-pass parity helpers used to classify or compare render phases.

## Does not own

- Canvas transform logic.
- Concrete backend implementation details.
- World/entity render pipelines.
