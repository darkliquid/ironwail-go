# Interface

## Main consumers

- runtime code selecting or driving the GoGPU backend

## Contracts

- this node fulfills the package-level backend contract for the GoGPU path
- when using the GoGPU renderer, all render/draw mutations MUST happen via the `OnDraw` callback
- `OnUpdate` may stage data for rendering, but it must not directly mutate render-thread-owned draw state
- callers must treat `OnDraw` as the only safe place to perform GoGPU-backed camera, world-upload, canvas, and other draw-state mutations because it runs on the dedicated render thread
