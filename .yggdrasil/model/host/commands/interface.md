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
- explicit `rcon` forwarding for remote admin command payloads
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
- The explicit `rcon` command forwards `rcon <payload>` to the current connection when forwarding policy allows, prints `usage: rcon <command>` for missing payload, and otherwise inherits standard forwarding failure text (`Can't "rcon", not connected`).
- Save/load commands enforce strict save-name and path safety rules.
- Local map/changelevel/load commands may synchronously mutate both host and loopback client state.
- Successful load operations synchronize autosave timing with restored server time so autosave scoring does not fire immediately after a save restore.
- `profile` is local-server/QC-only: it no-ops when no active local server/QC VM is available, prints at most 10 lines in `%7d %s` format, and consumes/reset VM counters.
- `devstats` is local-server only: it prints an 8-row C-style `Curr/Peak` table (`Edicts`, `Packet`, `Visedicts`, `Efrags`, `Dlights`, `Beams`, `Tempents`, `GL upload`) sourced from server-side counters; fields not currently produced by server/runtime remain zero.
- `listen` mirrors C query/set semantics: no argument prints `"listen" is "0|1"`, one numeric argument toggles transport listening.
- `maxplayers` mirrors C query/set semantics: no argument prints current max, setting is rejected while server is active, values clamp to `[1, MaxScoreboard]`, `deathmatch` tracks `maxplayers>1`, and listen toggles are queued (`listen 0`/`listen 1`) when crossing single-player/multiplayer boundary.
- `port` mirrors C query/set semantics: no argument prints current host port, set validates `[1, 65534]`, and when already listening it queues `listen 0` then `listen 1` to rebind.
- `ban` mirrors C transport/IP semantics rather than name moderation: local-server invocation prints current status with no args, accepts `ban off`, accepts `ban <ip>` or `ban <ip> <mask>`, prints `BAN ip_address [mask]` for invalid arity, and forwards to the remote server when no local server owns the command path.
- `net_stats` exposes the global datagram transport counters maintained by `internal/net/stats.go`, using the same line labels as C's no-argument `NET_Stats_f` branch.
- `test2 <server>` mirrors C's rule-info query path by printing the target server's `FlagServerInfo` cvars in `%-16.16s  %-16.16s` rows.
- `players <server>` mirrors C's player-info query path by printing queried slot/name/colors/frags/ping rows sourced from `CCReqPlayerInfo`/`CCRepPlayerInfo`.
- `map` mirrors C query behavior when called without args: in dedicated mode it prints current map if active, in non-dedicated mode it prints map/help depending on connection state, and map names normalize trailing `.bsp`.
- `god`/`noclip`/`fly`/`notarget` mirror C arity semantics: no arg toggles, one numeric arg explicitly sets 0/1, other arities print usage.
- `pause` mirrors C local behavior by toggling demo pause during playback, honoring `pausable` when running a server, and printing pause/unpause status.
- `record` mirrors C forms `record <demo> [<map> [cdtrack]]`, preserves full argument vector when routing to host command handler, stops prior recording before re-recording, and validates usage arity.
- `slist` mirrors C's LAN-browser console contract: prints `Looking for Quake servers...`, then `Server/Map/Users` header rows, then one row per host (`%-15.15s %-15.15s %2u/%2u` when max users is known, otherwise name/map only), ending with `== end list ==` or `No Quake servers found.`.
- `fog` mirrors C's client-side query/set forms: no args prints usage plus current values, one arg sets density, two args set density plus fade time, three args set RGB, four args set density+RGB, and five args set density+RGB+fade time while clamping density/RGB ranges.
- `edictcount` mirrors C's local-server debug summary by printing `num_edicts`, `active`, `view`, `touch`, and `step` counts derived from the current server entity array, and also exposes parity-facing physics counters (`peak`, `c_yes`, `c_no`) sourced from server physics/movement debug stats.
- `path` mirrors C's filesystem debug command by printing `Current search path:` followed by the active VFS lookup stack, with pack entries shown as `path (N files)`.
- `mapname` mirrors C's query semantics by printing `"mapname" is "<name>"` for the active local server map or the connected client map, and `no map loaded` otherwise.
- `mods` mirrors C's mod-directory listing by printing discovered non-`id1` mod directories plus a count footer, optionally filtering by substring; `games` is an exact alias to the same listing.
- `game` now provides the runtime mod-switch seam used by menu mods-browser selection: no args prints current gamedir, one arg validates/switches filesystem mount to the selected gamedir (`id1` or discovered mods), runs host runtime's optional game-dir-changed callback with the freshly mounted `*fs.FileSystem` (used by executable wiring to reload draw assets and renderer palette/conchars), and invalid names print usage/error text without mutating the active VFS.
- `skies` mirrors C's skybox listing by printing discovered `gfx/env/*up.tga` skybox bases plus a count footer, optionally filtering by substring.
- `demoseek`/`demogoto`/`rewind` rebuild playback state from frame 0 and must clear any rewind-edge backstop flag as part of seeking so explicit seeks do not remain stuck in the "first-frame rewind clamp" state.
- `playdemo` prefers a filesystem-provided `OpenFile` seam when available so PAK-aware playback can start from a seekable VFS handle directly, falling back to byte-slice loading and finally loose OS-file playback for older/mock filesystem implementations.
- `stopdemo` must emit timedemo summary output on timedemo sessions by using demo stop-with-summary helpers, not only on EOF playback path.

## Failure modes

- Invalid save names, unsafe paths, wrong active game directories, or missing save files are rejected.
- Network commands surface transport/setup failures.
- Demo commands reject incompatible or unsupported states.
