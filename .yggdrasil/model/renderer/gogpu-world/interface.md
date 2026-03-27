# Interface

## Main consumers

- GoGPU backend runtime and shared world/entity state producers

## Contracts

- this node consumes shared world/entity structures and renders them through the GoGPU backend-specific path
- receiver-free GoGPU brush-upload helpers live in `internal/renderer/world/gogpu` and are invoked by the root GoGPU world path for vertex/index packing and buffer allocation
- receiver-free GoGPU brush-entity build helpers now also live in `internal/renderer/world/gogpu`, with opaque packing in `brush_build.go` and translucent/liquid face planning in `brush_translucent.go`; the root GoGPU world path adapts renderer-owned face policy, camera-distance callbacks, and lightmap state onto those shared DTOs before HAL submission
- late translucent GoGPU brush/entity rendering now flows through the collector/render-helper stack in `internal/renderer/world_gogpu.go` (`collectGoGPUWorldTranslucentLiquidFaceRenders`, `collectGoGPUTranslucentLiquidBrushFaceRenders`, `collectGoGPUTranslucentBrushEntityFaceRenders`, `renderGoGPUAlphaTestBrushFaceRendersHAL`, `renderGoGPUSortedTranslucentFaceRendersHAL`); the older inline `renderTranslucent*BrushEntitiesHAL` implementations were removed once `renderer_gogpu.go` was confirmed to use the collector path exclusively
- the live brush-entity collectors now also share a root-local renderer snapshot helper (`loadGoGPUTranslucentBrushCollectState`) and transient buffer uploader (`createGoGPUTranslucentBrushBuffers`), keeping HAL handles and cleanup in root while trimming duplicated queue/device/liquid-alpha setup from the collectors themselves
- root translucent brush adapters now also share root-local face-conversion/assembly helpers (`convertGoGPUTranslucentFaceDraws`, `appendGoGPUTranslucentLiquidBrushFaceRenders`, `appendGoGPUTranslucentBrushEntityFaceRenders`), so the remaining collector code is mostly orchestration over shared upload and render-assembly helpers
- the late-translucent HAL passes now also share root-local material-selection helpers (`gogpuLateTranslucentTextureBindGroups`, `gogpuLateTranslucentLightmapBindGroup`), keeping texture/lightmap/fullbright policy in root while trimming repeated bind-group selection from both alpha-test and sorted translucent loops
- receiver-free GoGPU sprite draw planning, uniform packing, and sprite quad projection now live in `internal/renderer/world/gogpu/sprite.go`; the root sprite draw path now limits itself to orchestration, passing `SpriteEntity` params, a resolver closure for caller-owned `*gpuSpriteModel`, and a tiny projection callback for shared quad vertices
- receiver-free GoGPU decal helpers now live in `internal/renderer/world/gogpu/decal.go`; the root decal path keeps shared `buildDecalQuad` ownership, clamps final mark color/alpha, and keeps HAL/resource ownership, while tiny root-local adapters in `internal/renderer/world_gogpu.go` (`gogpuDecalPreparedMark`, `gogpuDecalMarkParams`, `gogpuDecalQuad`, `prepareGoGPUDecalHALDraws`) convert `decalDraw` state into `worldgogpu.DecalPreparedMark` / `DecalMarkParams` and route quad building into `PrepareDecalDrawsWithAdapter`
- receiver-free alias CPU mesh/interpolation math now lives in `internal/renderer/alias/mesh.go`; the GoGPU root stores cached alias refs as `[]aliasimpl.MeshRef` and consumes them via `MeshFromRefs` while keeping alias skin resolution, draw submission, and HAL ownership locally
- the decal seam is now considered complete before resource ownership: the remaining root helpers are explicitly policy/geometry adapters, so any further extraction should stay root-local unless it removes meaningful receiver-free logic without pulling shared quad construction, policy, or HAL lifecycle out of `internal/renderer/world_gogpu.go`
- world-face visibility for GoGPU draws is expected to come from shared-world PVS helpers (`WorldGeometry.LeafFaces` + `selectVisibleWorldFaces`) rather than backend-specific culling logic
- alias-model draw preparation applies `r_nolerp_list` through shared alias-state helpers before `SetupAliasFrame`, keeping no-lerp model handling aligned with OpenGL and C parity expectations
- normal tagged builds (`-tags gogpu`) are expected to exercise the root seam files in `internal/renderer/`, including the root decal seam coverage in `internal/renderer/world_gogpu_decal_test.go`
