# Internals

## Logic

This layer implements the GoGPU equivalents of world/entity/effect rendering.
For static world visibility selection, the GoGPU world pass now consumes the backend-neutral shared helpers (`buildWorldLeafFaceLookup` output from world upload plus `selectVisibleWorldFaces` during draw) instead of carrying a backend-local leaf/PVS policy.
The live GoGPU world/entity implementation now sits in `internal/renderer/world_gogpu.go`. Earlier root seam fragments (`world_alias_gogpu_root.go`, `world_alias_shadow_gogpu_root.go`, `world_brush_gogpu_root.go`, `world_late_translucent_gogpu_root.go`, `world_sprite_gogpu_root.go`, `world_decal_gogpu_root.go`, plus support/cleanup helpers) were merged into that file after tagged validation confirmed they were the only live code path. An earlier duplicate `internal/renderer/world/gogpu/*.go` tree had already been removed. The next extraction step has now restarted by moving WGSL shader payloads into `internal/renderer/world/gogpu/shaders.go`, GoGPU brush vertex/index packing plus buffer-allocation helpers into `internal/renderer/world/gogpu/buffer.go`, opaque brush-entity CPU build helpers into `internal/renderer/world/gogpu/brush_build.go`, translucent/liquid brush-face planning into `internal/renderer/world/gogpu/brush_translucent.go`, the sprite draw-planning/uniform/vertex conversion helpers into `internal/renderer/world/gogpu/sprite.go`, and the first decal mark/draw-prep/uniform/vertex packing helpers into `internal/renderer/world/gogpu/decal.go`, which the root backend imports.
Alias-model CPU mesh shaping now also consumes `internal/renderer/alias/mesh.go`: `world_gogpu.go` keeps only renderer-owned draw orchestration, and cached alias refs are normalized to shared `[]aliasimpl.MeshRef` storage consumed through `MeshFromRefs`. The earlier renderer-local stateless pose/blend helper copy has been removed instead of being maintained beside the shared alias seam.
GoGPU world runtime submission now records command buffers through public `wgpu.CommandEncoder` / `wgpu.RenderPassEncoder` wrappers and submits them through public `wgpu.Queue`, while the CPU-only subpackage helpers in `internal/renderer/world/gogpu` remain backend-neutral planning code.
GoGPU particle runtime submission now follows the same wrapper-only path in `internal/renderer/particle_gogpu.go` (public encoder/render-pass/finish/submit plus wrapper resource lifetimes) to keep world-adjacent draw flows on one API surface.
GoGPU world-adjacent face draws now quantize evaluated dynamic-light vectors before writing scene uniforms (`1/32` increments with a small zero deadzone). This trims qbj2-scale per-face uniform churn caused by tiny floating-point differences while keeping dynamic-light response visually stable.
The live GoGPU world paths now reuse renderer-owned visibility scratch storage when asking the shared layer for PVS-visible world faces. That keeps qbj2-scale world and late-translucent liquid passes from allocating a fresh visibility mask/result slice every frame while still consuming the shared ordering/visibility policy.
The GoGPU world and late-translucent liquid passes now also feed quantized dynamic-light RGB values into their world uniforms instead of raw per-face floating-point results. On dense BSP2 maps with only a few active lights, that reduces spurious uniform churn from near-identical values without changing pass ordering or the underlying shared light-evaluation rules.
The particle shaders now avoid writable swizzle updates on the GoGPU Vulkan/SPIR-V path. The vertex shader reconstructs `clipPosition` instead of mutating `clipPosition.xy`, and the fragment shader reconstructs `color` from `foggedRGB` instead of assigning through `color.rgb`. This keeps behavior equivalent while avoiding the Naga lowering failure (`ExprSwizzle is not a pointer expression`) seen during shader-module creation.
GoGPU alias-model and alias-shadow submission must not batch per-draw `queue.WriteBuffer` updates for one shared uniform/scratch buffer inside a single recorded render pass. On this stack, later queue writes can overwrite the data seen by earlier recorded draws, so alias submissions now record and submit one pass per draw after uploading that draw's uniform/vertex payload.
Opaque GoGPU brush-entity submission now follows the scratch-buffer pattern already used by alias/sprite/decal passes instead of creating fresh vertex/index buffers every frame. The renderer keeps reusable brush-entity vertex/index scratch buffers, uploads all prepared brush draws into those buffers, and records one pass over the packed offsets. The pass also reuses the same material-bind-state tracking as the main world pass so consecutive faces that share diffuse/lightmap/fullbright bind groups do not redundantly rebind them. This preserves existing face ordering and bind-group decisions while removing per-frame buffer create/release churn and unnecessary face-local bind churn from qbj2-style entity-heavy scenes.
Opaque-liquid GoGPU brush entities now use that same renderer-owned scratch-buffer path instead of allocating and releasing fresh vertex/index buffers for every liquid submodel draw. The liquid pass uploads prepared draws into the reusable scratch buffers, records one pass over packed offsets, and reuses material-bind tracking so qbj2-style turbulent brush entities stop paying avoidable per-frame buffer allocation churn on the Vulkan path.
When `host_speeds` is enabled, the GoGPU world pass also emits `render_world_speeds` so qbj2-sized maps can distinguish visible-face selection, world-face classification, batch building, batched-index upload, sky drawing, opaque/alpha/liquid drawing, and final queue submit time. That keeps later world-side performance work focused on the real hot subphase instead of treating the whole `world_ms` bucket as opaque.
World-face material sorting is order-insensitive as long as equal material/light/fog keys remain grouped for batching, so the batched world path now uses an unstable sort instead of a stable sort for opaque/alpha/liquid world draws. That preserves the batching contract while reducing per-frame batch-build overhead on large visible-face sets such as qbj2.
The main qbj2 world bottleneck turned out not to be submission or visibility selection but rebuilding the same batched world index list every frame while the camera remained inside the same BSP leaf. The GoGPU world pass now caches the most recent sky face list plus opaque/alpha/liquid batched index layout by `(camera leaf, dynamic-light signature)` and reuses it while that key stays stable. This follows the same broad shape as the C renderer's mark-and-draw flow: the visible-surface worklist is reused across draws until the visibility basis changes, instead of rebuilding material-grouped world work every frame. The cache key is intentionally based on visually effective light state rather than raw light age/lifetime bookkeeping, so tiny dynamic-light fade updates do not flush the cache unless they actually change the quantized world-light buckets used for batching.

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

