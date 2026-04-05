# Internals

## Logic

This layer handles entity and effect categories that require OpenGL-specific draw code, especially alias models, sprites, decals, and transparency strategies.
`internal/renderer/alias_opengl.go` now acts as a narrow OpenGL adapter over the shared `renderer/alias` package for interpolated alias world-vertex shaping, with `glAliasModel` caching shared `[]aliasimpl.MeshRef` values directly so the shim can pass ref slices through `MeshFromRefs` without any backend-local adapter closure. The remaining OpenGL-specific entity concerns stay focused on draw orchestration and tests rather than duplicated CPU mesh math.

## Constraints

- Transparency/OIT behavior is strongly backend- and parity-sensitive.
- Atlas/scrap behavior must stay aligned with the OpenGL texture path.

## Decisions

### Dedicated OpenGL entity/effect slice

Observed decision:
- OpenGL world geometry and OpenGL entity/effect rendering are split into separate nodes.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

### OpenGL sprite draw fallback honors model-retained sprite data

Observed decision:
- OpenGL sprite draw tests now assert that `buildSpriteDrawLocked` can upload from `entity.Model.SpriteData` when `entity.SpriteData` is nil.
- OpenGL sprite/shared sprite tests also lock the shared `SPR_ORIENTED` poster offset so wall-mounted sprites keep a small depth separation from coplanar world geometry.

Rationale:
- Shared sprite fallback behavior should preserve parsed sprite payloads regardless of whether explicit entity sprite data is present.
- This keeps OpenGL behavior aligned with GoGPU and prevents placeholder-only cache uploads when explicit sprite payload wiring is absent.
- Keeping the poster offset under shared/OpenGL test coverage helps preserve Ironwail parity for the path that had been shimmering during camera motion.

### Alias interpolation honors shared no-lerp model list

Observed decision:
- OpenGL alias draw preparation now applies `applyAliasNoLerpListFlags` to alias header flags before invoking `SetupAliasFrame`.
- Alias shadow exclusion parsing (`parseAliasShadowExclusions`) reuses shared alias model-list parsing.
- The OpenGL-only alias shim now delegates interpolated vertex shaping to `renderer/alias` mesh helpers via `MeshFromRefs` over shared cached `aliasimpl.MeshRef` values instead of carrying a second copy of interpolation/rotation math or repeated call-site mesh adapter closures.

Rationale:
- C/Ironwail model interpolation parity depends on `r_nolerp_list` being respected during frame blending decisions.
- Reusing shared list parsing keeps OpenGL and GoGPU behavior aligned for case/whitespace token handling in model-list cvars.
- Sharing the CPU mesh helper implementation with GoGPU reduces backend drift risk while preserving an OpenGL-local adapter surface for tests and call sites.
