# Internals

## Logic

`ServerBrowser` runs a background LAN search goroutine. It broadcasts `CCReqServerInfo`, retries once after 750ms (matching C `Slist_Send` reschedule at +0.75s), collects `CCRepServerInfo` responses for roughly 1.5 seconds, parses address/hostname/map/player fields, and deduplicates entries by address before exposing a sorted snapshot. Result ordering now mirrors C `NET_SlistSort` exactly: nested loops compare names with `strcmp < 0` semantics and swap in place, rather than delegating to Go’s `sort.Slice`. `QueryServerRules` reuses the same control-packet framing for the C `test2` path, but runs synchronously: it resolves a target address, sends `CCReqRuleInfo` with the last returned name, and keeps appending decoded `RuleInfoEntry` values until the server replies with an empty-name terminator. `QueryServerPlayers` follows the same synchronous control-query style for `players`: it requests indexed `CCReqPlayerInfo` rows, decodes slot/name/colors/frags/ping from `CCRepPlayerInfo`, and stops when the server returns an empty-name row. `AsyncReceiver` is a lightweight background poller that repeatedly calls a provided poll function, sleeps on empty polls, copies message payloads, and delivers them through a buffered channel. `IPBan` implements a single active IPv4 address/mask ban matching Quake's server-side ban behavior and is surfaced through package-level helpers so both host commands and the datagram accept path share one configured ban. `PartialIPAddress` recreates abbreviated Quake address entry by overlaying missing octets from the local IPv4 address and applying a default port unless one is explicitly provided.

## Constraints

- `ServerBrowser` is concurrent and protects mutable state with a mutex.
- Discovery parsing trusts the shared control-packet format and falls back to packet-source address when the embedded address is placeholder-like.
- Player-info parsing is strict about control framing and expected payload width so malformed replies fail fast instead of producing partial rows.
- `AsyncReceiver` preserves message ownership by copying bytes, which is part of its contract rather than an optimization detail.
- `IPBan` is IPv4-only and models one configured ban rather than a general list.
- Partial-address parsing is right-aligned: fewer octets replace the tail of the local IPv4 address.
- Audit note (C parity): current Go `PartialIPAddress` matches the broad right-aligned merge behavior used by `UDPStringToAddr`, but it is more strict/Go-idiomatic than C token parsing in several edge cases (`atoi`-style permissive port parsing, 3-digit-per-octet run limit, and consecutive-dot handling). These differences are currently observable only when parser edge cases are reached.

## Decisions

### Keep discovery and utility features adjacent to the transport package instead of pushing them into callers

Observed decision:
- The package includes browser, async-polling, banning, and partial-address helpers as first-class networking support utilities.

Rationale:
- **unknown — inferred from code and Quake lineage, not confirmed by a developer**

Observed effect:
- Callers can stay thin and reuse Quake-compatible helper behavior, but the package includes several support concepts that are related to networking without participating directly in the core transport state machine.

### Keep partial-IP parser tightening scoped to tokenization parity tests first

Observed decision:
- The next parity increment should target only `PartialIPAddress` tokenization/validation edges proven against C (`net_udp.c:PartialIPAddress`) and should not expand host command or transport architecture scope.

Rationale:
- This seam is the smallest change that can remove known C/Go parser drift while preserving the existing `UDPStringToAddr` call path and command surfaces (`connect`, `test2`, `players`).

Observed effect:
- Work can stay focused on parser behavior plus focused `internal/net` tests, avoiding overbuilding in unrelated networking or host-command code.
