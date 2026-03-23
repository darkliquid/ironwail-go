# Responsibility

## Purpose

`host/runtime` owns the core `Host` object, subsystem container lifecycle, startup/shutdown sequencing, frame scheduling, and runtime policy that the executable layer depends on.

Primary evidence:
- `internal/host/types.go`
- `internal/host/init.go`
- `internal/host/frame.go`

## Owns

- Host state fields such as timing, frame counters, client/server state, directories, demo loop state, loading plaque state, and compatibility RNG.
- Startup ordering for filesystem, commands, console, server, client, audio, and renderer initialization.
- Shutdown ordering for those same subsystems.
- Runtime frame cadence decisions, including 72 Hz network isolation when `maxFPS` is above the network tick threshold.
- Builtin startup config bootstrap through `quake.rc`, user config fallback, and default config availability.
- Host-facing LAN server browser exposure state.

## Does not own

- The implementation of console commands themselves beyond runtime registration points.
- Autosave heuristics or remote datagram session behavior; those belong to sibling host nodes.
- Concrete subsystem internals such as rendering, server simulation, client parsing, or audio mixing.
