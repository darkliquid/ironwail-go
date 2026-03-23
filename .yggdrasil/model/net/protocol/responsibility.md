# Responsibility

## Purpose

`net/protocol` owns the shared wire-contract definitions used by the rest of the engine when encoding, decoding, and interpreting network traffic.

## Owns

- Protocol version constants (`PROTOCOL_NETQUAKE`, `PROTOCOL_FITZQUAKE`, `PROTOCOL_RMQ`).
- Message IDs (`SVC*`, `CLC*`, temporary entity events, sound flags, update flags, baseline flags, etc.).
- Alpha/entity encoding helpers and related protocol utility functions.
- Protocol-level enumerations and bit flags referenced by client/server parsing and send logic.

## Does not own

- Socket management or packet transport state.
- Parsing loops or serializer implementations outside the constant/helper definitions.
