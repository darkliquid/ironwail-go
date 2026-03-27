# Internals

## Logic

This layer implements the GoGPU equivalents of world/entity/effect rendering.
For static world visibility selection, the GoGPU world pass now consumes the backend-neutral shared helpers (`buildWorldLeafFaceLookup` output from world upload plus `selectVisibleWorldFaces` during draw) instead of carrying a backend-local leaf/PVS policy.
The live GoGPU world/entity implementation now sits in `internal/renderer/world_gogpu.go`. Earlier root seam fragments (`world_alias_gogpu_root.go`, `world_alias_shadow_gogpu_root.go`, `world_brush_gogpu_root.go`, `world_late_translucent_gogpu_root.go`, `world_sprite_gogpu_root.go`, `world_decal_gogpu_root.go`, plus support/cleanup helpers) were merged into that file after tagged validation confirmed they were the only live code path. An earlier duplicate `internal/renderer/world/gogpu/*.go` tree had already been removed. The next extraction step has now restarted by moving WGSL shader payloads into `internal/renderer/world/gogpu/shaders.go`, GoGPU brush vertex/index packing plus buffer-allocation helpers into `internal/renderer/world/gogpu/buffer.go`, the generic brush-entity CPU build path into `internal/renderer/world/gogpu/brush_build.go`, the sprite draw-planning/uniform/vertex conversion helpers into `internal/renderer/world/gogpu/sprite.go`, and the first decal mark/draw-prep/uniform/vertex packing helpers into `internal/renderer/world/gogpu/decal.go`, which the root backend imports.
Alias-model CPU mesh shaping now also consumes `internal/renderer/alias/mesh.go`: `world_gogpu.go` keeps only renderer-owned draw orchestration, and cached alias refs are normalized to shared `[]aliasimpl.MeshRef` storage consumed through `MeshFromRefs`. The earlier renderer-local stateless pose/blend helper copy has been removed instead of being maintained beside the shared alias seam.

## Constraints

- Backend parity with the OpenGL path is a major ongoing concern.
- Late-translucent and decal handling are especially sensitive to ordering differences.

## Decisions

### GoGPU world/entity slice parallel to OpenGL

Observed decision:
- The GoGPU path mirrors many renderer concerns with dedicated files rather than trying to express all rendering through one shared implementation.
- During the refactor, the live implementation was first consolidated into root seam files instead of keeping half-migrated method receivers in a dead `internal/renderer/world/gogpu/` subtree, then those seams were merged into one backend file to reduce root-file count.

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
- Sprite draw planning, render uniform packing, and the world-vertex conversion now live beside other GoGPU receiver-free helpers in `internal/renderer/world/gogpu/sprite.go`, while `internal/renderer/world_gogpu.go` keeps sprite model resolution/cache ownership, draw orchestration, and quad expansion.
- The draw-planning seam now follows a resolver-based shape: root adapts `SpriteEntity` into a DTO containing model id, parsed sprite payload, frame/origin/angles/alpha/scale, then supplies a closure that resolves a caller-owned `*gpuSpriteModel` plus frame count.
- Package-local tests now cover resolver hits, misses, nil resolver rejection, frame clamping, and alpha visibility without instantiating root renderer state.
- The final sprite quad DTO bridge (`spriteQuadVerticesToGoGPU`) is gone from the root file; `world_gogpu.go` now passes expanded shared sprite vertices through a package-local projection helper in `internal/renderer/world/gogpu/sprite.go`.

Rationale:
- GoGPU sprite upload must keep parity with OpenGL and avoid backend-specific fallback differences.
- Reusing the shared fallback path ensures cache-miss sprite uploads preserve parsed payload data instead of synthetic metadata-only placeholders.
- Keeping GPU upload/cache ownership in `Renderer` avoids leaking root-only HAL/bind-group state into the subpackage, while a resolver-based helper still moves frame clamping/alpha visibility and draw assembly out of the root file.
- Moving the uniform byte layout and world-vertex conversion into `package gogpu` trims root-file byte-packing noise while keeping the remaining root adapter limited to concrete sprite-model resolution plus quad expansion.

### Decal quad expansion keeps shared geometry ownership in root

Observed decision:
- GoGPU decal extraction currently stops at mark DTO shaping, packed draw preparation, uniform packing, vertex expansion, and vertex byte packing in `internal/renderer/world/gogpu/decal.go`, while `internal/renderer/world_gogpu.go` now routes `decalDraw` values through tiny root-local adapters (`gogpuDecalPreparedMark`, `gogpuDecalMarkParams`, `gogpuDecalQuad`, `prepareGoGPUDecalHALDraws`) before calling `PrepareDecalDrawsWithAdapter`.
- Those root helpers keep the last root-owned seam explicit: shared `buildDecalQuad`, per-mark final color/alpha clamping, and HAL resource setup/bind groups remain in `internal/renderer/world_gogpu.go`.
- Gogpu-tagged root coverage in `internal/renderer/world_gogpu_decal_test.go` locks the adapter seam to geometry preservation, final color/alpha clamping, root quad building, and packed draw output.

Rationale:
- `buildDecalQuad` is shared with the OpenGL path, so leaving quad construction in root/shared code avoids backend drift in decal placement math.
- Stopping here keeps the remaining logic small and obviously root-owned: shared decal placement math, final color/alpha policy, and HAL submission are still coupled to renderer state and backend resource lifetime.
- Extracting the adapter into root-local helpers trims `renderDecalMarksHAL` without moving policy into the subpackage; beyond this point, extra helpers would mostly wrap root-owned policy instead of removing meaningful receiver-free logic.

### Alias interpolation honors shared no-lerp model list

Observed decision:
- GoGPU alias draw preparation now mutates alias header flags via `applyAliasNoLerpListFlags` before calling `SetupAliasFrame`.
- GoGPU alias shadow exclusion parsing (`parseAliasShadowExclusionsGO`) delegates to the shared alias model-list parser.
- GoGPU alias CPU mesh/interpolation math now delegates to shared `renderer/alias` helpers via callback-based DTO adaptation instead of duplicating Euler rotation and vertex interpolation logic in `world_gogpu.go`.
- The remaining GoGPU root seam intentionally stops at draw preparation, scratch-buffer ownership, and HAL submission; backend-local alias refs are stored in the shared ref shape directly instead of keeping a backend-specific ref wrapper.

Rationale:
- C/Ironwail uses `r_nolerp_list` as a model-level interpolation override; applying the same list in GoGPU prevents backend-specific animation blending drift.
- Sharing parser behavior with OpenGL avoids diverging tokenization/case-handling behavior for model-list cvars.
- Moving the pure mesh math behind DTO callbacks removes duplicated alias-vertex shaping logic while leaving GoGPU skin lookup, draw orchestration, and HAL submission in the root backend where renderer-owned state already lives.
