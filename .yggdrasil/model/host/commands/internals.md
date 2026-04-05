# Internals

## Logic

### Registration

`RegisterCommands` is the entry point that binds host policies into the command system during startup.

Registration refreshes existing host-owned command names before re-adding them. This keeps command closures bound to the current `Host`/`Subsystems` pair across repeated init/test lifecycles instead of silently retaining the first registration forever.

### Command families

- **Map/session commands** manage local server startup, reconnect-style transitions, and changelevel/load flows.
- **Menu commands** now include a broad C-style `menu_*` family that maps directly to menu states (`menu_main`, `menu_singleplayer`, `menu_maps`, `menu_load`, `menu_save`, `menu_multiplayer`, `menu_setup`, `menu_options`, `menu_keys`, `menu_video`, `menu_help`, `menu_quit`).
- **Map/session command queries** include `mapname`, which now reports the real active map from `Server.GetMapName()` for local sessions or `ActiveClientState(subs).MapName` for connected clients instead of placeholder text.
- **Map/session discovery commands** now include `mods`/`games`, which type-assert `subs.Files` to `*fs.FileSystem`, reuse `ListMods()`, and mirror C's list-or-filter count summaries without introducing a separate mod registry in host state.
- **Game-directory switch seam** now includes `game`, which uses the same concrete `*fs.FileSystem` seam to query current gamedir, validate requested targets against `id1` plus `ListMods()`, rebuild a fresh VFS mount for the selected gamedir, atomically swap `subs.Files`, invoke the host runtime optional game-dir-changed callback (with the same `subs` pointer and swapped filesystem) so executable wiring can reload mod-scoped runtime state from the new mount, update host/menu current-mod state, and leave existing mounts untouched on validation/init failure. The previous filesystem now remains open until the callback returns, so executable reload code can tear down stale consumers safely before the old PAK handles are released.
- **Random map selection** now mirrors the C command-buffer flow instead of requiring a live server: `randmap` enumerates mounted BSP names from the active `*fs.FileSystem`, chooses one, prints the selected map, and queues `map <name>\n` through `Subsystems.Commands.AddText(...)` so the transition runs through normal command execution.
- That same discovery family also includes `skies`, which reuses `ListFiles("gfx/env/*up.tga")`, lowercases/deduplicates skybox base names, and mirrors the same count/filter footer contract over discovered external sky assets.
- **Network commands** create, reset, or tear down remote datagram client sessions.
- **Network admin commands** expose C-like `listen`/`maxplayers`/`port` query-set behavior and queue command-buffer listen transitions (`listen 0`/`listen 1`) when max-player mode or host port changes require rebinding. `ban` now also mirrors C's transport-owned IP/mask surface instead of maintaining a host-local player-name blacklist. `net_stats` surfaces the existing package-level datagram counters through the host console without adding a separate host-owned stats cache. `test2` now uses the discovery-support rule-query helper to print serverinfo cvars from a remote host. `players` uses the discovery-support player-query helper to print remote slot/name/colors/frags/ping rows. `slist` now mirrors C's print contract (banner, `Server/Map/Users` table, and `== end list ==` / `No Quake servers found.` trailer) while continuing to reuse the Go server-browser implementation for discovery.
- **Gameplay/save commands** manage native saves, imported saves, and load validation.
- **Gameplay/save menu gating policy** also exposes `SaveEntryAllowed(subs)` so menu flow can mirror C parity preconditions (local active single-player only, no intermission) before entering the Save menu.
- **Gameplay/save commands** also align host autosave timestamps to restored server time after successful load restores.
- **Gameplay/client-state commands** include local view/render helpers such as `particle_texture` and now `fog`, which unwrap the concrete loopback/remote client wrappers to reach the shared client runtime state without widening the public host client interface.
- **Gameplay/server-debug commands** now include `edictcount`, a read-only summary over `server.Server`'s existing `NumEdicts`/`Edicts` surface that mirrors C's quick entity-population counts without pulling in the broader QC-aware edict pretty-printer machinery. The same output also wires through physics/movement parity counters (`peak` from `Physics` dev stats, `c_yes`/`c_no` from `CheckBottom` counters) so the command exposes the same class of runtime diagnostics that C tracks for `dev_stats`/`sv_move` debugging.
- **System/filesystem debug commands** now include `path`, which type-asserts `subs.Files` to the concrete `*fs.FileSystem` and prints the exported `SearchPathEntries` snapshot instead of reaching into private VFS internals directly.
- **Gameplay/profile and dev-stats commands** expose QC VM counters via `profile` (top 10, `%7d %s`) and C-style runtime dev counters via `devstats` (`Curr/Peak` table mirroring the C overlay labels). `devstats` consumes a narrow server bridge (`DevStatsSnapshot`) and currently reports server-owned counters (`Edicts`, `Packet`) while leaving renderer/client-owned rows as zero.
- **Demo commands** coordinate record, playback, seek, and timedemo state. `playdemo` now first probes an optional filesystem `OpenFile` seam so PAK-aware playback can keep a seekable VFS handle open (closer to `COM_FOpenFile` behavior) before falling back to byte-slice loading or loose OS-file playback. Seek-based flows (`demoseek`, `demogoto`, `rewind`) call a shared replay-from-zero path (`seekDemoFrame`) that clears the demo rewind-backstop latch before replaying frames into parser state, so an earlier negative-speed rewind edge does not leak into subsequent explicit seeks. `stopdemo` now uses the demo stop-with-summary helper so timedemo benchmark output is emitted consistently on explicit command stop, matching C parity with EOF teardown. The shared disconnect teardown path (`disconnectCurrentSession`) also routes demo stop through the same helper so manual disconnect/reconnect shutdown emits timedemo summary before playback state reset.
- **System/config commands** rebuild startup commands from argv and execute config text from builtin, user, or filesystem sources.
- **Forwarding commands** decide whether a command should remain local or be sent to a remote server; that decision keys off active local-session state, not mere availability of a server subsystem instance.
- **Explicit `cmd` forwarding** bypasses that local-session ownership check and mirrors C Ironwail's dedicated `cmd` command: local console only, current connection only, silent during demo playback, and payload reconstruction that drops the leading `cmd` token before sending the remainder.
- **Explicit `rcon` forwarding** reuses the same forwarding gate but keeps `rcon` prefixed in the forwarded payload so remote servers can parse it as remote-admin input. The host command adds only a minimal UX layer (`usage: rcon <command>` when empty args) and otherwise defers transport/connectivity behavior to the existing forwarding path.

## Constraints

- Command behavior depends on command source; server-sent text is more restricted than local input.
- Some paths require concrete server/filesystem implementations even though host otherwise programs to interfaces.
- Map/load/changelevel behavior is parity-sensitive because it touches both host policy and client/server transition state.
- `menu_quit` intentionally preserves `ShowQuitPrompt` callback/populated-line behavior instead of using raw state selection.

## Decisions

### Explicit command families instead of one monolithic file

Observed decision:
- The Go port splits host commands into multiple files by concern.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- related policies are grouped by domain
- tests can target command families more precisely

### Keep QC profiling as a host command, not telemetry output

Observed decision:
- QC profiling remains exposed via the `profile` host command instead of being folded into the server telemetry stream.

Rationale:
- `profile` already mirrors the C-style operational contract (top 10 rows, `%7d %s` formatting, local-server/QC-only behavior, counter reset after read), and telemetry has a different goal: event tracing for parity/debug workflows.

Observed effect:
- QC profiling is implemented and available for focused VM hot-spot checks without increasing telemetry noise.
