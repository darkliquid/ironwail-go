# Internals

## Logic

`Registry` is another map-plus-`RWMutex` helper, but its semantics differ from `Cache`: writes are treated as initialization-time declarations rather than ordinary mutations. `Register` checks the frozen flag and existing keys under the write lock, panicking immediately on misuse. `Freeze` flips a permanent gate that keeps later writes from silently changing runtime-visible tables.

## Constraints

- Panic paths are intentional and define the programming-error contract.
- `Range` executes under the read lock and inherits the same practical callback caveat as other lock-held iterators.
- Iteration order is Go map order and therefore arbitrary.
- As with other map-backed helpers here, mutation-safe use expects construction via `NewRegistry`.

## Decisions

### Use panic-on-misuse instead of silent overwrite for registration bugs

Observed decision:
- Registry misuse is treated as a developer wiring error, not as recoverable runtime state.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- Duplicate or too-late registration failures surface immediately during initialization/testing instead of corrupting later runtime lookup behavior.
