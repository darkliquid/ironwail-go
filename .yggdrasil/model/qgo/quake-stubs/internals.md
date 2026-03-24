# Internals

## Logic

Vector/operator behavior is implemented as pure methods/helpers on `Vec3` and remains deterministic.

Entity-level gameplay helpers can live as receiver methods when they only depend on entity-local fields. The first pilot is `(*Entity).Heal`, which centralizes the `T_Heal` arithmetic/ceiling/clamping behavior into the entity type without changing gameplay semantics.

Engine builtin stubs stay as top-level functions with unchanged signatures. A selected subset now performs a fast hook lookup via `backend()` and either calls the configured hook or returns legacy stub defaults. This gives tests a minimal seam without changing call sites in translated gameplay packages.

The stub catalog intentionally includes a broader runtime ID surface than the compiler alias table. Alias-backed directives (`//qgo:builtin <name>`) are intended for frequently used/readable names and must map to declared stubs; numeric directives remain valid for any runtime-only/extension IDs even when no alias exists.

`FieldFloat` and `SetFieldFloat` are intentionally no-op-compatible stubs at runtime; qgo treats them as compiler intrinsics and lowers calls directly to QC field opcodes instead of relying on Go runtime behavior.

A narrow receiver refactor adds `(*Entity).FieldFloat` and `(*Entity).SetFieldFloat` with identical runtime no-op semantics. The package-level wrappers now delegate to the receiver forms, which keeps compiler-facing helper names stable while enabling method-style usage for cohesive entity-local helper clusters.

## Constraints

- do not break `//qgo:builtin` signature and naming expectations used by compiler lowering
- keep alias-backed IDs and runtime stub declarations in sync where aliases are advertised (for example `precache_file2` -> builtin 77)
- preserve default behavior for callers that do not install a backend
- keep backend API small and additive; avoid introducing runtime-engine assumptions into the test seam

## Decisions

### Chose a hook-table backend over replacing builtin functions

Observed decision:
- add `Backend` with function fields and route selected builtins through it.

Rationale:
- keeps existing `engine.*` API stable for both compiler and translated code while enabling deterministic unit tests with regular Go tooling.

Rejected alternatives:
- replace builtins with interface methods and require dependency injection at every call site — rejected because it would require large churn across translated gameplay code.
- implement all builtins immediately in pure Go — rejected because it increases scope and does not provide the minimal testing seam needed for this slice.
