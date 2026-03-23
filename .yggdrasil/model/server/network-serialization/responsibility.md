# Responsibility

## Purpose

`server/network-serialization` owns the wire-format side of the authoritative server: signon payloads, unreliable datagrams, reliable per-client updates, and entity/effect serialization.

## Owns

- message-buffer helpers and protocol-aware primitive writing
- serverinfo and signon buffer serialization
- transient datagram events such as sounds and particles
- entity/static-state/stat update encoding for connected clients

## Does not own

- Client session state transitions themselves.
- Collision/physics/gameplay authority.
- Savegame persistence.
