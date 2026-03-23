# Responsibility

## Purpose

`model` owns the shared vocabulary for Quake models at runtime: common model types, brush-model data structures, alias-model attachment points, sprite containers, bounds, hulls, and narrow collision-facing accessors.

## Owns

- The package-level abstraction that lets server collision, renderer code, and model loaders talk about the same `Model`, `Texture`, `Hull`, alias, and sprite concepts.
- Shared enums/constants such as `ModelType`, `SyncType`, texture/surface flags, and alias limits.
- Runtime structs for brush geometry, collision hulls, alias headers, sprite frames/groups, and aggregate `Model` instances.
- Collision adapter methods that expose just enough of `Model` for world/collision consumers.
- Cross-format tests that lock in alias and sprite loader expectations.

## Does not own

- BSP parsing and world-model population.
- Renderer-specific mesh upload or sprite submission.
- Filesystem access for loading model bytes.
