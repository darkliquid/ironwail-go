# Responsibility

## Purpose

`net/discovery-support` owns the package features that sit beside the core transport state machine: LAN server discovery, asynchronous socket polling, IP banning, and partial-address expansion.

## Owns

- LAN server browser search timing, response parsing, deduplication, and result exposure.
- `HostCacheEntry` formatting.
- `AsyncReceiver` background polling and message ownership semantics.
- `IPBan` configuration and subnet-based match behavior.
- `PartialIPAddress` expansion of abbreviated host strings.
- Tests for LAN browser parsing/state, async polling, bans, and partial-IP rules.

## Does not own

- Core socket connect/listen/send/receive behavior.
- The shared wire constant catalog in `protocol.go`.
