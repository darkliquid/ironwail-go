# Interface

## Main consumers

- `server/client-session`, which triggers serverinfo/signon progression
- `server/physics-core`, which emits transient sounds/particles during simulation
- `host`/runtime paths that flush outgoing server messages to clients

## Main surface

- message buffer primitives
- signon buffer allocation/reservation helpers
- serverinfo, static world, sound, particle, stat, and entity-state serialization

## Contracts

- Protocol version/flag handling is part of the network ABI.
- Precache/model/sound indices must remain stable across spawn, updates, and restore flows.
- Datagram limits are enforced defensively; overflow-sensitive events may be dropped instead of corrupting packets.
- Datagram assembly updates server dev-stats packet counters from final datagram size (`msg.Len()`), including a one-time warning when the packet first exceeds the classic 1024-byte threshold.
- Secret/monster intermission counters are serialized from QuakeC globals (`total_secrets`, `total_monsters`, `found_secrets`, `killed_monsters`) into the corresponding `STAT_*` slots for both signon and later reliable stat updates.
- FitzQuake `U_LERPFINISH` is emitted only when physics marked the entity's `SendInterval` flag; the payload byte then encodes the remaining `nextthink - sv.time` interval for the client.
