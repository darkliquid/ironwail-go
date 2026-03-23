# Responsibility

## Purpose

`net` owns the engine's networking substrate: transport/runtime socket behavior, Quake wire-protocol constants, and supporting discovery/addressing utilities used by host, client, server, and menu code.

## Owns

- The package-level split between runtime transport/state-machine code, protocol definitions, and LAN/discovery/support helpers.
- The common networking vocabulary shared by host/client/server code.
- Quake-style transport semantics for local loopback and UDP datagram communication.

## Does not own

- Higher-level client parsing or server message construction.
- Gameplay/session policy above connect/listen/send/receive/server-discovery primitives.
- Renderer or menu behavior beyond consuming the networking APIs.
