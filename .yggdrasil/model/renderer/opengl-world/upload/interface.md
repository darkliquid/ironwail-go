# Interface

## Main consumers

- `renderer/opengl-world`

## Contracts

- exposes helper surfaces for CPU-side render-data assembly (`BuildWorldGeometry`, `BuildModelGeometry`, `BuildWorldRenderData`, `BuildModelRenderData`)
- exposes texture-plan and upload helpers (`BuildTextureUploadPlan`, `ApplyTextureUploadPlan`, `UploadTextureRGBA`, texture mode parsing)
- exposes lightmap update helpers (`LightStylesChanged`, `MarkDirtyLightmapPages`, `UploadLightmapPages`, `UpdateLightmapTextures`)
- exposes raw mesh upload and GL cleanup helpers (`UploadWorldMesh`, `Delete*`, `SetInt32Fields`)

These helpers stay callback-friendly where needed so the renderer-root package can keep ownership of fallback setup and specific upload call sites without creating import cycles.
