# Interface

## Main consumers

- `host`, which registers most engine commands and pushes config/script text through the buffer
- input/menu/QC/runtime code that injects or executes command text
- console completion paths that enumerate commands and aliases

## Main surface

- `CmdSystem` construction and package-level singleton helpers
- `AddCommand`, `AddClientCommand`, `AddServerCommand`, `RemoveCommand`
- `AddAlias`, `RemoveAlias`, `UnaliasAll`, `Alias`, `Aliases`
- `AddText`, `InsertText`, `Execute`, `ExecuteWithSource`, `ExecuteText`, `ExecuteTextWithSource`
- `SetSource`, `Source`, `Exists`, `Complete`, `CompleteAliases`, `SetForwardFunc`

## Contracts

- Command names and aliases are normalized to lowercase.
- Resolution order and source filtering are parity-critical.
- `InsertText` and `wait` semantics define command ordering guarantees relied on by config/script execution.
