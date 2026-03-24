# Internals

## Logic

This layer implements the GoGPU equivalents of world/entity/effect rendering that in the OpenGL path are spread across dedicated GL files.
For static world visibility selection, the GoGPU world pass now consumes the backend-neutral shared helpers (`buildWorldLeafFaceLookup` output from world upload plus `selectVisibleWorldFaces` during draw) instead of carrying a backend-local leaf/PVS policy.

## Constraints

- Backend parity with the OpenGL path is a major ongoing concern.
- Late-translucent and decal handling are especially sensitive to ordering differences.

## Decisions

### GoGPU world/entity slice parallel to OpenGL

Observed decision:
- The GoGPU path mirrors many renderer concerns with dedicated files rather than trying to express all rendering through one shared implementation.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

### World-PVS parity scope outcome

Observed decision:
- Treat the original "GoGPU world PVS culling missing" parity item as closed for code-path parity: GoGPU world rendering now invokes shared world-PVS selection before sky/opaque/alpha/liquid passes.
- Keep broader renderer-parity backlog items (OIT, higher-order feature gaps, and visual parity tuning) outside this scoped item.

Rationale:
- `internal/renderer/world.go` applies `selectVisibleWorldFaces` over `worldData.Geometry.LeafFaces`, matching the shared visibility policy used by the OpenGL path.
- Remaining renderer differences are broader feature deltas rather than a narrow leaf/PVS selection mismatch.

### Sprite draw fallback consumes model-retained sprite payload

Observed decision:
- `world_sprite_gogpu` sprite draw preparation continues to use shared `spriteDataForEntity`, with regression coverage asserting that a nil `SpriteEntity.SpriteData` can still upload frame pixels from `entity.Model.SpriteData`.

Rationale:
- GoGPU sprite upload must keep parity with OpenGL and avoid backend-specific fallback differences.
- Reusing the shared fallback path ensures cache-miss sprite uploads preserve parsed payload data instead of synthetic metadata-only placeholders.
