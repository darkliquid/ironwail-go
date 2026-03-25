# Interface

## Main consumers

- `host`, which registers most engine commands and pushes config/script text through the buffer
- input/menu/QC/runtime code that injects or executes command text
- console completion paths that enumerate commands and aliases

## Main surface

- `CmdSystem` construction and package-level singleton helpers
- built-in registry commands: `wait`, `cmdlist`, `apropos`, `find`, `aliaslist`
- `AddCommand`, `AddClientCommand`, `AddServerCommand`, `RemoveCommand`
- `AddAlias`, `RemoveAlias`, `UnaliasAll`, `Alias`, `Aliases`
- `AddText`, `AddTextWithSource`, `InsertText`, `InsertTextWithSource`, `Execute`, `ExecuteWithSource`, `ExecuteText`, `ExecuteTextWithSource`
- `SetSource`, `Source`, `Exists`, `Complete`, `CompleteAliases`, `SetForwardFunc`

## Contracts

- Command names and aliases are normalized to lowercase.
- Resolution order and source filtering are parity-critical.
- `InsertText` and `wait` semantics define command ordering guarantees relied on by config/script execution.
- Source-aware buffered wrappers (`AddTextWithSource`, `InsertTextWithSource`) must preserve per-chunk provenance through deferred execution, even when injected text preempts remaining buffered lines.
- `cmdlist` and `apropos`/`find` only surface non-server commands; names beginning with `__` remain hidden as reserved/internal entries.
- Cvar query output must include default-state annotations to match Ironwail semantics: `"(default)"` when current value equals the default and `"(default: \"...\")"` when it differs.
- For local (`SrcCommand`) unknown commands without a forwarding hook, the fallback must invoke apropos-style listing (`listAllContaining`) instead of printing a plain unknown-command line.
- `aliaslist` enumerates a sorted snapshot of user-defined aliases and ends with an alias count summary.
