# Interface

## Main consumers

- menu/UI code that displays discovered LAN servers.
- host/runtime code that wants non-frame-blocking polling or support utilities.
- admin/debug flows that need ban or partial-address behavior.

## Main surface

- `ServerBrowser`, `NewServerBrowser`, `HostCacheEntry`
- `QueryServerRules`, `RuleInfoEntry`
- `QueryServerPlayers`, `PlayerInfoEntry`
- `AsyncReceiver`, `ReceivedMessage`, `PollFunc`
- `IPBan`
- `PartialIPAddress`

## Contracts

- `ServerBrowser.Start` is asynchronous and `Results` returns a sorted snapshot.
- LAN search uses Quake-style timing aligned to C Ironwail `Slist_Send`/`Slist_Poll`: broadcast immediately, retry at 750ms, stop around 1500ms.
- `Results` sorting follows C `NET_SlistSort` semantics: in-place nested-loop name comparison (`strcmp < 0`) instead of Go's stable sort helpers.
- Browser results are deduplicated by address.
- `QueryServerRules` performs the C `test2`-style control query loop synchronously: send `CCReqRuleInfo` starting with an empty name, append each returned `(name, value)` pair, and stop when the server returns an empty-name `CCRepRuleInfo`.
- `QueryServerPlayers` performs the C-style `CCReqPlayerInfo` loop synchronously: request each player index in order, append decoded rows while names are non-empty, and stop on an empty-name `CCRepPlayerInfo` terminator.
- `AsyncReceiver` copies payload bytes before delivering them so receivers own the message data.
- `PartialIPAddress` fills omitted octets from the right using the local IPv4 address and default port.
- `IPBan` models one active IPv4 address/mask pair, supports `off`/empty-address disable semantics, and exposes human-readable status text consumed by the host `ban` command.
