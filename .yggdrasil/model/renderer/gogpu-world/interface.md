# Interface

## Main consumers

- GoGPU backend runtime and shared world/entity state producers

## Contracts

- this node consumes shared world/entity structures and renders them through the GoGPU backend-specific path
- world-face visibility for GoGPU draws is expected to come from shared-world PVS helpers (`WorldGeometry.LeafFaces` + `selectVisibleWorldFaces`) rather than backend-specific culling logic
- alias-model draw preparation applies `r_nolerp_list` through shared alias-state helpers before `SetupAliasFrame`, keeping no-lerp model handling aligned with OpenGL and C parity expectations
