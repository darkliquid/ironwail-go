# Internals

## Logic

The package is organized around a central `Manager` state machine plus child areas for drawing primitives and screen-specific behaviors. The manager holds all active page state, wires providers and callbacks, redirects input destinations when the menu opens or closes, and dispatches keys/chars/mouse movement to the active screen implementation. Drawing is a second step that renders the current page in Quake's fixed virtual coordinate space using shared picture/text helpers.

## Constraints

- Cursor behavior, wrap rules, and return-to-parent semantics are screen-specific and hard-coded rather than stack-driven.
- Many screens synchronize from live cvars/providers when entered, so opening a page is part of its behavioral contract.
- Tests span menu activation, command queueing, provider refresh, and rendering helpers, so the umbrella node must preserve relationships between all child areas.

## Decisions

### Replace Quake's file-level menu globals with an explicit manager object

Observed decision:
- The Go port centralizes menu runtime state in a `Manager` struct instead of leaving it in file-level statics spread across menu functions.

Rationale:
- **unknown — inferred from code and comments, not confirmed by a developer**

Observed effect:
- Menu state transitions and per-screen cursors are easier to test and inject, but the manager becomes the central owner of many heterogeneous menu fields.
