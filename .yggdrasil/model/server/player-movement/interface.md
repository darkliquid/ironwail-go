# Interface

## Main consumers

- `server/client-session`, which feeds user commands into server-side movement
- `server/physics-core`, which provides shared simulation context and helpers

## Main surface

- player-think/move helpers
- walk/fly/swim/step movement routines
- movement tests that define expected server-side traversal behavior

## Contracts

- Movement uses authoritative server collision and contents queries; it must not bypass world-link semantics.
- User command interpretation should preserve classic Quake movement feel and edge cases.
- Movement helpers assume the shared server frame has already set current time and callback context appropriately.
