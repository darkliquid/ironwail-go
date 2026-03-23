# Interface

## Primary consumers

- `internal/host`, which drives client frame phases and signon transitions.
- sibling client nodes that mutate or read `Client`.
- renderer/HUD/audio consumers that read client state and transient events.

## Main API

### `Client`

Observed responsibilities:
- store authoritative client-side runtime state
- expose connection/signon/state transitions
- hold entity baselines/live entities and transient event queues
- hold precache lists, stats, and view-related state

## Contracts

- `Client` is the shared state target for parser, input, prediction, relink, and demo logic.
- Signon count and connection state transitions affect host behavior and downstream rendering/audio activation.
