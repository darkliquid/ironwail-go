# Internals

## Logic

This layer is the bridge between BSP/model-facing world data and backend-specific submission. It centralizes data preparation and shared rendering rules so OpenGL and GoGPU can consume consistent inputs.

It now also owns the shared BSP visibility helpers that map leaf mark-surfaces to built world faces and select the camera-visible face subset from a PVS mask. Backend-specific render loops are expected to consume these shared results rather than reimplementing leaf visibility policy independently.
The shared sky helper layer now includes flat-sky color synthesis for `r_fastsky`: it averages non-transparent alpha-layer pixels into a 1x1 RGBA color swatch that backend runtimes can upload and bind for fast-sky rendering.
The shared sky helper layer also normalizes cvar-driven embedded-sky layer speed controls (`r_skysolidspeed`, `r_skyalphaspeed`) with stable defaults and non-negative clamping so backend sky passes can safely consume runtime-tunable motion multipliers.
OpenGL world-runtime upload now builds and stores a per-sky-texture 1x1 fast-sky texture cache from this helper output, and world teardown releases that cache with other sky textures.
Texture animation chain building now treats any `'+'`-prefixed name as an animation participant and relies on `textureAnimationFrame` for token validation. This closes a narrow parity gap where a malformed `"+"` texture name was previously skipped silently (due to a pre-validation length guard) instead of surfacing the canonical "bad animating texture" error path used for other malformed animated names.

## Constraints

- Shared world data must be backend-neutral enough for both OpenGL and GoGPU.
- Fog, sky, liquid alpha, and lightmap helpers are parity-sensitive and feed directly into visible output differences.
- Flat-sky color derivation must ignore transparent alpha-layer pixels so fast-sky output stays stable across maps and texture animations.
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
