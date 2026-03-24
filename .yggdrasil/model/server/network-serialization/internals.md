# Internals

## Logic

This node turns authoritative server state into Quake/FitzQuake/RMQ wire messages. It maintains signon buffers for initial world state, shared unreliable datagrams for transient events, and per-client reliable payloads for serverinfo, stats, and entity updates. Encoding paths must select the correct extended or legacy representation based on protocol limits. Non-entity intermission stats are refreshed from QuakeC globals before serialization so spawn packets and later reliable `svc_updatestat` messages match C Ironwail's `host_cmd.c` behavior. Final datagram assembly now also records packet-size dev stats on the server (`recordDevStatsPacketSize`) so host-side diagnostics can report current/peak packet bytes with C-like thresholds.

## Constraints

- Entity/model/sound index width and coordinate encoding vary by protocol and must stay aligned with client expectations.
- Omission/update rules for packet entities are parity-sensitive because clients infer lifecycle from those deltas.
- FitzQuake interpolation timing is split across physics and serialization: physics decides whether an entity's think cadence is non-default (`SendInterval`), while serialization writes the remaining `nextthink` byte only when that flag is set.
- Message buffers enforce MTU/signon size ceilings and may intentionally skip non-essential updates when capacity is exhausted.

## Decisions

### Centralized protocol-gated serialization layer

Observed decision:
- The package keeps protocol branching close to message emission instead of scattering it across gameplay code.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Protocol compatibility is easier to audit and test, even though the serializer remains tightly coupled to authoritative server state.

### Active-weapon overflow compatibility

Observed decision:
- `WriteClientDataToMessage` mirrors C Ironwail's Alkaline workaround: it still writes the legacy one-byte active-weapon field inside `svc_clientdata`, then appends a reliable `svc_updatestat` for `STAT_ACTIVEWEAPON` when the QuakeC weapon bitmask exceeds `0xff`.

Rationale:
- The base `svc_clientdata` layout only has one byte for the active weapon, but some mods use higher `IT_*` bits (for example Alkaline weapons at `1<<8` and above). Re-sending the full 32-bit stat preserves the exact weapon bitmask without changing the packet schema.

Observed effect:
- NetQuake/Fitz-style HUD updates stay wire-compatible for normal weapons while still matching Ironwail's compatibility behavior for large active-weapon values.
