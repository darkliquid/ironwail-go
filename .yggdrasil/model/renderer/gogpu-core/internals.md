# Internals

## Logic

This layer owns the GoGPU backend’s event/render loop integration and the core frame helpers needed before world/entity drawing.
Backend-internal frame diagnostics now default to `Debug`: pipeline-creation traces, one-time device/surface dumps, GoGPU frame-state snapshots, overlay/fallback markers, and camera-matrix dumps remain available when needed without spamming normal runtime logs.

It intentionally does not retain dormant debug rendering branches. When a helper path no longer feeds the live `renderEntities` / frame runtime flow, it should be deleted rather than kept as an unused overlay hook inside the backend core.

The current gogpu version in this repo exposes public `*wgpu.Device` / `*wgpu.Queue` values from `DeviceProvider`, so GoGPU renderer code now treats those wrappers as the canonical creation/submission API. Core helpers no longer rebuild wrapper devices from HAL handles for renderer-owned paths.

## Constraints

- Backend thread/event behavior is critical for correctness.
- Shared canvas/world data must be consumed consistently with the GoGPU runtime model.
- In frames where `RenderFrameState.DrawWorld` is false but `MenuActive` is true, the frame-clear step must not wipe the existing surface to black, otherwise menu overlays lose the expected frozen-world backdrop.
- All render/draw mutations must be pinned to the dedicated render thread by running through `OnDraw`.
- `OnUpdate` is for event-loop-side staging only; moving draw mutations there is unsafe for the GoGPU backend.
- `warpscale_gogpu_test.go` includes a menu-only regression check that locks this clear-skipping rule so future frame-pipeline refactors do not reintroduce black menu backgrounds.
- Go source in this node aliases the standard library `image` import (`stdimage`) where needed because Quake pic types come from `internal/image`; this avoids symbol collision while preserving screenshot/export behavior.
- The GoGPU scene-composite fragment shader currently uses a conservative passthrough sample (`textureSample(sceneTexture, sceneSampler, input.uv * uvScale)`) instead of the OpenGL waterwarp distortion math. Earlier WGSL variants using derivative-driven aspect compensation and then a reduced `textureDimensions`-based rewrite still triggered a Vulkan pipeline-creation SIGSEGV on this stack, so the live GoGPU path keeps the fullscreen blit simple to preserve runtime stability while the backend/compiler bug is investigated separately.

## Decisions

### Dedicated GoGPU backend slice

Observed decision:
- The GoGPU path is factored into a distinct core slice, parallel to the OpenGL backend core.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

### Render mutations only through `OnDraw`

Observed decision:
- Runtime orchestration stages transient data in `OnUpdate`, then performs renderer-facing mutations such as camera updates, world uploads, and canvas changes from inside `OnDraw`.

Rationale:
- The app will crash if these mutations happen off the render thread, because most graphics driver backends require state changes to remain thread-local.
- GoGPU guarantees that code inside `OnDraw` runs on the dedicated render thread, so all render/draw mutations must be routed there.

### Public WebGPU handles for GoGPU core paths

Observed decision:
- GoGPU renderer core helpers now fetch `*wgpu.Device` / `*wgpu.Queue` from `app.DeviceProvider()` and pass those wrappers through postprocess/runtime helpers, command encoding, and submission.
- The renderer does not use `wgpu.NewDeviceFromHAL` in these creation/submission paths.

Rationale:
- This keeps GoGPU renderer-owned paths aligned with the public API surface actually exposed by gogpu, avoids redundant HAL-wrapper reconstruction, and reduces backend-boundary ambiguity during resource ownership and submission.

### Public render-pipeline creation for GoGPU core helpers

Observed decision:
- GoGPU render-pipeline creation now routes through public `wgpu.RenderPipelineDescriptor` values and `wgpu.Device.CreateRenderPipeline`.

Rationale:
- This keeps GoGPU pipeline construction on the same public API surface as command encoding and queue submission, instead of mixing public wrappers with raw HAL creation only at pipeline setup time.

### Core postprocess command recording stays on public wgpu APIs

Observed decision:
- Scene-composite/water-warp and polyblend helpers now encode and submit with public `wgpu` APIs (`CreateCommandEncoder`, `BeginRenderPass`, `Finish`, `Queue.Submit`) using the same `*wgpu.Device` / `*wgpu.Queue` handles.

Rationale:
- Keeping these core frame passes on one public API surface avoids renderer-local HAL/public mixing and keeps resource-creation, pass encoding, and submission semantics aligned.
