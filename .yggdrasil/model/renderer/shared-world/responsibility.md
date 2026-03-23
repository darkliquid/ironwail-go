# Responsibility

## Purpose

`renderer/shared-world` owns backend-neutral world-data preparation and shared helpers for surfaces, textures, fog, sky, transforms, and liquid/lightmap handling.

## Owns

- world geometry preparation shared across backends
- shared world-pass inputs and transforms
- shared fog/sky/liquid-alpha helpers
- shared surface/texture/lightmap sample helpers

## Does not own

- concrete backend submission logic
- non-world renderer helpers unrelated to shared world data
