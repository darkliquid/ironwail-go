# Responsibility

## Purpose

`cmdsys` owns the Quake-style text command substrate used by the console, configs, menu actions, key bindings, and server/client command injection.

## Owns

- The command buffer and immediate execution helpers.
- Command registration, aliases, source tracking, and unknown-command forwarding.
- Quake-style command splitting/tokenization.
- Optional helper commands that manipulate cvars through the command layer.

## Does not own

- Most user-facing engine commands such as `exec`, `alias`, `writeconfig`, and gameplay commands, which are registered by `host` and other consumers.
- Cvar storage semantics themselves, which belong to `cvar`.
