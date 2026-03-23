# Interface

## Main consumers

- `internal/host`, which calls client send/read phases and relies on signon progression.
- renderer/HUD/audio consumers that read the resulting client state and transient events.

## Main API

- `Parser.ParseServerMessage(data []byte) error`
- relink/interpolation helpers such as `Client.RelinkEntities()`
- temp-entity and view-blend update paths through client state mutation

## Contracts

- Message parsing must preserve Quake protocol semantics closely enough for signon, clientdata, entities, sounds, temp entities, and stufftext behavior.
- Temp-entity coordinate/angle decoding must honor active protocol flags just like main entity parsing, because the server writes temp effects with the same flagged coord encodings.
- Beam segment roll jitter must follow C temp-entity behavior: seed compat RNG from `int(cl.time*1000)` once per update pass and consume one random roll (`rand()%360`) per emitted beam segment.
- `svc_killedmonster` / `svc_foundsecret` increment both legacy counters and HUD-facing `Stats` / `StatsF` indices so intermission overlays and score snapshots stay in sync with C behavior.
- `svc_levelcompleted` and `svc_backtolobby` (re-release opcodes) are accepted as no-op markers instead of aborting packet parsing.
- `RelinkEntities` must run after message parsing and before renderer entity collection.
- Entities not refreshed by the latest server message are intentionally dropped from the current live render set.
