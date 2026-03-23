# Internals

## Logic

`ServerBrowser` runs a background LAN search goroutine. It broadcasts `CCReqServerInfo`, retries once after 500ms, collects `CCRepServerInfo` responses for roughly 1.5 seconds, parses address/hostname/map/player fields, and deduplicates entries by address before exposing a sorted snapshot. `AsyncReceiver` is a lightweight background poller that repeatedly calls a provided poll function, sleeps on empty polls, copies message payloads, and delivers them through a buffered channel. `IPBan` implements a single active IPv4 address/mask ban matching Quake's server-side ban behavior. `PartialIPAddress` recreates abbreviated Quake address entry by overlaying missing octets from the local IPv4 address and applying a default port unless one is explicitly provided.

## Constraints

- `ServerBrowser` is concurrent and protects mutable state with a mutex.
- Discovery parsing trusts the shared control-packet format and falls back to packet-source address when the embedded address is placeholder-like.
- `AsyncReceiver` preserves message ownership by copying bytes, which is part of its contract rather than an optimization detail.
- `IPBan` is IPv4-only and models one configured ban rather than a general list.
- Partial-address parsing is right-aligned: fewer octets replace the tail of the local IPv4 address.

## Decisions

### Keep discovery and utility features adjacent to the transport package instead of pushing them into callers

Observed decision:
- The package includes browser, async-polling, banning, and partial-address helpers as first-class networking support utilities.

Rationale:
- **unknown — inferred from code and Quake lineage, not confirmed by a developer**

Observed effect:
- Callers can stay thin and reuse Quake-compatible helper behavior, but the package includes several support concepts that are related to networking without participating directly in the core transport state machine.
