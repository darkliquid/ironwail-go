# Internals

## Logic

This layer centralizes effect- and entity-oriented helper logic that should not need to know the concrete graphics backend. It reduces duplication across backend render paths.

## Constraints

- Dynamic light, particle, and alpha behavior feed directly into visible parity outcomes.
- Shared helper behavior must stay consistent across multiple backend renderers.
- Runtime parity requires dynamic-light emission and world-light contribution math to share the same `r_dynamic` gate semantics to avoid "spawn disabled but still lit" or "spawn enabled but never contributes" drift.
- Particle parity with C/Ironwail depends on consuming a process-global rand stream when explicit RNG injection is absent; per-call fixed-seed RNG fallbacks desynchronize long-running effect sequences.

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

### Gate dynamic-light spawn and contribution with `r_dynamic`

Observed decision:
- Dynamic-light helper entrypoints in this node now check `r_dynamic` before spawning temporary/keyed lights and before evaluating per-point contribution sums.

Rationale:
- Dynamic-light parity from C/Ironwail expects `r_dynamic=0` to disable both creation and visual contribution of dynamic lights; gating one side without the other leaves inconsistent lighting behavior.

Rejected alternative:
- Gate only spawn-side calls and leave contribution evaluation unchanged.
- Rejected because pre-existing active lights would continue contributing while new lights stop spawning, violating the expected hard-off behavior.

### Use shared compatibility rand stream when effect helpers receive nil RNG

Observed decision:
- Particle helpers (`RunParticleEffect`, `ParticleExplosion2`, `BlobExplosion`, `LavaSplash`, `TeleportSplash`, `RocketTrail`) use `internal/compatrand` when callers do not supply an explicit `*rand.Rand`.
- Entity-particle angular velocity table initialization (`initEntityParticleAngularVelocities`) also consumes the shared compatibility stream.

Rationale:
- C/Ironwail particle code consumes the process-global libc rand() sequence; replacing nil-RNG paths with per-call fixed seeds causes repeated local subsequences and visible long-run parity drift.

Rejected alternative:
- Keep deterministic per-call `rand.New(rand.NewSource(1))` fallbacks for nil RNG paths.
- Rejected because this repeats the same sequence at every call site and diverges from process-global rand progression used by C/Ironwail.
