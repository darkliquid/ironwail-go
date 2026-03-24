# Interface

## Main consumers

- OpenGL and GoGPU world/entity render paths

## Contracts

- this node prepares or exposes the backend-neutral structures that concrete world renderers consume
- this node owns backend-neutral leaf-to-face lookup and PVS face selection so OpenGL and GoGPU can consume the same visibility subset
- world data and surface/texture conventions must stay stable across backends
- shared fog helpers expose deterministic one-step transition blending (`blendFogStateTowards`) so backends can soften abrupt fog state changes without introducing time-based nondeterminism
- shared sky helpers expose `readWorldFastSkyEnabled`, `readWorldProceduralSkyEnabled`, sky-layer speed cvar readers, procedural-sky gating/color helpers, and `buildSkyFlatRGBA` so backend world runtimes can read `r_fastsky`/`r_proceduralsky`/layer-speed controls and derive deterministic embedded-sky fallbacks without changing external skybox paths
- `BuildTextureAnimations` treats any `'+'`-prefixed texture name as an animation candidate and delegates frame-token validation to `textureAnimationFrame`, returning explicit "bad animating texture" errors for malformed tokens instead of silently skipping them
