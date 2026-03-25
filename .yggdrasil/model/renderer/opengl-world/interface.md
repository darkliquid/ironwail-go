# Interface

## Main consumers

- OpenGL backend runtime and shared world data producers

## Contracts

- this node consumes shared world structures and emits concrete OpenGL draw behavior for world geometry and related passes
- OpenGL world fog state submission applies a deterministic fixed-step transition baseline when accepting per-frame fog inputs, reducing hard pops between snapshot updates while preserving single-path world pass submission
- embedded sky rendering supports `r_fastsky`: when enabled and no external skybox path is active, sky draw calls bind a per-texture flat-color solid layer and a transparent alpha fallback to disable cloud scrolling while preserving fog and sky animation frame selection behavior
- embedded sky rendering also supports a narrow deterministic procedural baseline: when both `r_fastsky` and `r_proceduralsky` are enabled and no external skybox path is active, the OpenGL sky pass uses a dedicated procedural shader with fixed horizon/zenith colors instead of texture-backed layers
- non-fast embedded sky motion is cvar-tunable through `r_skysolidspeed` and `r_skyalphaspeed`, which scale the two scrolling sky layer offsets without changing external skybox paths
- OpenGL world texture uploads consume startup-registered texture cvars (`gl_texturemode`, `gl_lodbias`, `gl_texture_anisotropy`) for diffuse/filter selection while preserving linear lightmap filtering
