# Internals

## Logic

Builtins are registered by number onto the VM’s builtin table. Many builtins are thin adapters from VM memory state to typed Go hook calls, especially for server-backed entity/world operations. The trace-control slots (`traceon`/`traceoff`) are local VM toggles rather than server hooks: they flip `vm.Trace`, which the interpreter already checks before calling `TraceFunc` for each statement.

The hook interface separates builtin behavior from concrete server implementation so the VM does not import server internals directly.

The DP/FTE-style trig extension slots map directly to Go's `math` package without degree conversion so they match C `sin/cos/tan/asin/acos/atan/atan2` behavior. Quake angle-degree helpers like `makevectors`, `vectoyaw`, and `vectoangles` remain separate builtins with their own degree-based conventions.

`substring()` mirrors the C helper's byte-oriented slicing rules instead of clamping everything to non-negative bounds. Negative `start` is applied relative to the string end, negative `length` is rewritten as "remaining chars after start, minus tail trim", and non-positive effective lengths return the empty string.

The string-concatenation helpers use `vm.ArgC` to mirror C's variadic behavior. `strcat()` and the Go no-op version of `strzone()` both concatenate all provided QC string arguments in order, while still tolerating direct unit-test calls that bypass the interpreter and leave `ArgC` unset.

`random()` consumes the VM compat RNG stream (`rand() & 0x7fff`) exactly at its current injected state and then applies one of two formulas keyed by `sv_gameplayfix_random`. Default-on behavior uses the gameplay fix formula to avoid exact endpoints; legacy-off behavior uses the original closed-interval division formula.

Trace-parity audit confirmed C `PF_random` formula alignment against Ironwail `pr_cmds.c`:
`((rand() & 0x7fff) + 0.5f) * (1.f / 0x8000)` when fix is on, `(rand() & 0x7fff) / 0x7fff` when off.
The concrete stream delta is offset-only: if one upstream `rand()` draw has already happened (for example, host frame-time entropy upkeep), every QC `random()` value shifts by one compat-rand slot with no additional distortion.

If `sv_gameplayfix_random` is absent, builtin behavior stays on the gameplay-fix path (default formula), matching host default registration.

`mod()` follows C's `PF_mod` formula exactly (`a - n * (int)(a/n)`), which implies truncation-toward-zero quotient behavior for signed/float operands. Tests pin a behavior matrix for positive/negative operand combinations plus `±0` divisors, where zero divisors emit `PF_mod: mod by zero` and return `0`.

Builtin slot `#28` now maps to `coredump` semantics: iterating the currently allocated edict range (`0..NumEdicts-1`) and printing entity headers to the console. This preserves canonical slot behavior (C `PF_coredump`) and avoids silently swallowing QC calls at that index.

## Constraints

- Incorrect builtin numbering would silently break program behavior.
- Hook registration is required for server-backed builtins to function safely.
- Builtin behavior is tightly coupled to VM layout semantics and shared mutable state.

## Decisions

### Hook interface instead of direct server dependency

Observed decision:
- Builtins call into typed hook interfaces rather than importing concrete server code directly.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- the VM remains modular and testable
- server integration remains explicit at the boundary

### Narrowest next parity implementation slice for RNG provenance

Observed decision:
- Add regression trace coverage first (zero-offset vs one-draw-offset) before changing runtime RNG wiring.

Rationale:
- minimize churn while proving deterministic deltas in executable tests
- isolate parity work to provenance/wiring after formula parity is locked

Observed effect:
- concrete expected traces now exist for both gameplayfix modes
- next code slice can focus only on where/when upstream draws are consumed, not on float math semantics
