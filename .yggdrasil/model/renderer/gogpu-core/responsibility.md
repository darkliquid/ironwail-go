# Responsibility

## Purpose

`renderer/gogpu-core` owns the GoGPU backend lifecycle, frame runtime integration, and GoGPU-specific postprocess/runtime helpers.

## Owns

- GoGPU renderer construction and runtime control
- core GoGPU frame behavior
- GoGPU-specific polyblend and waterwarp support

## Does not own

- GoGPU world/entity submission details handled in sibling nodes
