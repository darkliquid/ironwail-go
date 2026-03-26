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
- `traceon()` and `traceoff()` must toggle the VM's `Trace` execution flag rather than silently no-op, because the interpreter already honors that flag when statement tracing is enabled.
- Trig builtins using C/Ironwail extension slots (`sin`, `cos`, `tan`, `asin`, `acos`, `atan`, `atan2`) follow raw C math semantics and therefore consume/return radians, not Quake angle degrees.
- `substring()` follows C's negative-index rules: negative `start` counts back from the end of the string, and negative `length` means "trim that many chars from the tail after `start`".
- `strcat()` and `strzone()` concatenate all provided QC string arguments in order rather than stopping at a fixed two-argument surface.
- `mod()` preserves C's divide-by-zero contract by returning `0` and printing `PF_mod: mod by zero` to the console for observability.
- `sound()` treats QuakeC volume as normalized `0..1` and forwards packet volume as `int(volume*255)` (defaulting to `255` when omitted/zero), while forwarding attenuation exactly (including `0`) so server-side sound serialization receives C-equivalent values.
- `random()` reads `sv_gameplayfix_random` to select formula parity:
  - `1` (default): `((rand()&0x7fff)+0.5)/0x8000` (open interval `(0,1)`).
  - `0` (legacy): `(rand()&0x7fff)/0x7fff` (closed interval `[0,1]`).
- RNG trace parity contract: with seed `1`, zero-offset gameplay-fix `random()` starts with
  `[0.54222107, 0.27949524, 0.1907196, 0.5660248, 0.7212372]`; if one upstream compat-rand
  draw is consumed first, QC `random()` starts at `[0.27949524, 0.1907196, 0.5660248, 0.7212372, 0.72654724]`.
  Legacy (`sv_gameplayfix_random=0`) follows the same one-draw shift with legacy-formula values.
- builtin `#28` (`coredump`) is wired to entity dump behavior (matching C slot mapping) instead of a generic no-op.
