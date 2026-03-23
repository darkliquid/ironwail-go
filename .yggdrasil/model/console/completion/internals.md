# Internals

## Logic

The completer tracks the last input, the partial token being completed, the current match list, and the cycling index. When input changes, it rebuilds matches by querying injected providers, sorting results alphabetically, and deduplicating equal names while retaining type metadata for display. Hinting computes the common prefix across current matches so callers can show only the unambiguous suffix.

## Constraints

- Completion correctness depends on callers resetting state when editing context changes.
- `FileProvider` exists as part of the interface but is not yet integrated into match building.
- The logic uses its own mutex because key handling and hint queries may occur concurrently.

## Decisions

### Dependency-injected completion providers over direct imports

Observed decision:
- The console package accepts provider callbacks instead of importing `cmdsys`, `cvar`, or filesystem packages directly.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Completion stays testable and decoupled, at the cost of extra startup wiring and a few not-yet-fully-used interfaces.
