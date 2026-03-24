# Internals

## Logic

This node manages the boundary between transport-level client presence and authoritative in-game player state. It allocates/binds player slots, sends initial serverinfo, progresses clients through signon stages, ingests `UserCmd` packets and string commands, and coordinates QC hooks for player spawn and disconnect behavior. During signon it refreshes the global secret/monster counters from the QC VM before handing them to the serializer, so newly spawned clients inherit correct intermission stats. For client-issued `ban` commands, the server now mirrors C dispatch behavior by handling them in the client-string-command path: in non-deathmatch it applies/query-disables the shared `internal/net` IP ban (`SetIPBan` / `IPBanStatus`) and reports status/errors via `SV_ClientPrintf`; in deathmatch it no-ops.

## Constraints

- Signon ordering is parity-sensitive and easy to regress.
- Loopback/local clients must still follow the same authoritative session rules as remote clients where behavior matters.
- Transport-level connection surges (e.g. handshake while server is full) must be handled as non-fatal admission rejections, not frame-fatal errors.
- Spawnparm preservation and loadgame flows intentionally alter the normal `SetNewParms`/spawn path.

## Decisions

### Explicit client signon state machine on top of the server runtime

Observed decision:
- Client admission/signon is represented as staged server-side state rather than a purely implicit message sequence.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Connect/spawn transitions are easier to test and debug, especially when signon buffer flushing or save/load behavior changes.
