# Internals

## Logic

Input is stored as runes so backspace and clipping stay sane for multi-byte characters. Enter commits the line, optionally appends it to a bounded history list, and clears the current prompt. Up/down traversal moves a history cursor through saved entries, with the sentinel position at `len(history)` representing the fresh editable line.

## Constraints

- History behavior is part of the user-facing console UX and can easily regress.
- The node assumes some external caller will pass committed text to `cmdsys`.
- Package-level wrappers preserve a C-style global console surface for most callers.

## Decisions

### Separate prompt/history logic from the scrollback store

Observed decision:
- Prompt editing/history is factored into its own file/API instead of being merged entirely into the scrollback core.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Input behavior is easier to evolve and test without entangling it with print/log/resize mechanics.
