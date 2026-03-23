# Internals

## Logic

This layer implements the GoGPU equivalents of world/entity/effect rendering that in the OpenGL path are spread across dedicated GL files.

## Constraints

- Backend parity with the OpenGL path is a major ongoing concern.
- Late-translucent and decal handling are especially sensitive to ordering differences.

## Decisions

### GoGPU world/entity slice parallel to OpenGL

Observed decision:
- The GoGPU path mirrors many renderer concerns with dedicated files rather than trying to express all rendering through one shared implementation.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**
