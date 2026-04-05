# Internals

## Logic

This layer owns the GoGPU backend’s event/render loop integration and the core frame helpers needed before world/entity drawing.
Backend-internal frame diagnostics now default to `Debug`: pipeline-creation traces, one-time device/surface dumps, GoGPU frame-state snapshots, overlay/fallback markers, and camera-matrix dumps remain available when needed without spamming normal runtime logs.
`RenderFrame` now also emits a one-shot Info summary on the first drawn world frame after each `UploadWorld`. That summary captures visible world face/triangle counts, pass buckets, entity totals, dynamic-light count, particle/decal counts, transparency mode, and whether late translucency was active, giving qbj2-scale regressions a stable runtime baseline without enabling debug logging.
That first-frame summary now resolves liquid alpha from cached shared-world geometry facts instead of reparsing worldspawn/VIS state on the logging path, so one-shot diagnostics reflect the same live cvar values without redoing immutable BSP-side work.
The backend core now owns reusable scratch slices for GoGPU world visibility/bucket planning so the per-frame world pass can recycle large qbj2-sized face lists instead of reallocating them every draw. That keeps the frame-runtime ownership of scratch memory in the backend core, while the shared-world layer continues to define the actual visibility policy and face classification rules.
The same core-owned scratch/runtime layer now also retains the reusable dynamic world index buffer used by the batched opaque GoGPU world pass. That keeps the backend responsible for the lifetime of transient frame buffers while the shared-world logic decides which opaque faces are packed into those per-frame buckets.
The same renderer-owned scratch lifetime now also includes reusable brush-entity vertex/index scratch buffers consumed by the GoGPU world/entity slice. That keeps transient buffer ownership centralized in the backend core while the world slice decides how prepared brush-entity draws are packed into those buffers each frame.
The backend core also now owns the lifetime of a small fixed-size GoGPU world batch cache keyed by `(leaf, light-signature)`, including cached sky faces, translucent world-liquid faces, batched indices, and batch descriptors. That keeps reuse state pinned to the live renderer instance while the shared-world slice decides when a cached entry is valid, and it gives movement-heavy qbj2 play a chance to hit a recent nearby-leaf entry instead of only the immediately previous one.
The same backend-owned overlay runtime state now also tracks the previous frame's dirty overlay bounds plus a reusable upload scratch buffer. That lets `flush2DOverlay` use `gogpu.Texture.UpdateRegion(...)` for the union of current and previous dirty HUD/menu bounds instead of re-uploading the full-screen RGBA overlay texture every frame, while preserving the existing single-texture draw path and correctly clearing pixels from moved or removed overlay elements.
The overlay compositor's `DrawString` path now also writes conchars pixels directly into the CPU overlay buffer instead of building temporary indexed-string and RGBA buffers first. That preserves the existing nearest-neighbor scaling and transparency behavior while removing a burst of per-string allocation and palette-conversion churn from HUD/menu frames.
When `host_speeds` is enabled, `DrawContext.RenderFrame` now also emits a `render_thread_speeds` info log from the actual GoGPU render thread. It splits the frame into clear, world, entity/decal/particle, viewmodel, scene-composite, polyblend, overlay, and total wall-clock milliseconds so performance investigations can distinguish host-side render queueing from real render-thread work.
When the global `host_speeds` cvar is enabled, `renderEntities` also emits a `render_entities_speeds` info log that splits the entity slice into opaque brush, opaque alias, sky brush, opaque liquid brush, translucent-world collection, translucent brush collection, alpha-test brush rendering, translucency flush, decals, sprites, and particle buckets. That keeps later performance work aimed at the real hot phase instead of guessing from one aggregate `entities_ms` number, and it matches the same cvar gate used by the render-thread timing log.

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
- The GoGPU scene-composite fragment shader currently uses a conservative passthrough sample (`textureSample(sceneTexture, sceneSampler, input.uv * uvScale)`) instead of the legacy waterwarp distortion math. Earlier WGSL variants using derivative-driven aspect compensation and then a reduced `textureDimensions`-based rewrite still triggered a Vulkan pipeline-creation SIGSEGV on this stack, so the live GoGPU path keeps the fullscreen blit simple to preserve runtime stability while the backend/compiler bug is investigated separately.
- GoGPU runtime adapter selection currently depends on Vulkan loader ordering. On Linux hybrid systems this can pick the integrated adapter even when `GPUPreferHighPerformance` is requested, so the renderer now applies `DRI_PRIME=1` at startup when that preference is selected and no explicit `DRI_PRIME` override is already set.

## Decisions

### Dedicated GoGPU backend slice

Observed decision:
- The GoGPU path is factored into a distinct core slice, parallel to the retired backend split.

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

### High-performance preference forces Linux PRIME default

Observed decision:
- `NewWithConfig` applies `DRI_PRIME=1` before `gogpu.NewApp` when `GPUPreferHighPerformance` is selected and `DRI_PRIME` is not already defined.

Rationale:
- Reproduction logs showed GoGPU runtime selecting the integrated AMD adapter even though the renderer requested high-performance mode; manually setting `DRI_PRIME=1` switched selection to the NVIDIA discrete adapter.
- Applying the same override in-process keeps behavior aligned with renderer intent while preserving an escape hatch (`DRI_PRIME` already set by the caller is respected).
