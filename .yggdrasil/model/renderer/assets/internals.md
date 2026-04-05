# Internals

## Logic

This layer gathers renderer-side asset helpers that are not purely backend-specific but also do not belong in the shared world-preparation slice.

## Constraints

- Asset helpers must stay consistent across backend consumers.
- Scrap atlas behavior is coupled to how backend-specific texture upload paths consume atlas data.

## Decisions

### Shared renderer asset helpers outside one backend

Observed decision:
- Asset-side helpers such as skybox, marks, sprite/model shared code, and scrap bookkeeping are factored into shared nodes rather than buried entirely inside one backend path.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

### Sprite fallback prefers parsed model payload

Observed decision:
- `spriteDataFromModel` now returns `model.Model.SpriteData` whenever it is present before synthesizing bounds-only placeholder sprite metadata.
- Shared sprite helpers also centralize poster policy predicates such as `spriteNeedsDepthOffset` and `spriteUsesOpaqueCutout` so backend-specific sprite passes can make the same `SPR_ORIENTED` cutout/depth-bias decision without duplicating type checks.

Rationale:
- Runtime entity collection can retain parsed sprite frame payload on `Model.SpriteData` even when `SpriteEntity.SpriteData` is absent on a downstream path.
- Keeping the poster-policy branch in shared code reduces backend drift while the concrete backends still own the actual render-state differences.
- Returning `Model.SpriteData` preserves real frame pixels for backend uploads and prevents cache-miss uploads from degenerating to empty placeholder frames.
