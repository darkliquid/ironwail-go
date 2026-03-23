# Internals

## Structure

The package is split into focused concerns:
- `client/runtime` — persistent client state, connection/signon state, and shared runtime helpers
- `client/protocol` — server-message parsing, temp entities, relink/interpolation, and blend/event generation
- `client/input` — input state, view-angle adjustment, and local prediction
- `client/demo` — demo recording, playback, indexing, and timedemo behavior

## Decisions

### One client package with focused implementation slices

Observed decision:
- The Go port keeps a single public client package but factors the implementation into runtime, protocol, input, and demo concerns.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- the host and renderer can depend on one client package
- parity-sensitive logic stays grouped by responsibility rather than one monolithic file
