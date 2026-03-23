# Internals

## Logic

`net.go` is the dispatcher: it exposes one facade and routes each socket operation based on `Socket.driver`. `types.go` holds the shared connection state for both loopback and datagram drivers, including sequence counters, ACK tracking, in-flight reliable buffers, and transport-specific fields like `peer`, `udpConn`, and `remoteAddr`.

`datagram.go` implements Quake's custom UDP reliability layer. Reliable sends copy the message into `sock.sendMessage`, send one fragment immediately, and clear `canSend`. ACK packets advance fragment state; once the final fragment is acknowledged, `canSend` becomes true again. Retransmission occurs after roughly one second without ACK. Incoming reliable data is ACKed immediately, accumulated until `FlagEOM`, and then returned as one completed message. Connection setup uses control packets: the client sends `CCReqConnect`, the server answers with `CCRepAccept` or `CCRepReject`, and accepted clients are switched onto a fresh per-client UDP socket whose port is returned in the accept reply. Reconnects from the same remote endpoint close stale accepted sockets before replacement.

`net.go` listen handling now reports accept-socket startup errors to callers. Enabling listen (`Listen(true)`) attempts to open/bind the UDP accept socket and returns an error on failure, leaving `listening=false` and `acceptSocket=nil` so callers do not proceed under a false "listening" state. Disabling listen (`Listen(false)`) closes and clears the accept socket and returns close errors if they occur.

`loopback.go` provides the in-process transport for local play and tests. It uses paired sockets, a simple packed in-memory message format, and Quake-style 4-byte alignment. Reliable loopback sends clear `canSend` on the sender and only restore it once the peer reads the message.

## Constraints

- Datagram reliability is intentionally a size-1 sliding window; there is never more than one reliable message/fragment in flight.
- Reliable datagram packets larger than `MaxDatagram` are fragmented, but the reassembled logical message may grow to `MaxMessage`.
- Server-info responses fall back to cvar/default hostname and packet-source IP when embedded address state is placeholder or missing.
- Accepted UDP sockets must be tracked so reconnects from the same remote endpoint can evict stale server-side sockets.
- Listen/open failures must be surfaced to callers so host startup and LAN-advertising policy can react instead of silently running with networking disabled.
- `Buffer.WriteFloat` appears incomplete/unrepresentative of a real IEEE wire-format helper and is not the core transport contract.

## Decisions

### Preserve Quake's transport semantics instead of replacing them with a higher-level network library

Observed decision:
- The package reimplements Quake's stop-and-wait datagram protocol, control-packet handshake, and loopback behavior directly on top of Go UDP and memory buffers.

Rationale:
- **unknown — inferred from code and comments, not confirmed by a developer**

Observed effect:
- The engine preserves classic Quake networking behavior closely, but transport correctness depends on subtle sequencing, ACK, and per-client-socket invariants that need explicit graph documentation.
