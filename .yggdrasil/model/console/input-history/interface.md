# Interface

## Main consumers

- runtime/input handling that maps keys to prompt editing and history traversal
- completion code that overwrites the current input line

## Main surface

- `InputLine`, `SetInputLine`, `AppendInputRune`, `BackspaceInput`
- `CommitInput`, `PreviousHistory`, `NextHistory`
- package-level wrappers for the same operations

## Contracts

- Control characters are filtered from normal text entry.
- Committing nonblank input appends to history, deduplicating consecutive duplicates and capping history length.
- Moving past the newest history entry returns the user to a fresh prompt.
