# Responsibility

## Purpose

`net/transport` owns the active networking runtime: the public networking facade, driver-agnostic socket state, loopback transport, UDP datagram transport, handshake/control packets, and connection-level reliability behavior.

## Owns

- Public facade functions in `net.go`.
- Shared socket state and packet flag/size constants in `types.go`.
- UDP socket wrappers and environment-derived local address state in `udp.go`.
- Datagram reliable/unreliable send/receive, retransmit, fragmentation, handshake, and server-info response handling in `datagram.go`.
- Loopback paired-socket behavior and buffer helpers in `loopback.go`.
- Global transport diagnostics counters in `stats.go`.
- Transport-focused tests covering connect/listen/send/receive/shutdown and accepted-socket behavior.

## Does not own

- The higher-level semantics of server/client protocol message payloads.
- LAN browser orchestration beyond answering server-info requests.
