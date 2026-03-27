# Internals

## Organization

OpenGL world backend implementations are now consolidated into `internal/renderer/world_opengl.go`. Earlier root seam fragments (`world_runtime_opengl_root.go`, `world_sprite_opengl_root.go`, `world_sky_opengl_root.go`, `world_sky_pass_opengl_root.go`, `world_sky_support_opengl_root.go`, `world_sky_texture_opengl_root.go`, `world_upload_opengl_root.go`, `world_render_opengl_root.go`, `world_alias_opengl_root.go`, `world_probe_opengl_root.go`, plus support/shader helpers) were merged into that file once tagged validation showed the split was only organizational. Core type definitions (`WorldGeometry`, `WorldVertex`, `WorldFace`) remain in the shared world layer and renderer root. The stale duplicate `internal/renderer/world/opengl/` tree has already been removed.

**Mapped files:**
- `world_opengl.go` — consolidated OpenGL world backend covering lifecycle, upload/runtime, sky setup/pass/helpers, render submission, alias handling, probes, and runtime support
- `world/opengl/shaders.go` — OpenGL world and sky shader source payloads extracted into a true backend subpackage seam
- `world/opengl/textures.go` — OpenGL texture-mode parsing and GL texture upload primitives extracted into the backend subpackage so root code can delegate pure GL texture setup without depending on renderer-owned state

## Logic

This layer translates shared world data and visibility decisions into OpenGL world draw passes, including sky and liquid behavior specific to the GL path.

The OpenGL path now consumes the shared-world visibility helpers for leaf-face lookup and PVS face selection rather than owning that logic locally, keeping backend visibility policy aligned with the GoGPU path.
As part of the refactor, root-owned OpenGL world lifecycle, sprite, sky setup, upload/runtime, render, alias, shader, and probe methods were first hoisted out of `internal/renderer/world/opengl/` into root seam files. Once tagged validation proved those seams were the only live path, the stale subdirectory copies were deleted, and the seam fragments were then merged into `world_opengl.go` to reduce root-file sprawl. The next extraction step has now begun again: world and sky shader source payloads live in `internal/renderer/world/opengl/shaders.go`, and pure GL texture-mode/upload helpers now live in `internal/renderer/world/opengl/textures.go`, giving the root backend a narrow subpackage seam for OpenGL-only assets and upload primitives without moving receiver-bound renderer lifecycle code yet.
Sky pass submission now samples `r_fastsky` per frame and switches embedded sky draw calls from the two-layer scrolling textures to precomputed flat-color textures derived from opaque alpha-layer pixels, while keeping external skybox cubemap/face modes unchanged.
Sky pass state now carries both scrolling-layer textures and precomputed flat-sky textures so `r_fastsky` can switch texture bindings without introducing a second shader path.
Sky pass state now also carries a dedicated procedural-sky shader program and deterministic horizon/zenith colors, but the path is intentionally gated to the narrow embedded fast-sky case (`r_fastsky=1`, `r_proceduralsky=1`, no external skybox) so legacy scrolling and external sky modes remain untouched.
Fog state ingestion now routes through the shared fixed-step fog transition helper before world-pass uniforms read `worldFogColor/worldFogDensity`, creating a narrow deterministic transition seam for snapshot-driven fog changes.
Embedded-sky shader uniforms now also include per-layer motion multipliers sourced from shared cvar readers (`r_skysolidspeed`, `r_skyalphaspeed`), preserving legacy default speeds while allowing narrow runtime tuning of the two scrolling layers.
The OpenGL liquid helper slice no longer owns the canonical liquid-alpha policy. It now delegates worldspawn override parsing, cvar-backed liquid alpha resolution, and transparent-water safety checks to shared-world helpers inside the consolidated backend file.

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
