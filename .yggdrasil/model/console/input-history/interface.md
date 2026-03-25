# Interface

## Main consumers

- runtime/input handling that maps keys to prompt editing and history traversal
- completion code that overwrites the current input line

## Main surface

- `InputLine`, `SetInputLine`, `AppendInputRune`, `BackspaceInput`
- `CommitInput`, `PreviousHistory`, `NextHistory`
- cursor/editing APIs: `CursorPos`, `MoveCursorLeft/Right`, `MoveCursorStart/End`, `DeleteInput`, `DeleteWordLeft/Right`, `ToggleInsertMode`
- package-level wrappers for the same operations plus print/callback helpers consumed outside the `Console` type

## Contracts

- Control characters are filtered from normal text entry.
- Committing nonblank input appends to history, deduplicating consecutive duplicates and capping history length.
- Cursor motion and word-delete helpers clamp to valid prompt bounds and preserve an editable backup when traversing back from history into the live prompt.
- Moving past the newest history entry returns the user to a fresh prompt.
- Editing supports insert/overwrite semantics with in-line cursor operations (not append-only), mirroring C console key behavior.
