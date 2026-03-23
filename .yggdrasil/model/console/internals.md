# Internals

## Logic

The package centers on one global console instance backed by a fixed-size ring buffer of space-padded lines. Printing updates that buffer, notify timestamps, and optional log sinks; drawing snapshots buffer state and renders either the full drop-down console or a fading notify overlay; input/history manages the editable prompt line; and tab completion maintains a small session state layered over injected providers. This keeps the console package focused on textual UI state while other subsystems handle execution and engine policy.

## Constraints

- Resize behavior must preserve the newest scrollback lines and keep notify mapping coherent.
- High-bit character conventions and notify timing are parity-sensitive.
- Several surfaces intentionally mimic C-era global/stateful APIs for easy integration.

## Decisions

### Keep console text state separate from command execution

Observed decision:
- The Go port splits textual console state/drawing from command execution and cvar ownership.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- The console can stay renderer-agnostic and testable, while `cmdsys` and other engine layers remain responsible for actually interpreting committed command text.
