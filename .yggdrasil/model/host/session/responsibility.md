# Responsibility

## Purpose

`host/session` owns session-adjacent support that is neither core runtime scheduling nor top-level command registration: remote datagram client behavior and host-side autosave heuristics.

## Owns

- The `remoteDatagramClient` adapter that wraps a concrete client/parser pair and a network socket.
- Automatic signon reply sequencing for remote sessions.
- Remote session reset and shutdown behavior.
- Autosave timing and safety heuristics driven by host time, server time, and player state.

## Does not own

- Core host frame cadence or subsystem initialization.
- The console command entry points that invoke save/load/network flows.
- Low-level network transport, client parsing, or server simulation internals.
