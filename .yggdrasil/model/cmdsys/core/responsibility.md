# Responsibility

## Purpose

`cmdsys/core` owns the reusable command substrate: buffered text execution, registration, alias expansion, source gating, parser helpers, and the package-level singleton surface.

## Owns

- `CmdSystem`, `Command`, and `CommandSource`.
- The buffered execution pipeline (`AddText`, `InsertText`, `Execute`, `ExecuteText`).
- Command registration/removal and alias management.
- Source scoping, forwarding hooks, and completion helpers.
- Quake-style command splitting and tokenization.

## Does not own

- The concrete host/game/menu/player commands registered onto the system.
- Cvar helper command implementations beyond generic cvar fallback lookup.
