# Internals

## Logic

This layer is the bridge between BSP/model-facing world data and backend-specific submission. It centralizes data preparation and shared rendering rules so OpenGL and GoGPU can consume consistent inputs.

## Constraints

- Shared world data must be backend-neutral enough for both OpenGL and GoGPU.
- Fog, sky, liquid alpha, and lightmap helpers are parity-sensitive and feed directly into visible output differences.

## Decisions

### Shared world prep below multiple backends

Observed decision:
- The renderer centralizes some world preparation in backend-neutral helpers rather than duplicating all world logic per backend.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**
