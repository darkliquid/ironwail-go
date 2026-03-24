# Interface

## Main consumers

- QuakeGo gameplay code in `pkg/qgo/quakego`
- focused unit tests that execute stubs directly via normal Go runtime
- `qgo/compiler`, which depends on stable builtin signatures and stub type names

## Main API

Core type + vector surface:
- `MakeVec3(x, y, z float32) Vec3`
- `Vec3` methods (`Add`, `Sub`, `Mul`, `Div`, `Dot`, `Neg`, `Cross`, `Lerp`)
- operator emulation helpers (`OpAddVV`, `OpSubVV`, `OpMulVF`, `OpMulFV`, `OpMulVV`, `OpDivVF`, `OpNegV`)

Engine backend injection surface:
- `type Backend struct { ... }` hook table for selected engine builtins used by translated gameplay code
- `SetBackend(backend Backend)` to install deterministic test hooks
- `ResetBackend()` to restore default no-op/zero-value stub behavior

Engine builtin stubs:
- existing `engine.*` builtin functions keep compiler-compatible signatures
- a focused subset routes through installed backend hooks when present (`SetOrigin`, `Random`, `Spawn`, `Find`, `WriteByte`, `WriteString`, etc.)

Dynamic field helper stubs:
- `FieldFloat(entity, field)` and `SetFieldFloat(entity, field, value)` are present as compiler-intrinsic seam helpers for qgo dynamic-field lowering.

## Contracts

- builtin function signatures remain stable for compiler directive mapping (`//qgo:builtin N`)
- when no hook is set, behavior remains compatible with previous stubs (zero/nil/identity/no-op)
- backend hooks are process-global and intended for controlled test setup/teardown

## Failure modes

- missing hooks for behavior a test expects will silently fall back to stub defaults
- forgetting to `ResetBackend` between tests can leak hook state across test cases
