# Internals

## Logic

This layer defines the package-level renderer role without tying callers directly to GoGPU implementation details. It owns the stable package surface and adapter helpers that the rest of the engine uses while the canonical GoGPU runtime carries the concrete rendering work.

## Constraints

- Backend-specific state must stay below this layer.
- Render-pass classification logic is shared so the canonical GoGPU runtime and parity tooling compare phases against common expectations.

## Decisions

### Keep runtime seams backend-agnostic

Observed decision:
- The package keeps adapter and classification seams separate from the concrete GoGPU runtime implementation.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**
