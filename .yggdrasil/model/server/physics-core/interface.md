# Interface

## Main consumers

- `host/runtime`, which advances the server each frame
- `server/player-movement`, which relies on shared physics primitives
- `server/debug-telemetry`, which hooks into callback and frame instrumentation

## Main surface

- `Frame`
- shared physics helpers such as gravity, clip, water, and QC callback dispatch
- physics debug counters surfaced for host diagnostics (e.g., peak active edicts via `PeakEdicts`)
- match-rule enforcement helpers

## Contracts

- Active-frame ordering is significant: client command ingestion precedes physics, and rules/messages run after simulation.
- QC callback dispatch must preserve execution context and synchronize mutated/spawned edicts back into Go state.
- `PushMove` blocked callbacks must refresh pusher/check edicts into the QC VM before execution and then apply QC mutations/spawns back to authoritative server state, matching C `SV_PushMove` callback expectations.
- Physics safety checks must sanitize invalid values before they leak into world/link/network code.
- Gravity helpers must honor the optional QuakeC `gravity` edict field as a per-entity multiplier, but treat a missing or zero field value as the canonical `1.0` fallback from C `SV_AddGravity`.
- `PeakEdicts` reflects the high-water mark captured by `Physics()` after each frame's active-edict scan; this is intended for parity/debug reporting rather than gameplay logic.
- `Physics()` also updates server dev-stats edict counters each frame (`current` + `peak`) and emits a one-time warning when active edicts first exceed the classic 600 threshold.
