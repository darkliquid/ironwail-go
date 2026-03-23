# Internals

## Logic

The package implements an additive-feedback RNG with a 31-element state array, a Park-Miller-style seed fill, a warmup phase, and two rotating pointers that feed the next output. Each output is derived from the updated forward slot, shifted/masked into a signed 31-bit range. A package-global shared instance preserves the process-global feel of the original C `rand()` usage while still allowing explicit instance injection where the Go architecture wants it.

## Constraints

- Sequence compatibility matters more than statistical novelty.
- Consumers that should share the authoritative gameplay stream must share the same instance.
- Nil/fallback instance creation in consumers can silently fork the stream if wiring drifts.

## Decisions

### Dedicated compatibility RNG package instead of embedding rand logic in each consumer

Observed decision:
- The Go port factors Quake-compatible `rand()` behavior into a shared package used by host, server, and QC.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Parity-sensitive random draws can share one implementation and one stream contract instead of each consumer re-implementing libc-style randomness independently.
