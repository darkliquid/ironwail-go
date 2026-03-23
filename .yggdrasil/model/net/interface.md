# Interface

## Main consumers

- host code that initializes networking, listens, connects, and answers LAN server info requests.
- server code that accepts sockets and sends framed messages.
- client code that receives protocol messages and interprets `protocol.go` constants.
- menu code that uses the LAN browser.

## Main surface

- transport/runtime facade: `Init`, `Shutdown`, `SetHostPort`, `Listen`, `CheckNewConnections`, `Connect`, `Close`, `GetMessage`, `SendMessage`, `SendUnreliableMessage`, `CanSendMessage`, `CanSendUnreliableMessage`
- transport types: `Socket`, `NewSocket`, `ServerInfoProvider`, `SetServerInfoProvider`, `Loopback`, `Buffer`
- discovery/support helpers: `ServerBrowser`, `HostCacheEntry`, `AsyncReceiver`, `IPBan`, `PartialIPAddress`, `GlobalStats`
- protocol contract: message IDs, protocol versions, encoding flags, and helper encoders/decoders in `protocol.go`

## Contracts

- `net.go` is the public dispatcher layer; callers do not choose loopback vs UDP per call.
- Reliable datagram sends are stop-and-wait and only allow one reliable message in flight.
- Protocol constants in `protocol.go` are the canonical cross-package wire contract.
