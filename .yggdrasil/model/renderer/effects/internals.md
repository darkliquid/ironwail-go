# Internals

## Logic

This layer centralizes effect- and entity-oriented helper logic that should not need to know the concrete graphics backend. It reduces duplication across backend render paths.

## Constraints

- Dynamic light, particle, and alpha behavior feed directly into visible parity outcomes.
- Shared helper behavior must stay consistent across multiple backend renderers.

## Decisions

### Shared effect helpers outside backend code

Observed decision:
- Many effect and skin/color helpers are kept backend-neutral rather than duplicated per renderer backend.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

### Preserve tracer alternation state in ParticleSystem

Observed decision:
- Tracer-specific state (`tracerCount`) is stored on `ParticleSystem` instead of as a `RocketTrail` local variable.

Rationale:
- Quake's tracer implementation uses a persistent static counter to alternate lateral tracer velocities and color phase across emission calls; resetting each call collapses direction alternation over time and causes visible parity drift.
