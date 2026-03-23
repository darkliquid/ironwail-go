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
