# Interface

## Main consumers

- QuakeGo gameplay code in `pkg/qgo/quakego`
- focused unit tests that execute stubs directly via normal Go runtime

## Main API

`Vec3` value methods:
- `MakeVec3(x, y, z float32) Vec3`
- `(Vec3).Add(o Vec3) Vec3`
- `(Vec3).Sub(o Vec3) Vec3`
- `(Vec3).Mul(s float32) Vec3`
- `(Vec3).Div(s float32) Vec3`
- `(Vec3).Dot(o Vec3) float32`
- `(Vec3).Neg() Vec3`
- `(Vec3).Cross(o Vec3) Vec3`
- `(Vec3).Lerp(o Vec3, t float32) Vec3`

Operator-emulation helpers:
- `OpAddVV(a, b Vec3) Vec3`
- `OpSubVV(a, b Vec3) Vec3`
- `OpMulVF(a Vec3, s float32) Vec3`
- `OpMulFV(s float32, a Vec3) Vec3`
- `OpMulVV(a, b Vec3) float32`
- `OpDivVF(a Vec3, s float32) Vec3`
- `OpNegV(a Vec3) Vec3`

Entity flag helpers:
- `type EntityFlags uint32`
- `EntityFlagsFromFloat(v float32) EntityFlags`
- `(EntityFlags).Float32() float32`
- `(EntityFlags).Has(mask EntityFlags) bool`
- `(EntityFlags).With(mask EntityFlags) EntityFlags`
- `(EntityFlags).Without(mask EntityFlags) EntityFlags`
- `(*Entity).FlagsValue() EntityFlags`
- `(*Entity).SetFlagsValue(flags EntityFlags)`
- `(*Entity).HasFlags(mask EntityFlags) bool`
- `(*Entity).AddFlags(mask EntityFlags)`
- `(*Entity).ClearFlags(mask EntityFlags)`
- `(*Entity).SpawnFlagsValue() EntityFlags`
- `(*Entity).SetSpawnFlagsValue(flags EntityFlags)`

Named `EntityFlags` constants:
- `FlagFly`, `FlagSwim`, `FlagClient`, `FlagInWater`, `FlagMonster`, `FlagGodMode`, `FlagNoTarget`, `FlagItem`
- `FlagOnGround`, `FlagPartialGround`, `FlagWaterJump`, `FlagJumpReleased`, `FlagIsBot`
- `FlagNoPlayers`, `FlagNoMonsters`, `FlagNoBots`, `FlagObjective`

## Contracts

- helpers mirror QC vector operation intent while remaining deterministic pure-Go functions
- vector math operates component-wise except `Dot`/`OpMulVV`, which return scalar dot product
- all operations are side-effect free and return new values
- entity flags stay float-backed on `Entity` fields for QC compatibility while providing typed bitmask APIs at call sites
- nil entity receivers are tolerated by flag helper methods (`FlagsValue`/`SpawnFlagsValue` return zero, mutators are no-ops)

## Failure modes

- `Div` and `OpDivVF` rely on Go float behavior; dividing by zero yields IEEE-754 infinities/NaN rather than panicking
