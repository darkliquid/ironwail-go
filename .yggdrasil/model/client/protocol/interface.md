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
- `svc_disconnect` parsing must perform full client-runtime teardown (clear signon/protocol/entity/state fields) before transitioning to disconnected.
- `svc_bf` must trigger the same bonus-flash side effect as the `bf` command path.
- Temp-entity coordinate/angle decoding must honor active protocol flags just like main entity parsing, because the server writes temp effects with the same flagged coord encodings.
- Beam segment roll jitter must follow C temp-entity behavior by reseeding the shared compat rand stream from `int(cl.time*1000)` each update pass and consuming one random roll (`rand()%360`) per emitted beam segment.
- `svc_fog` updates must reuse the runtime fog-state helper so server-driven fog transitions start fades from the current interpolated value, matching local `fog` command behavior.
- `svc_fog` wire-format decoding must read transition time as `short/100` (C protocol format), not float payload bytes.
- `svc_killedmonster` / `svc_foundsecret` increment both legacy counters and HUD-facing `Stats` / `StatsF` indices so intermission overlays and score snapshots stay in sync with C behavior.
- `svc_levelcompleted` and `svc_backtolobby` (re-release opcodes) are accepted as no-op markers instead of aborting packet parsing.
- `RelinkEntities` must run after message parsing and before renderer entity collection.
- Entities not refreshed by the latest server message are intentionally dropped from the current live render set.
