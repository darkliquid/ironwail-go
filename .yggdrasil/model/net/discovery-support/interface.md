# Interface

## Main consumers

- menu/UI code that displays discovered LAN servers.
- host/runtime code that wants non-frame-blocking polling or support utilities.
- admin/debug flows that need ban or partial-address behavior.

## Main surface

- `ServerBrowser`, `NewServerBrowser`, `HostCacheEntry`
- `AsyncReceiver`, `ReceivedMessage`, `PollFunc`
- `IPBan`
- `PartialIPAddress`

## Contracts

- `ServerBrowser.Start` is asynchronous and `Results` returns a sorted snapshot.
- LAN search uses Quake-style timing aligned to C Ironwail `Slist_Send`/`Slist_Poll`: broadcast immediately, retry at 750ms, stop around 1500ms.
- Browser results are deduplicated by address.
- `AsyncReceiver` copies payload bytes before delivering them so receivers own the message data.
- `PartialIPAddress` fills omitted octets from the right using the local IPv4 address and default port.