### Public wgpu command-recording contract for GoGPU world-adjacent passes

Observed decision:

- Renderer-owned GoGPU world-adjacent passes (world/entity/sprite/decal/particle/late-translucent) now use public `*wgpu.Device` / `*wgpu.Queue` wrappers for encoder creation, render-pass recording, command-buffer finish, and submission.
- Renderer code in this node no longer mixes wrapper submission with renderer-local HAL queue/device fetches for these flows.

Rationale:

- Keeping creation, encoding, and submission on a single public API surface reduces backend-boundary ambiguity and avoids mismatched wrapper-vs-HAL lifetime semantics in live draw paths.

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

### Translucent brush planning keeps renderer policy in root

Observed decision:

- GoGPU translucent brush-entity extraction now stops at receiver-free vertex transformation, index rebasing, alpha-test/translucent/liquid face bucketing, transformed center computation, and distance capture in `internal/renderer/world/gogpu/brush_translucent.go`.
- `internal/renderer/world_gogpu.go` keeps the face-policy callbacks (`shouldDrawGoGPUTranslucentLiquidBrushFace`, `shouldDrawGoGPUTranslucentBrushEntityFace`, `worldFaceAlpha`, `worldFacePass`, `worldFaceIsLiquid`) plus camera-distance adapters, lightmap attachment, and all buffer/pipeline submission.
- The legacy inline brush-entity translucent renderers were removed after the late translucent phase in `renderer_gogpu.go` was confirmed to use the collector path exclusively; the only live route is now collector/build (`collectGoGPUWorldTranslucentLiquidFaceRenders`, `collectGoGPUTranslucentLiquidBrushFaceRenders`, `collectGoGPUTranslucentBrushEntityFaceRenders`) followed by the shared root HAL helpers (`renderGoGPUAlphaTestBrushFaceRendersHAL`, `renderGoGPUSortedTranslucentFaceRendersHAL`).
- `collectGoGPUWorldTranslucentLiquidFaceRenders` now runs `selectVisibleWorldFaces` before building late-translucent world-liquid draws, so the late translucent phase follows the same shared PVS/leaf visibility gate as the main world sky/opaque/alpha/liquid passes.
- The remaining live collector duplication is now reduced to face-render assembly: the brush collectors share a root-local `gogpuTranslucentBrushCollectState` snapshot plus `createGoGPUTranslucentBrushBuffers`, which centralize HAL device/queue access, liquid-alpha snapshotting, and transient vertex/index upload while still leaving face bucketing and lightmap attachment local to each collector.
- The root adapter layer has also been collapsed around the remaining DTO conversions: `convertGoGPUTranslucentFaceDraws` now handles package-to-root translucent face conversion, and `appendGoGPUTranslucentLiquidBrushFaceRenders` / `appendGoGPUTranslucentBrushEntityFaceRenders` centralize the last mechanical render-wrapper assembly without moving lightmap ownership or HAL submission out of `world_gogpu.go`.
- Late-translucent render submission now also shares root-local material selection: `gogpuLateTranslucentTextureBindGroups` and `gogpuLateTranslucentLightmapBindGroup` centralize texture/fullbright/lightmap lookup for both alpha-test and sorted translucent draws, leaving the render loops focused on uniform updates, pipeline choice, and draw submission.
- GoGPU liquid/lightmap selection now carries an explicit model/world-level `hasLitWater` signal through opaque-liquid, translucent-liquid, and late-translucent liquid draw assembly, so turbulent faces follow the same lit-water gate as OpenGL (`HasLitWater` on the owning model/world) instead of toggling lit-water only when the individual face has a direct lightmap slot.
- Translucent brush collectors no longer create one temporary vertex/index buffer pair per brush draw. Each collector phase now packs all prepared translucent brush vertices and indices into one uploaded buffer pair and carries per-draw vertex/index offsets through `gogpuTranslucentBrushFaceRender`, so late-translucent alpha-test and sorted passes keep their existing ordering while cutting Vulkan buffer create/release churn during qbj2-scale brush-heavy scenes.
- The late-translucent alpha-test and sorted passes now also reuse the renderer-owned world uniform bind group and cache texture/lightmap/fullbright bind state inside the pass, mirroring the opaque brush passes. This preserves existing face order and per-draw uniform uploads, but it avoids recreating a pass-local uniform bind group every frame and stops rebinding identical material groups on consecutive translucent draws.
- Late-translucent **world** liquid collection now prefers the main world-pass cached translucent-liquid face subset instead of rerunning `selectVisibleWorldFaces` and then refiltering for translucent liquid. The collector still rebuilds per-frame `distanceSq` from the live camera before sorted translucency, so ordering stays correct while qbj2 stops paying duplicate world-visibility/filter work in the entity/translucency phase. It also now benefits from the renderer's small fixed set of recent `(leaf, light-signature)` cache entries instead of only the immediately previous world-pass entry, improving reuse during movement across nearby leaves without changing translucent ordering.
- GoGPU world, brush-liquid, late-translucent, and first-frame-stat paths now resolve liquid alpha from cached shared-world BSP facts on `WorldGeometry` instead of reparsing `worldspawn` and transparent-water VIS state in each hot path. Runtime alpha cvars still apply live, but the immutable map-side parse/visibility work now happens once per geometry build instead of once per draw phase.
- GoGPU world-adjacent hot loops now also reuse stack-backed `worldUniformBufferSize` scratch when packing world scene uniforms for `queue.WriteBuffer`. That keeps per-draw brush/late-translucent uniform uploads behaviorally identical while trimming repeated heap allocation churn from the renderer-side CPU path.
- Opaque brush-entity preparation now classifies opaque vs alpha-test faces from a single transformed-vertex build per entity instead of rebuilding the same transformed geometry twice. The render path still preserves the old opaque-then-alpha-test submission order and per-face lighting/material policy, but qbj2 stops paying duplicate model-space transform and duplicate scratch-vertex upload cost for brush entities that mix solid and alpha-tested faces.
- Late-translucent resource loading now keeps the renderer read lock for the full pass-recording window and releases it only after command submission setup is complete, preventing `ClearWorld`/world-reload teardown from releasing world uniform layout/buffer state while late-translucent passes are still creating/binding their transient uniform bind group.
- The sorted late-translucent GoGPU pass must bind a render pipeline before its first `SetBindGroup(0, ...)`. On this Vulkan wrapper stack, descriptor-set binding resolves through the currently active raw pipeline layout; skipping the initial `SetPipeline` left the raw pass without a layout and crashed qbj2/new-game startup on the first fogged translucent world frame.
- Package-local coverage in `internal/renderer/world/gogpu/brush_translucent_test.go` locks the seam to rebased indices, transformed centers, and alpha-test vs translucent/liquid partitioning without instantiating renderer HAL state.

