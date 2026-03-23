# Internals

## Logic

The completer tracks the last input, the partial token being completed, the current match list, and the cycling index. When input changes, it rebuilds matches by querying injected providers, sorting results alphabetically, and deduplicating equal names while retaining type metadata for display. Hinting computes the common prefix across current matches so callers can show only the unambiguous suffix.

File completions are only queried when the current command segment expects a file-like argument. The completion logic strips preceding semicolon-separated commands, inspects the leading command token, then chooses a pattern/normalizer pair for supported commands:

- `map` / `changelevel` -> `maps/*.bsp`, inserted as bare map names
- `exec` -> `*.cfg`, inserted with the config filename
- `playdemo` / `timedemo` -> `*.dem`, inserted as bare demo names

## Constraints

- Completion correctness depends on callers resetting state when editing context changes.
- `FileProvider` remains generic, but the console package currently only consumes it through explicit command-specific specs rather than attempting shell-style path completion for every token.
- The logic uses its own mutex because key handling and hint queries may occur concurrently.

## Decisions

### Dependency-injected completion providers over direct imports

Observed decision:
- The console package accepts provider callbacks instead of importing `cmdsys`, `cvar`, or filesystem packages directly.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Completion stays testable and decoupled, at the cost of extra startup wiring and command-shape knowledge inside the console package for file-oriented commands.
