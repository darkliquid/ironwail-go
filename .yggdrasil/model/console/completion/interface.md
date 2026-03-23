# Interface

## Main consumers

- runtime/input handling that triggers tab/shift-tab completion
- startup wiring that injects command/cvar/alias/file providers
- UI code that wants current matches or inline completion hints

## Main surface

- `NewTabCompleter`
- provider setters (`SetCommandProvider`, `SetCVarProvider`, `SetAliasProvider`, `SetFileProvider`)
- `Complete`, `GetHint`, `Reset`, `MatchCount`, `GetCurrentMatches`
- package-global completion wrappers

## Contracts

- Completion is provider-driven and intentionally decoupled from direct package imports.
- Repeated completion on unchanged input reuses the match set and cycles through it.
- Matching is case-insensitive and substring-based after provider results are gathered.
