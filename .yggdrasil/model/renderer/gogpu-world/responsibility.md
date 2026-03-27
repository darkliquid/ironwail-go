# Responsibility

## Purpose

`renderer/gogpu-world` owns GoGPU rendering paths for world brushes, alias models, shadow helpers, decals, late translucency, sprites, and particles.

## Owns

- GoGPU world and entity draw implementations
- GoGPU world brush buffer packing/allocation helpers that are backend-specific but receiver-free
- GoGPU brush-entity CPU build helpers that transform shared geometry into backend draw payloads before HAL submission
- GoGPU sprite draw-planning helpers, uniform packing, sprite vertex DTO adaptation, and package-local resolver/regression coverage
- GoGPU decal mark/draw-prep batching, uniform, and vertex packing helpers plus their package-local regression coverage
- GoGPU particle path
- GoGPU decal and late-translucent handling
- the root-owned GoGPU seam files under `internal/renderer/world_*_gogpu_root.go`, which now carry the live backend implementation used by `renderer_gogpu.go`

## Does not own

- backend-neutral world preparation
- GoGPU backend lifecycle itself
- deleted legacy subdirectory copies that were briefly used as quarantine scaffolding during the refactor
