# Internals

## Logic

This node exists to hold pure alias-model logic that would otherwise be duplicated across backend roots. It centralizes frame/state interpolation, CPU mesh shaping, generic mesh-adapter construction from backend ref slices, and Euler rotation math while staying independent from renderer-owned caches and backend APIs. Backend-local ref types can now satisfy the shared `MeshRefConvertible` contract so renderer roots pass ref slices directly without repeating inline `MeshFromAccessor` closures. `BuildVerticesInterpolatedInto` provides a caller-owned destination path so per-frame alias interpolation can reuse buffers instead of allocating a new vertex slice each call.

## Constraints

- behavior must stay parity-safe across OpenGL and GoGPU
- helpers should work from DTOs/adapters rather than importing backend-local model/cache types
- no GL state, HAL handles, or `*Renderer` methods belong here

## Decisions

### Shared alias helper seam

Observed decision:
- keep alias helper extraction focused on pure CPU math and DTO shaping
- leave backend submission, resource lifetime, and root-owned cache lookups in renderer roots

Rationale:
- this seam removes duplicated backend math without forcing backend-local runtime concerns into the shared package
