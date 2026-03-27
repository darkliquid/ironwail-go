# Interface

## Main consumers

- `renderer/opengl-world`

## Contracts

- exposes bucket/draw DTOs and helpers used by root pass orchestration (`DrawCall`, `RenderDrawCalls`, `BindWorldProgram`)
- exposes probe/debug DTOs and helpers consumed directly from the pass helpers or root tests (`FaceProbeStats`, `FaceProbeEntry`, `RenderPassName`, `ProbeFacesInBBox`)
- exposes sprite submission helpers that keep final draw mechanics out of the renderer root while leaving cache ownership local
