# Internals

## Logic

The subsystem centers on a `CmdSystem` instance that stores registered commands, aliases, a buffered text stream, and the current command source. Buffered execution drains the current text, splits it into commands while respecting quotes, tokenizes each line into argv-style slices, resolves it through the command/alias/cvar pipeline, and handles `wait` by reinserting deferred commands for the next frame. A package-level singleton preserves the original Quake-style global command surface for the rest of the engine.

## Constraints

- Parsing and lookup order are parity-sensitive with Quake's original command system.
- `wait` semantics only apply to buffered execution, not immediate execution.
- Unknown-command forwarding is only valid for `SrcCommand` execution; other sources stop earlier by design.

## Decisions

### Keep the command substrate narrower than the original C `cmd.c`

Observed decision:
- The Go package implements the shared command substrate while leaving many built-in command families to `host` and other consumers.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- `cmdsys` stays focused on buffering/parsing/routing semantics instead of also owning every engine command implementation.
