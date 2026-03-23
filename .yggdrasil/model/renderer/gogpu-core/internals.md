# Internals

## Logic

This layer owns the GoGPU backend’s event/render loop integration and the core frame helpers needed before world/entity drawing.

## Constraints

- Backend thread/event behavior is critical for correctness.
- Shared canvas/world data must be consumed consistently with the GoGPU runtime model.
- All render/draw mutations must be pinned to the dedicated render thread by running through `OnDraw`.
- `OnUpdate` is for event-loop-side staging only; moving draw mutations there is unsafe for the GoGPU backend.

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
