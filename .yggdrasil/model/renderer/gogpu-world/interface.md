# Interface

## Main consumers

- GoGPU backend runtime and shared world/entity state producers

## Contracts

- this node consumes shared world/entity structures and renders them through the GoGPU backend-specific path
- receiver-free GoGPU brush-upload helpers live in `internal/renderer/world/gogpu` and are invoked by the root GoGPU world path for vertex/index packing and buffer allocation
- receiver-free GoGPU brush-entity build helpers now also live in `internal/renderer/world/gogpu`, with the root GoGPU world path adapting renderer-owned entities/lightmap state onto those shared DTOs before HAL submission
- receiver-free GoGPU sprite draw planning, uniform packing, and sprite quad projection now live in `internal/renderer/world/gogpu/sprite.go`; the root sprite draw path now limits itself to orchestration, passing `SpriteEntity` params, a resolver closure for caller-owned `*gpuSpriteModel`, and a tiny projection callback for shared quad vertices
- receiver-free GoGPU decal helpers now live in `internal/renderer/world/gogpu/decal.go`; the root decal path keeps shared `buildDecalQuad` ownership, clamps final mark color/alpha, and keeps HAL/resource ownership, while delegating mark DTO shaping, batched packed draw preparation, uniform packing, and vertex byte shaping to the subpackage
- world-face visibility for GoGPU draws is expected to come from shared-world PVS helpers (`WorldGeometry.LeafFaces` + `selectVisibleWorldFaces`) rather than backend-specific culling logic
- alias-model draw preparation applies `r_nolerp_list` through shared alias-state helpers before `SetupAliasFrame`, keeping no-lerp model handling aligned with OpenGL and C parity expectations
- normal tagged builds (`-tags gogpu`) are expected to exercise the root seam files in `internal/renderer/`
