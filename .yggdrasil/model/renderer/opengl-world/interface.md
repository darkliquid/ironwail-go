# Interface

## Main consumers

- OpenGL backend runtime and shared world data producers

## Contracts

- this node consumes shared world structures and emits concrete OpenGL draw behavior for world geometry and related passes
- **Organization**: OpenGL backend implementations are organized in `internal/renderer/world_opengl.go` with true backend subpackage helpers under `internal/renderer/world/opengl/` for shader payloads and GL texture-upload primitives.
- those root seam files own the OpenGL-specific renderer lifecycle, sprite, sky, and upload/runtime entry points that must stay methods on `*Renderer`, including world lifecycle (`HasWorldData`, `GetWorldBounds`, `ClearWorld`, external skybox lifecycle), sprite submission/caching (`renderSpriteEntities`, sprite draw preparation), sky program/fallback initialization, and world upload/lightmap refresh helpers. Shared OpenGL support types/constants also have a root home (`glWorldMesh`, `glAliasModel`, `worldDrawCall`, core world shader strings).
- OpenGL world fog state submission applies a deterministic fixed-step transition baseline when accepting per-frame fog inputs, reducing hard pops between snapshot updates while preserving single-path world pass submission
- OpenGL liquid/worldspawn override handling consumes shared `internal/renderer/world/*` entity-lump parsing and alpha/bool cvar helpers instead of owning duplicate parsing primitives locally
- OpenGL liquid handling now also consumes shared liquid-alpha resolution / transparent-water safety policy from `internal/renderer/world/*`, keeping OpenGL-specific code focused on draw-time usage rather than ownership of the underlying rules
- embedded sky rendering supports `r_fastsky`: when enabled and no external skybox path is active, sky draw calls bind a per-texture flat-color solid layer and a transparent alpha fallback to disable cloud scrolling while preserving fog and sky animation frame selection behavior
- embedded sky rendering also supports a narrow deterministic procedural baseline: when both `r_fastsky` and `r_proceduralsky` are enabled and no external skybox path is active, the OpenGL sky pass uses a dedicated procedural shader with fixed horizon/zenith colors instead of texture-backed layers
- non-fast embedded sky motion is cvar-tunable through `r_skysolidspeed` and `r_skyalphaspeed`, which scale the two scrolling sky layer offsets without changing external skybox paths
- OpenGL world texture uploads consume startup-registered texture cvars (`gl_texturemode`, `gl_lodbias`, `gl_texture_anisotropy`) for diffuse/filter selection while preserving linear lightmap filtering
