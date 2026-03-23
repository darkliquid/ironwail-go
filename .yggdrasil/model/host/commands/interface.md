# Interface

## Consumers

- Console input
- config scripts (`quake.rc`, config files, `stuffcmds`)
- runtime code that registers host commands during startup

## Main surface

The public surface is command-oriented rather than method-oriented. Observed command groups:
- map/session lifecycle
- remote networking and reconnect
- explicit `cmd` console forwarding to the currently connected server
- save/load
- listen-server admin commands (`listen`, `maxplayers`, `port`)
- QC profiling (`profile`) for top QuakeC function counters on an active local server
- demo playback and capture
- system/config execution
- audio-facing host helpers

## Contracts

- Host commands execute under command-source gating from `cmdsys`.
- Some commands intentionally forward to the remote server only when command source is local console input and no active local server session owns the command path; merely having an initialized server subsystem must not block remote forwarding.
- The explicit `cmd` command is the escape hatch that always targets the current connection, even when a local server is active, strips the leading `cmd` token from the forwarded payload, prints `Can't "cmd", not connected` when disconnected, and silently no-ops during demo playback.
- Save/load commands enforce strict save-name and path safety rules.
- Local map/changelevel/load commands may synchronously mutate both host and loopback client state.
- Successful load operations synchronize autosave timing with restored server time so autosave scoring does not fire immediately after a save restore.
- `profile` is local-server/QC-only: it no-ops when no active local server/QC VM is available, prints at most 10 lines in `%7d %s` format, and consumes/reset VM counters.
- `listen` mirrors C query/set semantics: no argument prints `"listen" is "0|1"`, one numeric argument toggles transport listening.
- `maxplayers` mirrors C query/set semantics: no argument prints current max, setting is rejected while server is active, values clamp to `[1, MaxScoreboard]`, `deathmatch` tracks `maxplayers>1`, and listen toggles are queued (`listen 0`/`listen 1`) when crossing single-player/multiplayer boundary.
- `port` mirrors C query/set semantics: no argument prints current host port, set validates `[1, 65534]`, and when already listening it queues `listen 0` then `listen 1` to rebind.
- `ban` mirrors C transport/IP semantics rather than name moderation: local-server invocation prints current status with no args, accepts `ban off`, accepts `ban <ip>` or `ban <ip> <mask>`, prints `BAN ip_address [mask]` for invalid arity, and forwards to the remote server when no local server owns the command path.
- `net_stats` exposes the global datagram transport counters maintained by `internal/net/stats.go`, using the same line labels as C's no-argument `NET_Stats_f` branch.
- `test2 <server>` mirrors C's rule-info query path by printing the target server's `FlagServerInfo` cvars in `%-16.16s  %-16.16s` rows.

## Failure modes

- Invalid save names, unsafe paths, wrong active game directories, or missing save files are rejected.
- Network commands surface transport/setup failures.
- Demo commands reject incompatible or unsupported states.
