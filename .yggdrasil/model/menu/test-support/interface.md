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
- Host Game tests assert both listen-toggle command paths (`listen 1` for multiplayer host settings, `listen 0` for single-player host settings) to keep menu and host networking startup in parity.
- Mock contexts intentionally capture menu-space draw calls rather than exercising real backends.
- Same-package tests can inject provider callbacks (including new-game confirmation and resume-availability gates) to cover branching UI flows without requiring host/runtime wiring.
