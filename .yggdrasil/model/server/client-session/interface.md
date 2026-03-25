# Interface

## Main consumers

- `host/session` and host command flows that create local/remote server sessions
- `server/player-movement` and `server/physics-core`, which act on ingested client intent
- tests that verify signon, spawn, reconnect, and disconnect behavior

## Main surface

- client admission and connect/disconnect paths
- signon progression and serverinfo delivery
- user command and client string-command handling
- spawnparm preservation and respawn entry points
- spawn handshake QC sequencing (`ClientConnect` before `PutClientInServer`) and unknown-command fallback via `SV_ParseClientCommand`

## Contracts

- Player slots are reserved edicts and must remain stable across connect/disconnect flows.
- Client admission must reject new sockets when all server slots are occupied without aborting the frame loop.
- Signon stage transitions must align with the message-buffer flush protocol expected by clients.
- QC hooks such as `SetNewParms`, `PutClientInServer`, and `ClientDisconnect` are authoritative extension points and must be synchronized with Go state updates.
- Drop paths must preserve the reserved player edict, send the final disconnect packet when possible, and broadcast blank name/frags/colors roster clears to all peers.
- Spawnparm save/restore must round-trip the QC `serverflags` global and keep newly connected clients' scoreboard deltas dirty by seeding `OldFrags = -999999`.
- Client `ban` string-commands are accepted through the same whitelist/parsing path as C (`q_strncasecmp(..., "ban", 3)` parity), mutate the shared datagram IP ban state used by connection admission, print status/errors back to that client, and are ignored while `deathmatch != 0`.