Rationale:

- The face-policy decisions still depend on renderer-owned liquid-alpha settings and shared world-pass helpers, so keeping that policy in root avoids leaking renderer state into the subpackage.
- The extracted helpers are CPU-only planning with stable DTO boundaries, which trims `world_gogpu.go` without pulling HAL/resource lifetime code across the seam.
- Holding the renderer read lock across late-translucent pass setup removes a startup/reload lifetime race where a non-nil but already released world uniform resource could be observed after snapshotting, causing Vulkan `SetBindGroup` failures during mod new-game transitions.
- Seeding the sorted pass with an initial pipeline bind preserves the wrapper/HAL contract for descriptor-set binding and fixes the observed qbj2 mod-start crash without changing face ordering or material selection.

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

### Alias GoGPU draws submit one uploaded payload at a time

Observed decision:

- GoGPU alias-model and alias-shadow rendering now upload the shared scratch/uniform payload for one draw, record one render pass using that payload, submit it, then repeat for the next draw.
- The backend intentionally avoids encoding multiple alias draws after a sequence of `queue.WriteBuffer` calls into the same shared buffers.
- `internal/renderer/world_gogpu.go` now keeps an inline note beside `renderAliasDrawsHAL` so future cleanup does not accidentally re-batch those per-draw uploads into one pass.

Rationale:

- The previous batched encoding path reused one shared scratch vertex buffer and one shared uniform buffer across multiple draws in a single recorded pass while updating them with `queue.WriteBuffer` inside the draw loop.
- On the GoGPU/WebGPU path this can collapse multiple alias draws onto the last uploaded payload, which matches the observed flickering/missing-model symptoms for animated alias entities such as zombies and grenades.
