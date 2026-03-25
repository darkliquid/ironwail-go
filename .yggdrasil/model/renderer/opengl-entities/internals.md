# Internals

## Logic

This layer handles entity and effect categories that require OpenGL-specific draw code, especially alias models, sprites, decals, and transparency strategies.

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

Rationale:
- Shared sprite fallback behavior should preserve parsed sprite payloads regardless of whether explicit entity sprite data is present.
- This keeps OpenGL behavior aligned with GoGPU and prevents placeholder-only cache uploads when explicit sprite payload wiring is absent.

### Alias interpolation honors shared no-lerp model list

Observed decision:
- OpenGL alias draw preparation now applies `applyAliasNoLerpListFlags` to alias header flags before invoking `SetupAliasFrame`.
- Alias shadow exclusion parsing (`parseAliasShadowExclusions`) reuses shared alias model-list parsing.

Rationale:
- C/Ironwail model interpolation parity depends on `r_nolerp_list` being respected during frame blending decisions.
- Reusing shared list parsing keeps OpenGL and GoGPU behavior aligned for case/whitespace token handling in model-list cvars.
