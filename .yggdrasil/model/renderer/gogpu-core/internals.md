# Internals

## Logic

This layer owns the GoGPU backend’s event/render loop integration and the core frame helpers needed before world/entity drawing.

It intentionally does not retain dormant debug rendering branches. When a helper path no longer feeds the live `renderEntities` / frame runtime flow, it should be deleted rather than kept as an unused overlay hook inside the backend core.

The GoGPU dependency version currently used by the repo still exposes `hal.Device`/`hal.Queue` through `gogpu.DeviceProvider`, so renderer code cannot directly swap pipeline creation over to the public `*wgpu.Device` API without re-plumbing the backend boundary. Instead, `validatedGoGPURenderPipeline` mirrors the public wrapper’s behavior by calling `wgpu/core.ValidateRenderPipelineDescriptor` with default limits before delegating to HAL pipeline creation, which catches missing shader modules, entry points, fragment targets, and similar spec-level descriptor mistakes before Vulkan sees them.

## Constraints

- Backend thread/event behavior is critical for correctness.
- Shared canvas/world data must be consumed consistently with the GoGPU runtime model.
- In frames where `RenderFrameState.DrawWorld` is false but `MenuActive` is true, the frame-clear step must not wipe the existing surface to black, otherwise menu overlays lose the expected frozen-world backdrop.
- All render/draw mutations must be pinned to the dedicated render thread by running through `OnDraw`.
- `OnUpdate` is for event-loop-side staging only; moving draw mutations there is unsafe for the GoGPU backend.
- `warpscale_gogpu_test.go` includes a menu-only regression check that locks this clear-skipping rule so future frame-pipeline refactors do not reintroduce black menu backgrounds.
- Go source in this node aliases the standard library `image` import (`stdimage`) where needed because Quake pic types come from `internal/image`; this avoids symbol collision while preserving screenshot/export behavior.

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

### Render-pipeline validation before HAL submission

Observed decision:
- GoGPU render-pipeline creation now validates `hal.RenderPipelineDescriptor` values through `wgpu/core.ValidateRenderPipelineDescriptor` before calling `hal.Device.CreateRenderPipeline`.
- The renderer keeps HAL queue/encoder/render-pass submission unchanged and only inserts validation at pipeline-creation time.

Rationale:
- User confirmed runtime crashes were reaching Vulkan pipeline creation with invalid descriptors, while the WebGPU spec requires pipeline descriptors to be validated up front.
- In the gogpu/wgpu version currently vendored by the repo, the renderer-facing device provider still exposes `hal.Device`, so mirroring the public wrapper’s validation step is the version-correct way to get spec-aligned checks without a wider backend API rewrite.
