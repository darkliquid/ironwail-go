# Internals

## Logic

The umbrella package is intentionally narrow: each file contributes one small generic primitive with minimal cross-coupling. The package documentation frames these as replacements for repetitive ad-hoc engine scaffolding rather than as domain-specific abstractions.

## Constraints

- Production code in `internal/engine` imports only the Go standard library.
- The package is a utility substrate, not the owner of higher-level engine policies.
- Several primitives require constructor-based initialization; zero values are not uniformly mutation-safe.

## Decisions

### Keep foundational helpers separate from subsystem/domain packages

Observed decision:
- The Go port groups generic containers, queueing, eventing, and loader helpers into a dedicated internal package instead of embedding them inside specific subsystems.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Subsystems can adopt typed reusable helpers without depending on each other, and the package can evolve as shared infrastructure rather than as gameplay logic.
