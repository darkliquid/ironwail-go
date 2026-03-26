# Internals

## Logic

This layer owns the GoGPU backend’s event/render loop integration and the core frame helpers needed before world/entity drawing.

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
