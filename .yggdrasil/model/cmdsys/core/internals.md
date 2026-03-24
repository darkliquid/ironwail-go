# Internals

## Logic

The core runtime keeps commands and aliases in mutex-protected maps and stores buffered text in a `strings.Builder`. Buffered execution drains the current text snapshot, splits it on semicolons/newlines outside of quotes, and suppresses semicolon splitting inside C-style comments so config comments cannot spawn extra commands. Both `//` line comments and `/* ... */` block comments are stripped only when they appear outside quotes, matching C tokenizer behavior used by command buffers and config `exec` flows. The resulting line is then tokenized into argv-style slices before resolving the first token by command, alias, then cvar. Alias expansion carries a recursion guard; `wait` reinserts the remaining lines at the head of the buffer; injected text is drained immediately so commands like `exec` or server stufftext can preempt the rest of the current pass. `cmdlist` iterates a sorted snapshot of visible commands and filters by optional lowercase prefix. `apropos`/`find` reuse the same visible-command snapshot, then append sorted cvar hits when either the name or description contains the requested substring, printing current cvar values in the listing. `aliaslist` takes a shallow alias snapshot, sorts the alias names alphabetically, and prints `name : value` lines followed by a pluralized alias count summary.

## Constraints

- C-style comments (`//` and `/* ... */`) must be stripped only outside quotes.
- Source gating (`SrcCommand`, `SrcClient`, `SrcServer`) is the main policy boundary and must remain coherent.
- Forwarding is an optional hook, so unknown-command behavior depends on whether a consumer registered `ForwardFunc`.
- Built-in listing/search commands must not expose server-only commands and intentionally hide reserved `__*` command names.

## Decisions

### Package-level singleton over instance-only command routing

Observed decision:
- The package exposes both an instance-based `CmdSystem` API and a global singleton API mirroring Quake's procedural command subsystem.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Most of the engine can use a familiar flat command API while tests and isolated callers can still create fresh command-system instances.

### Command source attribution audit seam (console-command-source-tracking-audit)

Observed audit snapshot:
- `CommandSource` plumbing exists and is enforced at execution time (`ExecuteWithSource`, `ExecuteTextWithSource`, `SetSource`, source-gated command dispatch).
- Most ingress points still use source-implicit wrappers (`AddText`, `InsertText`, `Execute`, `ExecuteText`) that default to `SrcCommand`.
- Explicit attribution is already used in targeted host paths (`internal/host/init.go`) and via app-shell forwarding hooks, but not as the default at all ingress boundaries.

Narrowest next implementation seam:
- Add source-aware global queue wrappers in `internal/cmdsys/cmd.go` (for example `AddTextWithSource` / `InsertTextWithSource`) that preserve command provenance across buffered execution without mutating broad call-site APIs at once.
- Implement these wrappers by storing text with an associated source in the core buffer path, then executing each queued segment under `withSource(source, ...)`.

Why this seam is narrow:
- Centralizes change in `cmdsys/core` where source policy already lives.
- Avoids immediate churn across host, QC, menu, and runtime packages.
- Enables incremental migration of high-value ingress points (network/server/QC) first, while keeping legacy callers behaviorally stable.
