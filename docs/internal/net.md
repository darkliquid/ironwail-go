# Package `net`

## Purpose
The `net` package serves as the foundational transport substrate for the Ironwail-Go engine. It implements Quake's classic networking model, allowing the client and server to exchange both reliable and unreliable messages. It abstracts the differences between local single-player communication (loopback) and remote multiplayer communication (UDP) behind a unified interface.

## Key Types & Interfaces
- **`Socket`**: The central connection abstraction (mirroring `qsocket_t` in C). it holds the state for a single connection, including sequence numbers for the reliability protocol, message buffers, and driver-specific data (like UDP addresses or loopback peer pointers).
- **`Network`**: A higher-level manager that handles global networking state, such as the listening port, active connections, and the cvar system. It acts as a dispatcher for operations like `Connect`, `SendMessage`, and `GetMessage`.
- **`ServerInfoProvider`**: A callback interface used to provide live server status (hostname, map name, player count) to respond to LAN browser queries.

## Core Workflow
1. **Initialization**: The `Network.Init()` method sets up the underlying transport drivers (typically UDP).
2. **Connection**:
   - For loopback (local), `Connect("local")` creates a direct in-memory link between a client and server socket.
   - For remote, `DatagramConnect` initiates a UDP handshake with a remote host.
3. **Messaging**:
   - **Reliable**: Uses a stop-and-wait ARQ (Automatic Repeat Request) protocol. Payloads larger than 1400 bytes are fragmented and reassembled.
   - **Unreliable**: Fire-and-forget packets used for frequently updated state (like entity positions) where losing a packet is preferable to waiting for a retransmission.
4. **Dispatch**: The methods in `net.go` check the `driver` field of a `Socket` and route the call to either `loopback.go` or `datagram.go`.

## Integration
- **Client/Server**: Both the client and server use `net.Socket` to communicate. The server listens for new connections via `Network.Listen`, while the client initiates them via `Network.Connect`.
- **Host**: The `host` package manages the lifecycle of the `Network` instance.
- **Protocol**: `protocol.go` defines the wire-format constants (like `svc_*` and `clc_*` commands) that are packed into messages sent via this package.

## Learning Tips
- **Reliability Layer**: Examine `internal/net/datagram.go` to see how Quake implements reliability over raw UDP using sequence numbers and acknowledgments.
- **Dispatcher Pattern**: Look at `internal/net/net.go` to understand how a single "Socket" type can seamlessly switch between in-memory loopback and network UDP.
- **Packet Headers**: Check `internal/net/types.go` for the bitwise flags used to identify packet types in the 8-byte header.
