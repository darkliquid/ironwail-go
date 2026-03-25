# Internals

## Logic

`Client` is intentionally broad because it is the state handoff point between network parsing, local input generation, entity interpolation, HUD state, view effects, and demo playback.

Important state families include:
- connection/signon/protocol state
- timing and interpolation buffers
- view angles and punch/viewblend state
- pitch-drift tuning state (`CenterMove`, `CenterSpeed`) consumed by input drift recentering
- stats, items, and player identity tables
- entity baselines, live entities, static entities, and transient events
- fog target/fade state shared by parser, local console commands, demo capture, and renderer-facing readers
- prediction bookkeeping and telemetry

Runtime regression coverage includes parser-level tests in `internal/client/client_test.go` that verify entity slots are still updated when the command byte is `0xFF` (all fast-update low bits set), preventing accidental early termination of message parsing and downstream missing/disappearing entities. Coverage also includes the active-weapon overflow path where `svc_clientdata` carries a truncated byte and a following `svc_updatestat(STAT_ACTIVEWEAPON)` restores the full bitmask (Alkaline compatibility behavior).

Command-send behavior samples one-shot controls at send time via `BuildPendingMove` (instead of relying on values pre-written during frame accumulation). This keeps button/impulse latching aligned with wire emission timing and avoids dropping presses that occur between accumulation and transmission.

String-command serialization is intentionally thin: `SendStringCmd` now preserves the exact caller-provided payload instead of trimming whitespace first, because C-style console forwarding can intentionally send a bare newline for `cmd` with no arguments and host/network layers rely on the client serializer not to rewrite that payload.
Runtime now exposes `SignonReplyCommands(...)` so staged signon side effects (name/color/spawn/begin command emission) remain centralized with client runtime semantics and shared by host remote-client handshake code.

Fog state now has a small shared mutation surface: `SetFogState` snapshots the current interpolated fog via `CurrentFog` before swapping the target density/color/time, so both network-driven `svc_fog` messages and local `fog` console commands restart fades from the same visible in-flight value instead of snapping back to stale targets.
`ApplyWorldspawnFogDefaults` handles the missing C/Ironwail map-load baseline by parsing the BSP worldspawn `"fog"` key once, seeding density/color (including C's gray no-fog default), and then handing later transitions back to the normal `SetFogState` path without reapplying defaults after fog has already been configured.

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
