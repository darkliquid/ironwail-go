# Responsibility

## Purpose

`cvar` owns Quake-style console-variable registration, lookup, mutation policy, typed cached views, and archive serialization.

## Owns

- The cvar registry and package-global singleton facade.
- Canonical string storage plus cached float/int/bool access.
- Flag-based mutation restrictions such as archive, ROM, locked, and auto-cvar handling.
- Change callbacks and auto-cvar callback dispatch.
- Archive serialization and prefix completion over registered cvar names.

## Does not own

- Command parsing/routing, which belongs to `cmdsys`.
- Config file read/write orchestration, which is handled by host/runtime code.
- Serverinfo/userinfo broadcasting logic beyond storing the related flags.
