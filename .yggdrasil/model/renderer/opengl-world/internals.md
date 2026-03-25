# Internals

## Logic

This layer translates shared world data and visibility decisions into OpenGL world draw passes, including sky and liquid behavior specific to the GL path.

The OpenGL path now consumes the shared-world visibility helpers for leaf-face lookup and PVS face selection rather than owning that logic locally, keeping backend visibility policy aligned with the GoGPU path.
Sky pass submission now samples `r_fastsky` per frame and switches embedded sky draw calls from the two-layer scrolling textures to precomputed flat-color textures derived from opaque alpha-layer pixels, while keeping external skybox cubemap/face modes unchanged.
Sky pass state now carries both scrolling-layer textures and precomputed flat-sky textures so `r_fastsky` can switch texture bindings without introducing a second shader path.
Sky pass state now also carries a dedicated procedural-sky shader program and deterministic horizon/zenith colors, but the path is intentionally gated to the narrow embedded fast-sky case (`r_fastsky=1`, `r_proceduralsky=1`, no external skybox) so legacy scrolling and external sky modes remain untouched.
Fog state ingestion now routes through the shared fixed-step fog transition helper before world-pass uniforms read `worldFogColor/worldFogDensity`, creating a narrow deterministic transition seam for snapshot-driven fog changes.
Embedded-sky shader uniforms now also include per-layer motion multipliers sourced from shared cvar readers (`r_skysolidspeed`, `r_skyalphaspeed`), preserving legacy default speeds while allowing narrow runtime tuning of the two scrolling layers.

## Constraints

- Visibility, ordering, and sky/liquid behavior are parity-sensitive areas.
- It depends on shared world state being prepared consistently before draw.
- Fog transition handling must stay deterministic and bounded so OpenGL parity tests can assert exact baseline behavior without frame-time coupling.
- Fast-sky mode must keep fog blending and texture animation frame resolution consistent with non-fast-sky paths, only changing the bound texture content.
- Procedural-sky mode must remain deterministic and must not override external skybox cubemap/face rendering.
- Sky speed controls must remain bounded/non-negative to avoid invalid texture-coordinate regressions in the shader path.
- OpenGL world texture-mode controls must preserve Quake-style diffuse defaults while keeping lightmap filtering linear and clamping anisotropy to valid driver ranges.

## Decisions

### Separate OpenGL world slice

Observed decision:
- OpenGL world rendering is factored out from backend-neutral world prep and from non-world OpenGL rendering.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

### Parse world texture filtering from cvars at upload time

Observed decision:
- OpenGL world uploads now parse `gl_texturemode` into min/mag filters, apply `gl_lodbias` via `TEXTURE_LOD_BIAS`, and apply `gl_texture_anisotropy` via `TEXTURE_MAX_ANISOTROPY`.
- Lightmap textures keep linear min/mag filters while diffuse textures use the cvar-controlled path.

Rationale:
- C/Ironwail exposes these texture controls at runtime; hardcoded filter settings in Go prevented parity tuning and constrained visual matching.

Rejected alternative:
- Keep fixed nearest-mipmap-linear diffuse filtering and ignore lodbias/anisotropy cvars.
- Rejected because users could set startup/runtime cvars without any effect, diverging from expected OpenGL parity behavior.
