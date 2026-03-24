# Internals

## Logic

The vector API is implemented directly on top of `Vec3`'s `[3]float32` layout so semantics remain transparent and close to QCVM vector slots. Operator-emulation helpers are thin adapters to the method surface, keeping one canonical implementation path for each operation.

Entity flag helpers keep `Entity.Flags` and `Entity.SpawnFlags` as `float32` to preserve entvar compatibility while introducing `EntityFlags` (`uint32`) for bitwise operations. The conversion seam is explicit (`EntityFlagsFromFloat` and `Float32`) so gameplay code can use typed masks without changing field storage layout.

## Constraints

- preserve straightforward arithmetic semantics to support test execution outside compiler/lowering paths
- keep the helper naming explicit (`Op*`) to avoid confusion with regular Go operators that cannot be overloaded
- keep entity/storage compatibility by leaving `Entity` flag fields float-backed while adding typed wrappers for reads and writes

## Decisions

### Thin helper wrappers over duplicated arithmetic

Observed decision:
- `Op*` helper functions delegate to `Vec3` methods instead of re-implementing each formula.

Rationale:
- avoid divergence between method semantics and operator-emulation behavior.

Rejected alternatives:
- duplicate formulas in each `Op*` helper — rejected because it increases drift risk and test burden.

### Typed wrappers over changing `Entity` field types

Observed decision:
- introduce `EntityFlags` plus `Entity` helper methods, but keep `Entity.Flags` and `Entity.SpawnFlags` as `float32`.

Rationale:
- this tightens type safety for common bitflag operations immediately while avoiding broad compiler and generated-code churn in the same slice.

Rejected alternatives:
- change `Entity.Flags`/`SpawnFlags` field types directly to a non-float type — rejected for now due to larger compatibility and lowering implications outside this focused stub slice.
