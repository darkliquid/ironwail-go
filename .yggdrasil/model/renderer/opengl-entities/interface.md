# Interface

## Main consumers

- OpenGL backend runtime and shared model/sprite/decal/world state producers

## Contracts

- entity-specific rendering must integrate with the OpenGL frame pipeline and transparency ordering chosen by the backend/runtime
- alias-model draw preparation applies shared `r_nolerp_list` handling before interpolation setup so no-lerp model overrides match C parity behavior
- `internal/renderer/alias_opengl.go` now acts as an OpenGL adapter over `renderer/alias` mesh helpers for interpolated alias vertex shaping instead of owning the CPU interpolation/rotation implementation locally
