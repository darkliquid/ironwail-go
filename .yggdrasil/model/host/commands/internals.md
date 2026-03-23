# Internals

## Logic

### Registration

`RegisterCommands` is the entry point that binds host policies into the command system during startup.

Registration refreshes existing host-owned command names before re-adding them. This keeps command closures bound to the current `Host`/`Subsystems` pair across repeated init/test lifecycles instead of silently retaining the first registration forever.

### Command families

- **Map/session commands** manage local server startup, reconnect-style transitions, and changelevel/load flows.
- **Network commands** create, reset, or tear down remote datagram client sessions.
- **Gameplay/save commands** manage native saves, imported saves, and load validation.
- **Demo commands** coordinate record, playback, seek, and timedemo state.
- **System/config commands** rebuild startup commands from argv and execute config text from builtin, user, or filesystem sources.
- **Forwarding commands** decide whether a command should remain local or be sent to a remote server.

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
