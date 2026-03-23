# Internals

## Logic

`Client` is intentionally broad because it is the state handoff point between network parsing, local input generation, entity interpolation, HUD state, view effects, and demo playback.

Important state families include:
- connection/signon/protocol state
- timing and interpolation buffers
- view angles and punch/viewblend state
- stats, items, and player identity tables
- entity baselines, live entities, static entities, and transient events
- prediction bookkeeping and telemetry

## Constraints

- Host behavior depends on client signon and connection state being updated consistently.
- Many sibling subsystems mutate shared `Client` state; ordering between parse, relink, prediction, and rendering matters.

## Decisions

### Shared mutable client state object

Observed decision:
- The Go package centralizes client-side runtime knowledge in one mutable `Client` struct.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- protocol, prediction, and rendering-facing code share one authoritative client state container
