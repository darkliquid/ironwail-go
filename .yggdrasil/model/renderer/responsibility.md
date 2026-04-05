# Responsibility

## Purpose

The `renderer` node is the umbrella for the visual output subsystem. It covers runtime renderer orchestration, backend implementations, world/entity pipelines, 2D canvas handling, and rendering-side helpers for textures, particles, decals, and dynamic effects.

## Owns

- The top-level decomposition of renderer concerns into runtime, canvas/input, GoGPU, shared world logic, and rendering helpers.
- Package-level responsibility for turning renderer-facing game state into frame output.

## Does not own

- Host frame scheduling.
- Client or server authority over gameplay state.
- Asset file-format parsing beyond the renderer-facing pieces it consumes.
