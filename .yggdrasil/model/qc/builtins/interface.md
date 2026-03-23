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
- `random()` reads `sv_gameplayfix_random` to select formula parity:
  - `1` (default): `((rand()&0x7fff)+0.5)/0x8000` (open interval `(0,1)`).
  - `0` (legacy): `(rand()&0x7fff)/0x7fff` (closed interval `[0,1]`).
