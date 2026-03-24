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
