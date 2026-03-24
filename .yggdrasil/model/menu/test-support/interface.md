# Interface

## Main consumers

- the menu package's own tests.

## Main surface

- `mockInputBackend`
- `mockDrawManager`
- `mockMenuRenderContext`
- test helpers such as `renderedMenuLine`, cvar setup helpers, and the suite of menu state/draw assertions in `manager_test.go`

## Contracts

- Tests assert command ordering, menu-state transitions, provider refresh timing, and cursor/layout behavior at the package boundary.
- Mock contexts intentionally capture menu-space draw calls rather than exercising real backends.
- Same-package tests can inject provider callbacks (including the new-game confirmation gate) to cover branching UI flows without requiring host/runtime wiring.
