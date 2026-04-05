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
- **GoGPU implementations** currently live in `internal/renderer/world_gogpu.go` with receiver-free helper extraction under `internal/renderer/world/gogpu/*`.
- **OpenGL implementations** currently live in renderer-root OpenGL world files under `internal/renderer/world_*_opengl*.go`.
- The remaining structural reason for keeping backend code in the renderer package is method ownership on renderer-root types like `*Renderer` and `*DrawContext`; the shared `WorldRuntime` seam documents the root lifecycle boundary while shared helpers stay below it in `internal/renderer/world/*`.

It now also owns the shared BSP visibility helpers that map leaf mark-surfaces to built world faces and select the camera-visible face subset from a PVS mask. Backend-specific render loops are expected to consume these shared results rather than reimplementing leaf visibility policy independently.
The shared sky helper layer now includes flat-sky color synthesis for `r_fastsky`: it averages non-transparent alpha-layer pixels into a 1x1 RGBA color swatch that backend runtimes can upload and bind for fast-sky rendering.
The shared sky helper layer also normalizes cvar-driven embedded-sky layer speed controls (`r_skysolidspeed`, `r_skyalphaspeed`) with stable defaults and non-negative clamping so backend sky passes can safely consume runtime-tunable motion multipliers.
The same sky helper layer now also owns canonical Quake-vs-Quake64 embedded sky splitting and indexed-layer-to-RGBA conversion, so GoGPU and OpenGL no longer maintain duplicate palette-splitting implementations in backend files.
The same helper layer now also owns the narrow procedural-sky baseline policy: a dedicated `r_proceduralsky` gate, deterministic horizon/zenith colors, and a shared predicate that limits the path to embedded fast-sky rendering only.
The shared fog helper layer now also owns a narrow transition baseline (`blendFogStateTowards`) that clamps per-frame fog color/density deltas by a fixed step, providing a deterministic seam for snapshot-to-snapshot fog updates without introducing clock-based interpolation.
The shared GoGPU world WGSL that still lives in `world.go` intentionally follows the OpenGL world-fragment contract for surface lighting: world and lit-water passes use the same `* 2.0` lightmap overbright factor as OpenGL, and world-surface fog blends directly by the configured fog density instead of applying a backend-only distance-squared exponential term. Keeping those formulas aligned prevents colored `.lit` maps such as qbj2 from rendering noticeably darker or more strongly tinted on GoGPU than on OpenGL.
GoGPU cutout (`{...}`) world diffuse uploads in `world.go` now also run `image.AlphaEdgeFix` before texture creation. That preserves the transparent texels' invisible alpha while replacing their RGB with nearby opaque colors, which reduces grate/fence haloing and weird edge tinting on custom BSP textures without changing opaque world materials.
OpenGL world-runtime upload now builds and stores a per-sky-texture 1x1 fast-sky texture cache from this helper output, and world teardown releases that cache with other sky textures.
Texture animation chain building now treats any `'+'`-prefixed name as an animation participant and relies on `textureAnimationFrame` for token validation. This closes a narrow parity gap where a malformed `"+"` texture name was previously skipped silently (due to a pre-validation length guard) instead of surfacing the canonical "bad animating texture" error path used for other malformed animated names.
GoGPU shared world setup now constructs public `wgpu` resource wrappers directly in `world.go`: shader modules, vertex/index/uniform buffers, texture/sampler/bind-group state, depth/render targets, and world pipeline descriptors are created from `*wgpu.Device` / `*wgpu.Queue` instead of raw HAL handles so the shared upload/setup layer matches the public renderer submission path.
Shared-world upload and render-stage tracing now logs at `Debug` instead of `Info`: geometry-build/upload summaries, GoGPU world-pass state transitions, and per-pass draw counts remain available for diagnostics without polluting normal startup/frame logs.

## Constraints

- Shared world data must be backend-neutral enough for both OpenGL and GoGPU.
- Fog, sky, liquid alpha, and lightmap helpers are parity-sensitive and feed directly into visible output differences.
- GoGPU world WGSL that remains rooted in `world.go` must preserve OpenGL-visible lighting and fog math unless an intentional parity change is being made.
- Cutout world materials must keep transparent-edge RGB padded even when alpha stays zero; otherwise custom grate/fence textures can pick up dark or off-color fringes during GPU sampling.
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
