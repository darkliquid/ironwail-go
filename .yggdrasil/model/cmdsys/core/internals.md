# Internals

## Logic

The core runtime keeps commands and aliases in mutex-protected maps and stores buffered text in a `strings.Builder`. Buffered execution drains the current text snapshot, splits it on semicolons/newlines outside of quotes, but once `//` starts outside quotes it suppresses further semicolon splitting until the next line break so config comments cannot spawn extra commands. The resulting line is then tokenized into argv-style slices, with `//` comment text stripped outside quotes before resolving the first token by command, alias, then cvar. Alias expansion carries a recursion guard; `wait` reinserts the remaining lines at the head of the buffer; injected text is drained immediately so commands like `exec` or server stufftext can preempt the rest of the current pass.

## Constraints

- `//` comments must be stripped only outside quotes.
- Source gating (`SrcCommand`, `SrcClient`, `SrcServer`) is the main policy boundary and must remain coherent.
- Forwarding is an optional hook, so unknown-command behavior depends on whether a consumer registered `ForwardFunc`.

## Decisions

### Package-level singleton over instance-only command routing

Observed decision:
- The package exposes both an instance-based `CmdSystem` API and a global singleton API mirroring Quake's procedural command subsystem.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Most of the engine can use a familiar flat command API while tests and isolated callers can still create fresh command-system instances.
