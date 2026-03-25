# Internals

## Logic

The core runtime keeps commands and aliases in mutex-protected maps and stores buffered text as source-tagged chunks (`[]bufferedText`). Buffered execution drains the queued chunks in order, splits each chunk on semicolons/newlines outside of quotes, and suppresses semicolon splitting inside C-style comments so config comments cannot spawn extra commands. Both `//` line comments and `/* ... */` block comments are stripped only when they appear outside quotes, matching C tokenizer behavior used by command buffers and config `exec` flows. The resulting line is then tokenized into argv-style slices before resolving the first token by command, alias, then cvar. Alias expansion carries a recursion guard; `wait` reinserts the remaining source-tagged chunks at the head of the buffer; injected text is drained immediately so commands like `exec` or server stufftext can preempt the rest of the current pass while retaining source provenance. `cmdlist` iterates a sorted snapshot of visible commands and filters by optional lowercase prefix. `apropos`/`find` share a common `listAllContaining` path that scans visible command names/descriptions plus sorted cvars, then prints a parity-style summary line. `aliaslist` takes a shallow alias snapshot, sorts the alias names alphabetically, and prints `name : value` lines followed by a pluralized alias count summary. During cvar fallback lookup, query output now mirrors C `Cvar_Command`: default-valued cvars print `"(default)"`, changed cvars print `"(default: \"...\")"`, and only cvars without default metadata use the plain `\"name\" is \"value\"` form.

## Constraints

- C-style comments (`//` and `/* ... */`) must be stripped only outside quotes.
- Source gating (`SrcCommand`, `SrcClient`, `SrcServer`) is the main policy boundary and must remain coherent.
- Legacy wrappers (`AddText`, `InsertText`) must remain behaviorally stable but now capture the current command source so nested inserts preserve provenance.
- Forwarding is an optional hook. Unknown local commands now use the same `listAllContaining` fallback as `apropos`/`find` when no forwarder is set, preserving Ironwail suggestion behavior.
- Built-in listing/search commands must not expose server-only commands and intentionally hide reserved `__*` command names.

## Decisions

### Package-level singleton over instance-only command routing

Observed decision:
- The package exposes both an instance-based `CmdSystem` API and a global singleton API mirroring Quake's procedural command subsystem.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Most of the engine can use a familiar flat command API while tests and isolated callers can still create fresh command-system instances.

### Source-aware buffered wrappers landed (console-source-buffered-wrappers)

Observed implementation:
- Added explicit buffered wrappers: `AddTextWithSource` and `InsertTextWithSource` at both instance and package-level surfaces.
- Refactored queue storage from a single text builder to source-tagged buffered chunks so deferred execution can keep per-chunk provenance.
- Preserved compatibility by keeping `AddText`/`InsertText` wrappers and making them capture the current command source (`c.Source()`), which keeps existing call sites unchanged while preserving nested-source behavior.

Verification:
- `internal/cmdsys/cmd_test.go` now covers per-chunk source retention and insert-preemption provenance (`TestAddTextWithSourceRetainsPerChunkProvenance`, `TestInsertTextWithSourcePreemptsAndRetainsSource`).
