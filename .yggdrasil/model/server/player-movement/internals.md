# Internals

## Logic

This node specializes the general physics system for player-controlled entities. It interprets current move commands, performs slide/step/water movement against authoritative traces, and updates orientation/velocity state used by both gameplay and later network snapshots.

## Constraints

- Small movement-order changes can surface as visible parity regressions.
- Ground/water classification depends on collision queries staying synchronized with the current world link state.
- Player motion is intentionally server-authoritative even when client prediction exists elsewhere.

## Decisions

### Separate player movement layer over shared physics primitives

Observed decision:
- Player movement lives in a dedicated layer instead of being folded entirely into generic movetype simulation.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- User-command handling and Quake-specific traversal quirks are easier to reason about without mixing them into every other movetype path.
