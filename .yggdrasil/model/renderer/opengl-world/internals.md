# Internals

## Logic

This layer translates shared world data and visibility decisions into OpenGL world draw passes, including sky and liquid behavior specific to the GL path.

The OpenGL path now consumes the shared-world visibility helpers for leaf-face lookup and PVS face selection rather than owning that logic locally, keeping backend visibility policy aligned with the GoGPU path.

## Constraints

- Visibility, ordering, and sky/liquid behavior are parity-sensitive areas.
- It depends on shared world state being prepared consistently before draw.

## Decisions

### Separate OpenGL world slice

Observed decision:
- OpenGL world rendering is factored out from backend-neutral world prep and from non-world OpenGL rendering.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**
