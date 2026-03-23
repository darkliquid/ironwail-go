# Interface

## Main consumers

- host/server/client runtime code that needs to initialize networking, accept or open sockets, and send/receive messages.
- discovery helpers that depend on control packet constants and server-info response behavior.

## Main surface

- `Init`, `Shutdown`, `SetHostPort`, `HostPort`, `IsListening`, `Listen`, `CheckNewConnections`, `Connect`, `Close`, `GetMessage`, `SendMessage`, `SendUnreliableMessage`, `CanSendMessage`, `CanSendUnreliableMessage`
- `Socket`, `NewSocket`, `NetTime`
- `ServerInfoProvider`, `SetServerInfoProvider`
- `Loopback`, `NewLoopback`, loopback send/get/close helpers
- `Buffer`, its read/write helpers, and `GlobalStats`

## Contracts

- `Connect("local")`/`Connect("localhost")` create a loopback connection; other hosts use the datagram/UDP path.
- `UDPStringToAddr` preserves C parity by applying partial-IP expansion only when the address begins with a digit; hostname/non-numeric-leading inputs use normal UDP resolution.
- Reliable datagram traffic is stop-and-wait with at most one outstanding reliable message per socket.
- Large reliable datagram payloads are fragmented and advance only after ACK of the current fragment.
- Server-side UDP acceptance returns a newly opened per-client socket, not the listening socket.
- Server-side UDP acceptance rejects banned IPv4 clients with `CCRepReject` and the C-style reason string `You have been banned.\n` before any accepted per-client socket is created.
- Server-side UDP control packets also answer `CCReqRuleInfo` by enumerating the next `FlagServerInfo` cvar after the provided name and returning it as `CCRepRuleInfo`; an empty-name reply terminates enumeration.
- `Listen(state)` returns an error if enabling listen fails to open/bind the accept socket; on failure the transport stays non-listening with no accept socket retained.
- `SetHostPort` accepts only `[1, 65534]` and updates both current host port and default host port used by discovery/control-packet paths.
- Loopback reliable flow control is released when the receiver reads the message.
