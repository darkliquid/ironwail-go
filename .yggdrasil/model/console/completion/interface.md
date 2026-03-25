# Interface

## Main consumers

- runtime/input handling that triggers tab/shift-tab completion
- startup wiring that injects command/cvar/alias/file providers
- UI code that wants current matches or inline completion hints

## Main surface

- `NewTabCompleter`
- provider setters (`SetCommandProvider`, `SetCVarProvider`, `SetAliasProvider`, `SetFileProvider`, `SetCommandArgsProvider`, `SetCVarValueProvider`, `SetPrintFunc`)
- `Complete`, `GetHint`, `Reset`, `MatchCount`, `GetCurrentMatches`
- package-global completion wrappers used by the singleton console and QC/command subsystems

## Contracts

- Completion is provider-driven and intentionally decoupled from direct package imports.
- Repeated completion on unchanged input reuses the match set and cycles through it.
- Matching is case-insensitive by prefix after provider results are gathered, then de-duplicated while preserving the user-visible cycle order.
- File completion is command-aware rather than global: `map`/`changelevel` consume VFS map listings, `exec` consumes `*.cfg`, and `playdemo`/`timedemo` consume demo names with `.dem` trimmed from the inserted token.
- First user-triggered tab with multiple matches logs a formatted match list and inserts the longest common prefix instead of immediately cycling to a specific candidate.
- Columnar match rendering honors `con_maxcols` when set (>0), otherwise auto-sizes columns from current console width.
- Argument completion can dispatch to command-specific providers and cvar-value providers, with command-name completion and argument completion treated as distinct phases.
