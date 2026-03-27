# Responsibility

## Purpose

`renderer/alias` owns backend-neutral alias-model helpers that can be shared across renderer backends without pulling renderer-owned cache state, GL state, or HAL resource lifecycle into the helper package.

## Owns

- alias animation/state helpers in `internal/renderer/alias/model.go`
- shared alias CPU mesh/interpolation math and Euler rotation helpers in `internal/renderer/alias/mesh.go`
- package-local regression coverage for backend-neutral alias helper behavior

## Does not own

- backend submission, GL draw calls, or GoGPU HAL/resource lifetime
- renderer-owned alias model caches, skin resolution, or entity orchestration
- world/entity policy that depends on `*Renderer` state

