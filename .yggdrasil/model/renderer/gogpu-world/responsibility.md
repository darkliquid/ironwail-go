# Responsibility

## Purpose

`renderer/gogpu-world` owns GoGPU rendering paths for world brushes, alias models, shadow helpers, decals, late translucency, sprites, and particles.

## Owns

- GoGPU world and entity draw implementations
- GoGPU particle path
- GoGPU decal and late-translucent handling

## Does not own

- backend-neutral world preparation
- GoGPU backend lifecycle itself
