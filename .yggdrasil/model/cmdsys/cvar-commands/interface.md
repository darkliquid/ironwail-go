# Interface

## Main consumers

- initialization code that wants Quake/Ironwail-style cvar helper commands available through the console

## Main surface

- `(*CmdSystem).RegisterCvarCommands()`
- the registered command verbs: `toggle`, `cycle`, `inc`, `reset`, `resetall`, `resetcfg`

## Contracts

- These commands delegate to the `cvar` package for all storage and default-value behavior.
- `resetcfg` only targets archived cvars.
- Logging/usage output is part of the operator-facing console behavior.
