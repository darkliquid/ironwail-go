# Interface

## Main consumers

- initialization code that wants Quake/Ironwail-style cvar helper commands available through the console

## Main surface

- `(*CmdSystem).RegisterCvarCommands()`
- the registered command verbs: `cvarlist`, `toggle`, `cycle`, `cycleback`, `inc`, `reset`, `resetall`, `resetcfg`

## Contracts

- These commands delegate to the `cvar` package for all storage and default-value behavior.
- `cycle`/`cycleback` only operate on existing cvars and match numeric list entries by parsed numeric value before falling back to string equality.
- `cvarlist` emits a deterministic alphabetical listing based on cvar names, supports an optional prefix filter, prints `*` for archived cvars and `s` for notify cvars, and ends with the standard summary line.
- `resetcfg` only targets archived cvars.
- `inc` only operates on existing cvars; it must not implicitly create a new variable.
- Logging/usage output is part of the operator-facing console behavior.
