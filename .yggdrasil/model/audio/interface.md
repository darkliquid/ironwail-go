# Interface

## Main consumer

- `internal/host` through the host audio adapter interface.

## Main exposed shape

The package exposes an adapter-backed sound system that supports:
- initialization and shutdown
- listener updates
- sound precache/start/stop
- static and ambient sound management
- music control
- diagnostics such as sound info and sound listing

Detailed contracts live in the child nodes where the runtime, backend, and music surfaces are implemented.
