# Responsibility

## Purpose

`host/commands` owns the host-level command surface exposed to scripts, config files, console input, and network/session control.

## Owns

- Registration of host-facing commands into the command system.
- Map and local session commands.
- Runtime game-directory switching command seam (`game`) that validates and swaps active VFS mounts.
- Connect/disconnect/reconnect and remote-session commands.
- Demo record/playback/timedemo commands.
- Save/load command handlers and save path validation.
- System/config commands such as config execution and command-line `stuffcmds`.
- Host-level forwarding policy that decides when commands are executed locally versus forwarded to a connected server.

## Does not own

- Tokenization, alias expansion, or command buffer mechanics; those belong to the command system.
- Server, client, or filesystem internals beyond invoking their public behavior.
