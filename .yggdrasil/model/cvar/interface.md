# Interface

## Main consumers

- `cmdsys`, which falls back to cvar print/set behavior and installs cvar helper commands.
- `host` and other subsystems that register persistent configuration and later serialize archived vars.
- `console`, which consumes cvar-name completion through injected providers.
- `qc`, which reads and writes cvars through builtins.

## Main surface

- `CVar`, `CVarSystem`, and `CVarFlags`
- `Register`, `Get`, `Set`, `SetFloat`, `SetInt`, `SetBool`
- `FloatValue`, `IntValue`, `BoolValue`, `StringValue`
- `All`, `ArchiveVars`, `Complete`
- `LockVar`, `UnlockVar`, `SetAutoCvarCallback`, `MarkAutoCvar`
- `(*CVarSystem).MarkAutoCvar` for non-global registries (tests and isolated runtimes)
- package-global wrappers over the singleton registry

## Contracts

- Cvar names are canonicalized to lowercase.
- The canonical stored representation is the string value; numeric caches are derived from it.
- Callbacks run after successful non-latched updates and after the registry lock is released.
- `AutoCvarChanged` only runs on successful non-latched, non-locked updates of cvars marked `FlagAutoCvar`.
- Marking a cvar as auto-cvar is supported both globally and per-registry (`MarkAutoCvar` variants).
- `ArchiveVars` only includes `FlagArchive` vars and returns deterministic sorted output.
