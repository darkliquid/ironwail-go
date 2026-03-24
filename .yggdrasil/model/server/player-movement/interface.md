# Interface

## Main consumers

- `server/client-session`, which feeds user commands into server-side movement
- `server/physics-core`, which provides shared simulation context and helpers

## Main surface

- player-think/move helpers
- walk/fly/swim/step movement routines
- debug counter helpers for `CheckBottom` (`CheckBottomStats`, `ResetCheckBottomStats`)
- movement tests that define expected server-side traversal behavior

## Contracts

- Movement uses authoritative server collision and contents queries; it must not bypass world-link semantics.
- User command interpretation should preserve classic Quake movement feel and edge cases.
- Movement helpers assume the shared server frame has already set current time and callback context appropriately.
- `MoveToGoal`/`NewChaseDir` randomness comes from `compatRand()` (shared compat-rand stream), not a local movement-only RNG, so chase-direction branch choices stay in the same global draw order as QC `random()`.
- `CheckBottom` debug counters are process-global parity counters (`c_yes`/`c_no`); callers that need deterministic assertions should reset them explicitly before sampling.
