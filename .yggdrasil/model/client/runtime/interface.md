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
- serialize outbound client string commands for signon replies and forwarded console commands

## Contracts

- `Client` is the shared state target for parser, input, prediction, relink, and demo logic.
- Signon count and connection state transitions affect host behavior and downstream rendering/audio activation.
- Pitch-drift tuning fields on `Client` (`CenterMove`, `CenterSpeed`) are the authoritative runtime values consumed by input drift logic; startup/control-cvar sync updates these from `v_centermove` and `v_centerspeed`.
- `SendStringCmd` preserves the literal command payload passed by callers, including newline-only strings used by the explicit `cmd` console forwarder, and wraps it in a `CLCStringCmd` message.
- Fog target state lives on `Client`; both parsed `svc_fog` updates and local `fog` console commands must route through the same fade-preserving update helper so in-flight fades restart from the current interpolated fog value.
