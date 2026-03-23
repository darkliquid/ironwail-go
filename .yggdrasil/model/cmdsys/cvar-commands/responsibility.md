# Responsibility

## Purpose

`cmdsys/cvar-commands` owns the concrete console commands that mutate cvars through the command layer.

## Owns

- `RegisterCvarCommands`
- implementations for `toggle`, `cycle`, `inc`, `reset`, `resetall`, and `resetcfg`
- usage/error logging for those helper commands

## Does not own

- Generic command parsing/buffering.
- Cvar storage, registration, or persistence semantics.
