# Interface

## Main consumers

- `server/client-session`, which triggers serverinfo/signon progression
- `server/physics-core`, which emits transient sounds/particles during simulation
- `host`/runtime paths that flush outgoing server messages to clients

## Main surface

- message buffer primitives
- signon buffer allocation/reservation helpers
- serverinfo, static world, sound, particle, stat, and entity-state serialization
- shared reliable-datagram fanout and protocol-specific clientdata encoding

## Contracts

- Protocol version/flag handling is part of the network ABI.
- Precache/model/sound indices must remain stable across spawn, updates, and restore flows.
- NetQuake serverinfo must cap model/sound precache enumeration at 255 entries, while Fitz/RMQ keep the wider tables.
- `MSG_ALL` writes are staged through the shared reliable datagram and only fanned out during `UpdateToReliableMessages`, matching the C reliable-send phase.
- Active-weapon serialization must use raw bitmasks for standard Quake and bit-number encoding for mission-pack game dirs (`rogue`, `hipnotic`, `quoth`).
- Datagram limits are enforced defensively; overflow-sensitive events may be dropped instead of corrupting packets.
- Datagram assembly updates server dev-stats packet counters from final datagram size (`msg.Len()`), including a one-time warning when the packet first exceeds the classic 1024-byte threshold.
- Secret/monster intermission counters are serialized from QuakeC globals (`total_secrets`, `total_monsters`, `found_secrets`, `killed_monsters`) into the corresponding `STAT_*` slots for both signon and later reliable stat updates.
- FitzQuake `U_LERPFINISH` is emitted only when physics marked the entity's `SendInterval` flag; the payload byte then encodes the remaining `nextthink - sv.time` interval for the client.
