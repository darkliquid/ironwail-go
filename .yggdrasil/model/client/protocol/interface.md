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
- `RelinkEntities` must run after message parsing and before renderer entity collection.
- Entities not refreshed by the latest server message are intentionally dropped from the current live render set.
