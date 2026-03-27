# Interface

## Main consumers

- `renderer/opengl-world`

## Contracts

- exposes helper surfaces for embedded sky texture/frame lookup
- exposes the canonical sky pass DTO/submission helper consumed directly by root callers (`SkyPassState`, `RenderSkyPass`)
- exposes external skybox upload helpers (`UploadSkyboxCubemap`, `UploadSkyboxFaceTextures`)

The parent node keeps sky-state snapshotting, skybox file loading, and external-sky renderer state ownership, but it now reuses one locked base `SkyPassState` snapshot across world and brush passes instead of rebuilding the full DTO twice.
