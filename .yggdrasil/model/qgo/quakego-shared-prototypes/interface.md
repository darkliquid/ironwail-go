# Interface

## Main consumers

- Other files in `pkg/qgo/quakego` that assign or invoke package-level forward declarations.
- Future maintainers reorganizing QuakeGo package structure without changing emitted gameplay behavior.

## Main API

- Package-level function variables declared in `prototypes.go` for shared gameplay support flows.
- Existing gameplay entry points in the mapped files continue to use those declarations with unchanged names and signatures.
- `combat.go` keeps global QuakeC-compatible entry points (`CanDamage`, `Killed`, `T_Damage`) while allowing receiver-backed internals for entity-local combat behavior.
- `doors.go` keeps `EntitiesTouching(e1, e2 *quake.Entity) float32` as a wrapper API while delegating overlap logic to a receiver-backed helper for pilot method grouping.
- `buttons.go` keeps global QuakeC-compatible entry points (`button_wait`, `button_done`, `button_return`, `button_fire`) while delegating their internals to a receiver-backed adapter for one contained subsystem.

## Contracts

- Moving a forward declaration into `prototypes.go` must not change its identifier, signature, or initialization pattern.
- The mapped files may reference the declarations as before, but they should no longer duplicate the prototype blocks locally.
- Receiver-oriented refactors in mapped files must preserve existing top-level function call patterns used by translated gameplay code.

## Failure modes

- Missing or mismatched declarations fail at Go compile time because the shared files still reference the same package-level symbols.
