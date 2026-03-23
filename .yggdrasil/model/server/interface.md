# Interface

## Main consumers

- `host/runtime`, `host/commands`, and `host/session`, which create, drive, and transition server instances.
- The executable/runtime wiring that boots a local or remote game session.
- Tests that exercise authoritative map, movement, network, and save/load behavior through the package surface.

## Main surface

- server construction, initialization, shutdown, and map spawning
- per-frame simulation and client/session handling
- collision, trace, and world-link helpers used by QC builtins and gameplay code
- server-side message and signon serialization
- save/load capture and restore APIs
- debug telemetry and QC trace controls

## Contracts

- `server` is the gameplay authority for a loaded map; other engine layers should treat it as the source of truth for entity state.
- Go edict state and QC VM state must only diverge transiently at explicit synchronization boundaries.
- Protocol/precache ordering is part of the network ABI and must remain stable across spawn, networking, and save/load paths.
