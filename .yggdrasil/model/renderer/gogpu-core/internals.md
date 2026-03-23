# Internals

## Logic

This layer owns the GoGPU backend’s event/render loop integration and the core frame helpers needed before world/entity drawing.

## Constraints

- Backend thread/event behavior is critical for correctness.
- Shared canvas/world data must be consumed consistently with the GoGPU runtime model.

## Decisions

### Dedicated GoGPU backend slice

Observed decision:
- The GoGPU path is factored into a distinct core slice, parallel to the OpenGL backend core.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**
