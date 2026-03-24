# Internals

## Logic

This layer is the bridge between BSP/model-facing world data and backend-specific submission. It centralizes data preparation and shared rendering rules so OpenGL and GoGPU can consume consistent inputs.

It now also owns the shared BSP visibility helpers that map leaf mark-surfaces to built world faces and select the camera-visible face subset from a PVS mask. Backend-specific render loops are expected to consume these shared results rather than reimplementing leaf visibility policy independently.
The shared sky helper layer now includes flat-sky color synthesis for `r_fastsky`: it averages non-transparent alpha-layer pixels into a 1x1 RGBA color swatch that backend runtimes can upload and bind for fast-sky rendering.
OpenGL world-runtime upload now builds and stores a per-sky-texture 1x1 fast-sky texture cache from this helper output, and world teardown releases that cache with other sky textures.

## Constraints

- Shared world data must be backend-neutral enough for both OpenGL and GoGPU.
- Fog, sky, liquid alpha, and lightmap helpers are parity-sensitive and feed directly into visible output differences.
- Flat-sky color derivation must ignore transparent alpha-layer pixels so fast-sky output stays stable across maps and texture animations.

## Decisions

### Shared world prep below multiple backends

Observed decision:
- The renderer centralizes some world preparation in backend-neutral helpers rather than duplicating all world logic per backend.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**
