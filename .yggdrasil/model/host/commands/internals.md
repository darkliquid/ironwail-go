# Internals

## Logic

### Registration

`RegisterCommands` is the entry point that binds host policies into the command system during startup.

Registration refreshes existing host-owned command names before re-adding them. This keeps command closures bound to the current `Host`/`Subsystems` pair across repeated init/test lifecycles instead of silently retaining the first registration forever.

### Command families

- **Map/session commands** manage local server startup, reconnect-style transitions, and changelevel/load flows.
- **Network commands** create, reset, or tear down remote datagram client sessions.
- **Network admin commands** expose C-like `listen`/`maxplayers`/`port` query-set behavior and queue command-buffer listen transitions (`listen 0`/`listen 1`) when max-player mode or host port changes require rebinding. `ban` now also mirrors C's transport-owned IP/mask surface instead of maintaining a host-local player-name blacklist. `net_stats` surfaces the existing package-level datagram counters through the host console without adding a separate host-owned stats cache. `test2` now uses the discovery-support rule-query helper to print serverinfo cvars from a remote host.
- **Gameplay/save commands** manage native saves, imported saves, and load validation.
- **Gameplay/save commands** also align host autosave timestamps to restored server time after successful load restores.
- **Gameplay/client-state commands** include local view/render helpers such as `particle_texture` and now `fog`, which unwrap the concrete loopback/remote client wrappers to reach the shared client runtime state without widening the public host client interface.
- **Gameplay/server-debug commands** now include `edictcount`, a read-only summary over `server.Server`'s existing `NumEdicts`/`Edicts` surface that mirrors C's quick entity-population counts without pulling in the broader QC-aware edict pretty-printer machinery.
- **Gameplay/profile command** exposes QC VM per-function profile counters via `profile` (top 10), using C-like `%7d %s` line formatting and no output when no active local server/QC VM exists.
- **Demo commands** coordinate record, playback, seek, and timedemo state.
- **System/config commands** rebuild startup commands from argv and execute config text from builtin, user, or filesystem sources.
- **Forwarding commands** decide whether a command should remain local or be sent to a remote server; that decision keys off active local-session state, not mere availability of a server subsystem instance.
- **Explicit `cmd` forwarding** bypasses that local-session ownership check and mirrors C Ironwail's dedicated `cmd` command: local console only, current connection only, silent during demo playback, and payload reconstruction that drops the leading `cmd` token before sending the remainder.

## Constraints

- Command behavior depends on command source; server-sent text is more restricted than local input.
- Some paths require concrete server/filesystem implementations even though host otherwise programs to interfaces.
- Map/load/changelevel behavior is parity-sensitive because it touches both host policy and client/server transition state.

## Decisions

### Explicit command families instead of one monolithic file

Observed decision:
- The Go port splits host commands into multiple files by concern.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- related policies are grouped by domain
- tests can target command families more precisely
