# Internals

## Logic

Builtins are registered by number onto the VM’s builtin table. Many builtins are thin adapters from VM memory state to typed Go hook calls, especially for server-backed entity/world operations.

The hook interface separates builtin behavior from concrete server implementation so the VM does not import server internals directly.

`random()` consumes the VM compat RNG stream (`rand() & 0x7fff`) and then applies one of two formulas keyed by `sv_gameplayfix_random`. Default-on behavior uses the gameplay fix formula to avoid exact endpoints; legacy-off behavior uses the original closed-interval division formula.

If `sv_gameplayfix_random` is absent, builtin behavior stays on the gameplay-fix path (default formula), matching host default registration.

`mod()` follows C's parity contract for zero divisors by returning `0` while also surfacing the condition through a console warning (`PF_mod: mod by zero`) instead of failing silently.

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
