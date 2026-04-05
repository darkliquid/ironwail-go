# Interface

## Main consumers

- runtime code selecting or driving the GoGPU backend

## Contracts

- this node fulfills the package-level backend contract for the GoGPU path
- this node now implements the backend screenshot/export contract with a minimal deterministic PNG path, keeping command/runtime screenshot surfaces available on gogpu builds while full swapchain readback remains deferred
- when no world pass is submitted for a frame, opening the menu must preserve the already-presented scene behind the menu instead of forcing an immediate black clear
- when using the GoGPU renderer, all render/draw mutations MUST happen via the `OnDraw` callback
- `OnUpdate` may stage data for rendering, but it must not directly mutate render-thread-owned draw state
- callers must treat `OnDraw` as the only safe place to perform GoGPU-backed camera, world-upload, canvas, and other draw-state mutations because it runs on the dedicated render thread
- GoGPU core renderer setup now acquires the public `*wgpu.Device` / `*wgpu.Queue` directly from `app.DeviceProvider()` and treats those wrappers as the canonical backend handles for GoGPU resource creation and command submission
- when `GPUPreferHighPerformance` is requested and `DRI_PRIME` is unset, GoGPU runtime startup sets `DRI_PRIME=1` before creating the app so hybrid Linux systems prefer the discrete adapter by default
- GoGPU render-pipeline creation routes through `validatedGoGPURenderPipeline` in `internal/renderer/renderer_gogpu.go`, which accepts `*wgpu.Device`/`*wgpu.RenderPipelineDescriptor` and returns `*wgpu.RenderPipeline`; the HAL abstraction layer is no longer imported or used
- GoGPU postprocess helpers in `polyblend_gogpu.go` and `warpscale_gogpu.go` now use public `wgpu` descriptors/encoders/render passes/submit paths end-to-end (no renderer-side `getHAL*` fetches)
- active GoGPU core passes in this node (`clearCurrentWGPURenderTarget`, warpscale/polyblend scene composition) now use wrapper command encoding (`CreateCommandEncoder`, `BeginRenderPass`, `RenderPass.End`, `CommandEncoder.Finish`, `Queue.Submit`) with explicit pass/finish error handling
