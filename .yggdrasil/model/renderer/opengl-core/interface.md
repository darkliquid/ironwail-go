# Interface

## Main consumers

- runtime code that selects or drives the OpenGL backend

## Contracts

- this node fulfills the package-level backend contract for the OpenGL path
- it must coordinate with shared canvas and world/entity helpers without leaking backend specifics upward
- `Renderer.SetConfig` always updates the cached `Config`; monitor/vsync side effects are only applied when an OpenGL window is initialized
- `Renderer` tracks OpenGL-side fast-sky resources (`worldSkyFlatTextures`) and the procedural-sky shader handles alongside other world texture/program caches so map reload/clear paths can release them deterministically
- `DrawContext.RenderFrame` preserves the previously rendered world when `MenuActive` is true and `DrawWorld` is false, so in-game menu overlays do not hard-clear to black after returning from gameplay
- before invoking the 2D overlay callback, `DrawContext.RenderFrame` guarantees alpha blending is enabled with `SRC_ALPHA`/`ONE_MINUS_SRC_ALPHA` so menu and console backdrop alpha behave correctly after 3D world passes
