# Responsibility

## Purpose

`renderer/gogpu-core` owns the GoGPU backend lifecycle, frame runtime integration, and GoGPU-specific postprocess/runtime helpers.

## Owns

- GoGPU renderer construction and runtime control
- core GoGPU frame behavior
- GoGPU-specific polyblend and waterwarp support
- active GoGPU frame utilities that are still exercised by the backend runtime

## Does not own

- GoGPU world/entity submission details handled in sibling nodes
- dead debug-only overlays or helper paths once they no longer participate in the live frame pipeline
