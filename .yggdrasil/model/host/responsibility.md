# Responsibility

## Purpose

The `host` node coordinates engine startup, shutdown, frame timing, command processing, session state, and subsystem lifetime for the Go port. It is the control layer that binds filesystem, commands, console, server, client, renderer, audio, input, and menu state into one running executable.

Primary evidence:
- `internal/host/doc.go`
- `internal/host/types.go`
- `internal/host/init.go`
- `internal/host/frame.go`

## Owns

- Host process state and timing, including realtime, frametime, frame counters, net-frame isolation, client/server activation state, loading plaque state, demo loop state, and autosave tracking.
- Startup ordering for subsystem initialization and shutdown ordering for subsystem teardown.
- Registration of host-facing commands such as map/session, network, save/load, demo, audio, and system commands.
- Local loopback session bootstrap and remote datagram client session management.
- Config execution flow for `quake.rc`, archived cvar loading, and user config fallback.
- Gameplay-facing save/autosave policy and bookkeeping.

## Does not own

- Server simulation details such as physics, entity visibility, world collision, or QC execution. Those belong to the server and QC nodes.
- Client-side entity parsing, interpolation, prediction, or demo state internals. Host drives those phases but does not implement them.
- Renderer, audio, input, filesystem, or menu implementation details beyond lifecycle coordination and high-level command integration.
- CVar storage or command parsing mechanics. Host registers and uses those services but does not implement them.

## Boundaries

`host` is the orchestration boundary between the executable wiring in `cmd/ironwailgo` and the runtime subsystems under `internal/`. It is responsible for sequencing and policy, not for reproducing subsystem internals.
