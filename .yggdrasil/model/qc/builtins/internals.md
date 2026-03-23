# Internals

## Logic

Builtins are registered by number onto the VM’s builtin table. Many builtins are thin adapters from VM memory state to typed Go hook calls, especially for server-backed entity/world operations.

The hook interface separates builtin behavior from concrete server implementation so the VM does not import server internals directly.

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
