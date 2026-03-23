# Responsibility

## Purpose

`server/client-session` owns the lifetime of connected players inside the authoritative server: slot admission, signon, command ingestion, spawn/respawn, and disconnect.

## Owns

- accepting/allocating client slots
- binding clients to reserved player edicts
- signon stage progression and signon-buffer flushing
- user command ingestion and string-command handling
- disconnect, respawn, and spawnparm preservation behavior

## Does not own

- Wire-format message encoding details.
- Generic movement/collision implementations.
- Persistent savegame snapshot encoding.
