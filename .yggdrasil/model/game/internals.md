# Internals

## Current implementation state

`internal/game` currently contains only `doc.go`. There are no types, functions, or runtime logic to analyze beyond the package description.

## Observed design intent

The documentation describes a planned refactor:
- consolidate top-level game state that is currently scattered in `cmd/ironwailgo/main.go`
- centralize subsystem references into one `Game` object
- move per-frame update, entity collection, audio sync, input routing, camera/view logic, command registration, and demo helpers into that object

## Constraints

- Since the package is not implemented, all described responsibilities are provisional intent rather than guaranteed runtime behavior.
- Any future implementation should be reconciled with the existing executable orchestration in `cmd/ironwailgo` to avoid contradictory ownership.

## Decisions

### Documented placeholder package before implementation

Observed decision:
- The repository includes `internal/game` as a documented architectural target before the runtime code has been moved there.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- the intended long-term module boundary is visible to future contributors
- there is a risk that documentation and reality can diverge if the refactor does not happen or evolves differently
