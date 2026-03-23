# Internals

## Logic

This layer owns the OpenGL backend’s event loop, context ownership, and core per-frame setup. It also carries OpenGL-specific postprocessing-like utilities such as polyblend and waterwarp support.

## Constraints

- OpenGL context ownership and thread behavior are backend-critical.
- Core state transitions must stay deterministic for the downstream world/entity render passes.

## Decisions

### Dedicated OpenGL backend slice

Observed decision:
- The OpenGL path is factored into a distinct core slice rather than being intermixed with all shared renderer code.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**
