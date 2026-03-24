# Interface

## Main consumers

- OpenGL backend runtime and shared world data producers

## Contracts

- this node consumes shared world structures and emits concrete OpenGL draw behavior for world geometry and related passes
- embedded sky rendering now supports `r_fastsky`: when enabled and no external skybox path is active, sky draw calls bind a per-texture flat-color solid layer and a transparent alpha fallback to disable cloud scrolling while preserving fog and sky animation frame selection behavior
