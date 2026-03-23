# Responsibility

## Purpose

`renderer/assets` owns renderer-side shared asset helpers for models, sprites, decals/marks, skyboxes, and scrap atlas management.

## Owns

- renderer-facing model/sprite/decal shared helpers
- mark projection support
- external skybox support
- scrap atlas/shared atlas bookkeeping

## Does not own

- concrete OpenGL atlas upload behavior
- backend lifecycle or world submission logic
