# Internals

## Logic

The node keeps low-risk structural cleanup focused on package-level forward declarations. `prototypes.go` groups the declarations by source file so readers can locate shared cross-file hooks quickly while leaving the implementation functions in their original gameplay files.

A follow-up cleanup converted `doors.go` door spawnflag constants from mutable `float32` vars to compile-time `iota` bit constants (`doorFlag*`). This preserves QuakeC-equivalent bit masks while removing magic-number checks and making spawnflag intent explicit at call sites.

Another mechanical cleanup converted `items.go` health spawnflag constants to `iota` bit constants (`healthFlag*`) with unchanged mask values (`1` and `2`). The call sites still perform bit tests against `Self.SpawnFlags`; only declaration style and local naming were modernized.

A similarly narrow cleanup converted `misc.go` trap-shooter spawnflag constants to `iota` bit constants (`spawnFlagSuperSpike`, `spawnFlagLaser`) while preserving the original Quake mask values (`1` and `2`). Behavior remains the same because all call sites still test bits against `Self.SpawnFlags`; only constant representation changed.

A further scoped cleanup converted `doors.go` secret-door spawnflag constants (`SECRET_*`) from mutable `float32` vars to compile-time `iota` bit constants, preserving the original mask sequence (`1, 2, 4, 8, 16`). The existing bit tests and the `temp = 1 - (spawnflag & SECRET_1ST_LEFT)` parity trick continue to behave identically because `SECRET_1ST_LEFT` remains `2`.

A bounded readability pilot in `items.go` now delegates `T_Heal` to `(*quake.Entity).Heal(...)`. This keeps the global function entry point intact for parity and call-site stability while moving entity-local healing rules to a receiver method in the shared quake stubs.

An additional narrow combat/helper cluster in `combat.go` now uses a local receiver-backed adapter (`combatEntity`) for `CanDamage`, `Killed`, and `T_Damage` internals. Each global entry point still exists and immediately delegates, so translated call sites keep QuakeC-style names while entity-local logic is grouped as methods.

A doors pilot now mirrors this adapter pattern for one narrow helper: `EntitiesTouching` delegates to `(*doorEntity).touches(...)` in `doors.go`. The global helper signature remains unchanged for existing callers (such as `LinkDoors`) while bounding-box overlap checks are grouped on a receiver-backed alias.

## Constraints

The declarations must remain package-level `var` function slots because the translated QuakeC code assigns implementations after declaration order has been established. Replacing them with direct function declarations or broader API rewrites would risk changing compiler assumptions and is intentionally out of scope.

The pilot intentionally avoided converting broader `Self`-driven globals into methods because those functions mutate process-global quakego state (`Self`, `Other`, `Activator`, `Time`) and often swap `Self` mid-function; method conversion there is higher risk for parity.

For combat receiver adaptation, a type alias pattern (`type combatEntity quake.Entity`) is used so method grouping can be introduced without changing `quake.Entity` ownership in the `quake` package or altering existing function signatures.

## Decisions

- Chose a dedicated `prototypes.go` over spreading declarations across eight support files because the work item asked for a contained structural improvement and this keeps the cleanup mechanical.
- Chose not to consolidate every monster-specific prototype block in the package because limiting the first pass to shared support files avoids a large blast radius while still improving organization.
