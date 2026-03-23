# Interface

## Main consumers

- `qc/core`, which dispatches negative `FirstStatement` calls into registered builtins
- `internal/server`, which provides the concrete hook implementations

## Main API

Observed surfaces:
- `RegisterBuiltins(vm *VM)`
- hook registration for server-backed builtin behavior
- builtin handler functions grouped by concern

## Contracts

- Builtin numbers must remain aligned with Quake expectations.
- Server-facing builtins depend on hooks being registered before use.
- Builtins read/write VM globals and edict fields directly through VM helpers.
