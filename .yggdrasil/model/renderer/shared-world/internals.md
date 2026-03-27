# Internals

## Logic

This layer is the bridge between BSP/model-facing world data and backend-specific submission. It centralizes data preparation and shared rendering rules so OpenGL and GoGPU can consume consistent inputs.
As part of package-splitting work, canonical implementations for fog scaling/blending, texture metadata/classification/flag derivation, brush transform math, world pass/liquid-alpha primitives, and shared sky helper primitives now live in `internal/renderer/world/`; renderer-root `world_fog_shared.go`, `world_texture_shared.go`, `world_transform_shared.go`, `world_pass_shared.go`, `world_liquid_alpha.go`, and `world_sky_shared.go` delegate to those implementations (or adapter conversions) to preserve existing call sites.
The shared helper layer now also owns backend-neutral entity-lump parsing plus clamped alpha/bool cvar readers used by both worldspawn sky-fog overrides and OpenGL liquid/worldspawn override handling. This removes a temporary dependency where renderer-root shared sky code implicitly relied on helper functions stranded inside the OpenGL liquid file.
The same shared package now also owns the liquid-alpha policy that had been stranded in the OpenGL slice: liquid override parsing from worldspawn, cvar-backed alpha resolution, and transparent-water VIS safety checks now execute in `internal/renderer/world/liquid_alpha.go`, while renderer-root and OpenGL code keep thin compatibility wrappers with local field names.
Lightmap sample expansion now follows the same pattern: the canonical RGB-vs-monochrome normalization helper lives in `internal/renderer/world/lightmap_samples.go`, and renderer-root / OpenGL code expose thin local wrappers so existing code and tests can retain their local helper name during the transition.
Canonical geometry/lightmap metadata now also lives in `internal/renderer/world/types.go`. The root GoGPU/OpenGL slices and stub path now alias those shared types instead of redefining them locally, while each backend still keeps its own render-data container until upload/runtime concerns are split more cleanly.
The shared-world/root layer now also exposes an explicit `WorldRuntime` interface in `internal/renderer/world_runtime_shared.go`. It captures the renderer-root lifecycle surface already shared by the active backends (`UploadWorld`, `ClearWorld`, `HasWorldData`, `GetWorldBounds`, `SetExternalSkybox`) so the ongoing subpackage split has a stable root contract to preserve while moving method-heavy backend files behind wrapper/adaptor seams.

**Backend-specific implementation organization:**
- **GoGPU implementations** now live in renderer-root seam files (`internal/renderer/world_alias_gogpu_root.go`, `world_brush_gogpu_root.go`, `world_decal_gogpu_root.go`, `world_sprite_gogpu_root.go`, `world_late_translucent_gogpu_root.go`, plus support/cleanup helpers).
- **OpenGL implementations** now live in renderer-root seam files (`internal/renderer/world_runtime_opengl_root.go`, `world_render_opengl_root.go`, `world_alias_opengl_root.go`, `world_sky_*_opengl_root.go`, `world_upload_opengl_root.go`, plus support/probe helpers).
- An earlier attempt to stage true `internal/renderer/world/{gogpu,opengl}` subpackages left duplicate method-heavy copies in those directories. Those files were first quarantined, then removed once the root seam files proved to be the only live code path.
- The remaining structural reason for keeping backend code in the renderer package is method ownership on renderer-root types like `*Renderer` and `*DrawContext`; the shared `WorldRuntime` seam documents the root lifecycle boundary while shared helpers stay below it in `internal/renderer/world/*`.

It now also owns the shared BSP visibility helpers that map leaf mark-surfaces to built world faces and select the camera-visible face subset from a PVS mask. Backend-specific render loops are expected to consume these shared results rather than reimplementing leaf visibility policy independently.
The shared sky helper layer now includes flat-sky color synthesis for `r_fastsky`: it averages non-transparent alpha-layer pixels into a 1x1 RGBA color swatch that backend runtimes can upload and bind for fast-sky rendering.
The shared sky helper layer also normalizes cvar-driven embedded-sky layer speed controls (`r_skysolidspeed`, `r_skyalphaspeed`) with stable defaults and non-negative clamping so backend sky passes can safely consume runtime-tunable motion multipliers.
The same sky helper layer now also owns canonical Quake-vs-Quake64 embedded sky splitting and indexed-layer-to-RGBA conversion, so GoGPU and OpenGL no longer maintain duplicate palette-splitting implementations in backend files.
The same helper layer now also owns the narrow procedural-sky baseline policy: a dedicated `r_proceduralsky` gate, deterministic horizon/zenith colors, and a shared predicate that limits the path to embedded fast-sky rendering only.
The shared fog helper layer now also owns a narrow transition baseline (`blendFogStateTowards`) that clamps per-frame fog color/density deltas by a fixed step, providing a deterministic seam for snapshot-to-snapshot fog updates without introducing clock-based interpolation.
OpenGL world-runtime upload now builds and stores a per-sky-texture 1x1 fast-sky texture cache from this helper output, and world teardown releases that cache with other sky textures.
Texture animation chain building now treats any `'+'`-prefixed name as an animation participant and relies on `textureAnimationFrame` for token validation. This closes a narrow parity gap where a malformed `"+"` texture name was previously skipped silently (due to a pre-validation length guard) instead of surfacing the canonical "bad animating texture" error path used for other malformed animated names.

## Constraints

- Shared world data must be backend-neutral enough for both OpenGL and GoGPU.
- Fog, sky, liquid alpha, and lightmap helpers are parity-sensitive and feed directly into visible output differences.
- Fog transition blending must remain deterministic (fixed-step, no wall-clock dependency) so tests and parity captures stay reproducible.
- Flat-sky color derivation must ignore transparent alpha-layer pixels so fast-sky output stays stable across maps and texture animations.
- Procedural-sky gating must stay deterministic and must not activate for external skyboxes or non-fast-sky paths.
- Animation-name validation should not silently ignore malformed `'+'` names; invalid frame tokens must fail fast via `textureAnimationFrame` to keep texture-animation chain setup deterministic and diagnosable.

## Decisions

### Shared world prep below multiple backends

Observed decision:
- The renderer centralizes some world preparation in backend-neutral helpers rather than duplicating all world logic per backend.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

### Shared world-PVS helper as parity boundary

Observed decision:
- The shared world helpers (`buildWorldLeafFaceLookup`, `selectVisibleWorldFaces`) are the canonical parity boundary for backend world visibility decisions.
- Backend nodes are expected to consume helper outputs directly and treat world-PVS behavior changes as shared-world changes first.

Rationale:
- This keeps OpenGL and GoGPU world visibility selection aligned by construction and prevents backend-specific drift for leaf/PVS masking rules.
