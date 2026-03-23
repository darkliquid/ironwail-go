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

## Contracts

- Player slots are reserved edicts and must remain stable across connect/disconnect flows.
- Signon stage transitions must align with the message-buffer flush protocol expected by clients.
- QC hooks such as `SetNewParms`, `PutClientInServer`, and `ClientDisconnect` are authoritative extension points and must be synchronized with Go state updates.
