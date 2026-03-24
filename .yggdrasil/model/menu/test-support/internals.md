# Internals

## Logic

The test-support node assembles lightweight stand-ins for input and render dependencies so the rest of the menu package can be exercised deterministically. Tests operate from the package boundary: they instantiate `Manager`, drive it through key/mouse/text entry flows, inspect queued commands and cvar changes, and verify menu-space drawing by reconstructing rendered lines from captured character calls. For session-sensitive UX paths, tests inject callbacks such as `SetNewGameConfirmationProvider`, `SetResumeGameAvailableProvider`, and `SetSaveEntryAllowedProvider` to verify prompt/confirm/cancel behavior, skill-submenu branching, autosave-resume selection, and save-entry gating without bootstrapping a full host runtime. Host-game startup tests explicitly verify listen-toggle command parity for both multiplayer and single-player host settings.

## Constraints

- Tests are same-package and therefore can reach unexported fields such as `commandText`; this is part of how menu behavior is verified.
- Many tests rely on default labels, hard-coded row positions, and command sequences, so UI regressions surface as textual/draw-call mismatches rather than high-level snapshots.
- Mock backends are intentionally minimal and do not simulate real platform input semantics beyond what the menu package needs.
- Controls-menu tests now also guard parity-sensitive matrix behavior: cursor wrapping across expanded bind rows, binding-label rendering for newly surfaced commands, and clear/rebind semantics on those new rows.

## Recent update: Expanded-controls focused coverage

- Added focused manager tests for the expanded Controls binding matrix:
  - cursor navigation wrap from `BACK` to first row (and back),
  - label generation for a newly added command (`centerview`),
  - clear/rebind flow on a newly added command row while preserving existing semantics.
  - mouse-hover lock during rebinding (`KMouse1` enter capture, `M_MousemoveAbsolute` no-op until cancel).
- These tests keep coverage narrow and behavior-oriented, so matrix growth remains safe without broad menu snapshot churn.

## Decisions

### Test the menu from the package boundary with lightweight mocks instead of a full UI harness

Observed decision:
- Menu tests use local mock backends/render contexts and drive the manager directly through its exported-ish input/draw entrypoints.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- The package gets broad behavioral coverage without a full runtime harness, but tests encode many menu-specific constants and same-package assumptions.
