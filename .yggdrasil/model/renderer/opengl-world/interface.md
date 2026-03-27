# Interface

## Main consumers

- renderer root callers that need the OpenGL world runtime lifecycle

## Contracts

- this node exposes the renderer-root world lifecycle surface that must remain methods on `*Renderer`
- it owns the orchestration code in `internal/renderer/world_opengl.go` plus the shader source payloads in `internal/renderer/world/opengl/shaders.go`
- it delegates helper implementation details to child nodes:
  - `renderer/opengl-world/upload`
  - `renderer/opengl-world/passes`
  - `renderer/opengl-world/sky`
- it keeps the root-local support types and the remaining live wrapper methods that current callers/tests still rely on, including `glWorldMesh`, `glAliasModel`, and `worldDrawCall`
- it continues to consume shared-world helper policy for fog, liquid alpha, texture classification, and embedded-sky splitting
