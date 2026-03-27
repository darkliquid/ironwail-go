# Responsibility

`renderer/opengl-world` is now the root orchestration node for the OpenGL world path.

It owns the `*Renderer` methods and root-local state in `internal/renderer/world_opengl.go`:

- world lifecycle entry points (`UploadWorld`, `ClearWorld`, `HasWorldData`, `GetWorldBounds`)
- renderer-owned program/fallback creation and runtime state reset
- renderer-owned cache/state wrappers such as `glWorldMesh`, `glAliasModel`, and root draw-call orchestration
- delegation boundaries to the child helper nodes under `renderer/opengl-world/*`

It does not own the extracted helper implementations for upload/build, draw-pass helpers, or sky helpers. Those concerns now live in focused child nodes:

- `renderer/opengl-world/upload`
- `renderer/opengl-world/passes`
- `renderer/opengl-world/sky`

It also does not own backend-neutral world prep, which remains in `renderer/shared-world`.
