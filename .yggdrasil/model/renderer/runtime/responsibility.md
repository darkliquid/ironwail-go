# Responsibility

## Purpose

`renderer/runtime` owns the backend-agnostic renderer package surface, runtime adapter helpers, render-pass classification, and the stub/headless fallback path.

## Owns

- High-level renderer package intent and shared runtime surface.
- Adapter behavior that bridges renderer usage into the rest of the engine.
- Render-pass parity helpers used to classify or compare render phases.
- Stub/headless renderer behavior when no concrete backend is active.

## Does not own

- Canvas transform logic.
- Concrete OpenGL or GoGPU implementation details.
- World/entity render pipelines.
