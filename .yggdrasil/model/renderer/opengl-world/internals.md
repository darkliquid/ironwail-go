# Internals

## Logic

This layer translates shared world data and visibility decisions into OpenGL world draw passes, including sky and liquid behavior specific to the GL path.

The OpenGL path now consumes the shared-world visibility helpers for leaf-face lookup and PVS face selection rather than owning that logic locally, keeping backend visibility policy aligned with the GoGPU path.
Sky pass submission now samples `r_fastsky` per frame and switches embedded sky draw calls from the two-layer scrolling textures to precomputed flat-color textures derived from opaque alpha-layer pixels, while keeping external skybox cubemap/face modes unchanged.
Sky pass state now carries both scrolling-layer textures and precomputed flat-sky textures so `r_fastsky` can switch texture bindings without introducing a second shader path.
Embedded-sky shader uniforms now also include per-layer motion multipliers sourced from shared cvar readers (`r_skysolidspeed`, `r_skyalphaspeed`), preserving legacy default speeds while allowing narrow runtime tuning of the two scrolling layers.

## Constraints

- Visibility, ordering, and sky/liquid behavior are parity-sensitive areas.
- It depends on shared world state being prepared consistently before draw.
- Fast-sky mode must keep fog blending and texture animation frame resolution consistent with non-fast-sky paths, only changing the bound texture content.
- Sky speed controls must remain bounded/non-negative to avoid invalid texture-coordinate regressions in the shader path.

## Decisions

### Separate OpenGL world slice

Observed decision:
- OpenGL world rendering is factored out from backend-neutral world prep and from non-world OpenGL rendering.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**
