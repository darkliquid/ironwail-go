# Internals

## Logic

The parent node is the shared model vocabulary rather than a single loader. `types.go` defines the in-memory forms used across the engine for brush surfaces, BSP nodes/leaves, clip hulls, textures, alias headers, and sprite payloads. `collision.go` exposes a narrow adapter around `Model` so collision code can avoid knowing the whole struct layout. Tests span both alias and sprite parsing and therefore live at the parent boundary, where they can assert shared contracts such as bounds validity, grouped-skin preservation, sprite frame typing, and sync behavior.

## Constraints

- `Model` is asymmetric: brush data lives directly on the struct, alias data hangs off `AliasHeader`, and sprites are loaded as `*MSprite` rather than being embedded into `Model`.
- `MSpriteFrameDesc.FramePtr` is a union of `*MSpriteFrame` and `*MSpriteGroup`, so callers must branch on `Type` and then type-assert.
- `Model` also carries `bsp.DModel` submodels, making it the seam between BSP parsing and runtime consumers.
- Several limits (`MaxAliasVerts`, `MaxAliasFrames`, `MaxSkins`, `MaxMapHulls`, `MaxLightmaps`) are part of the package contract and constrain downstream loader behavior.

## Decisions

### Centralize model vocabulary in one package instead of scattering structs by subsystem

Observed decision:
- The Go port keeps common model types in a dedicated package shared by loaders, renderer code, and collision code.

Rationale:
- **unknown — inferred from code and package docs, not confirmed by a developer**

Observed effect:
- Cross-subsystem code can share one set of runtime structures and flags, but the parent package has to carry concepts spanning brush, alias, and sprite models rather than serving one narrow responsibility.
