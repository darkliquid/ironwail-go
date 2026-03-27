# Interface

## Main consumers

- OpenGL and GoGPU world/entity render paths

## Contracts

- this node prepares or exposes the backend-neutral structures that concrete world renderers consume
- this node owns backend-neutral leaf-to-face lookup and PVS face selection so OpenGL and GoGPU can consume the same visibility subset
- world data and surface/texture conventions must stay stable across backends
- canonical implementations of fog, texture classification/flag derivation, brush transform helpers, pass/liquid alpha helper logic, and sky helper primitives now live under `internal/renderer/world/*`, while renderer-root `world_*_shared.go` files remain compatibility wrappers so existing callers/tests keep current symbol names.
- backend-neutral entity-lump parsing and alpha/bool cvar readers for worldspawn-driven sky/liquid behavior now also live under `internal/renderer/world/*`, so shared sky code and OpenGL liquid handling consume the same helper surface instead of reaching across backend files.
- canonical liquid-alpha policy now also lives under `internal/renderer/world/*`, including worldspawn liquid override parsing, cvar-backed liquid alpha resolution, and transparent-water VIS safety checks. Renderer-root and OpenGL-specific slices consume this shared policy through adapter conversions.
- canonical lightmap sample expansion now also lives under `internal/renderer/world/*`, with thin renderer-root / OpenGL wrappers preserving existing local helper names while the package split is still in progress.
- canonical world geometry/lightmap metadata (`WorldGeometry`, `WorldVertex`, `WorldFace`, `WorldLightmapSurface`, `WorldLightmapPage`) now also lives under `internal/renderer/world/*`; backend/root slices consume those types through aliases while backend-specific render-data structs remain local for now.
- renderer-root world lifecycle is now formalized as the `WorldRuntime` contract in `internal/renderer/world_runtime_shared.go` (`UploadWorld`, `ClearWorld`, `HasWorldData`, `GetWorldBounds`, `SetExternalSkybox`). This is the stable seam root callers/tests depend on while backend-specific world code stays organized as tagged root seam files under `internal/renderer/`.
- backend-specific implementations currently live in renderer-root seam files rather than true imported subpackages:
  - GoGPU backend implementations live in `internal/renderer/world_*_gogpu_root.go`
  - OpenGL backend implementations live in `internal/renderer/world_*_opengl_root.go`
  - this keeps method ownership on renderer-root types local to the renderer package while still separating shared world helpers under `internal/renderer/world/*`
- shared fog helpers expose deterministic one-step transition blending (`blendFogStateTowards`) so backends can soften abrupt fog state changes without introducing time-based nondeterminism
- shared sky helpers expose `readWorldFastSkyEnabled`, `readWorldProceduralSkyEnabled`, sky-layer speed cvar readers, procedural-sky gating/color helpers, `buildSkyFlatRGBA`, and canonical embedded-sky layer extraction helpers so backend world runtimes can read `r_fastsky`/`r_proceduralsky`/layer-speed controls and derive deterministic embedded-sky fallbacks without changing external skybox paths
- `BuildTextureAnimations` treats any `'+'`-prefixed texture name as an animation candidate and delegates frame-token validation to `textureAnimationFrame`, returning explicit "bad animating texture" errors for malformed tokens instead of silently skipping them
- GoGPU world pipeline constructors in `internal/renderer/world.go` now validate render-pipeline descriptors through `validatedGoGPURenderPipeline` before calling HAL pipeline creation, keeping world/sky/turbulent/translucent setup aligned with WebGPU descriptor rules without changing the HAL submission path
