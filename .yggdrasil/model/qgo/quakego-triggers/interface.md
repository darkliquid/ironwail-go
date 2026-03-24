# Interface

## Main consumers

- quakego gameplay code that invokes trigger entrypoints directly by QuakeC-translated names.
- tests that execute trigger callbacks through `quake.Entity` function slots (`Touch`, `Use`, `Think`).

## Main API

- Top-level trigger functions remain stable for translated call sites:
  - `trigger_multiple()`
  - `trigger_once()`
  - `multi_touch()`
  - `multi_use()`
  - `multi_trigger()`
  - `multi_wait()`
  - `multi_killed()`
- Trigger setup still writes callbacks into entity fields (`Self.Touch`, `Self.Use`, `Self.Think`) using existing signatures.
- `trigger_multiple` now binds the callback fields through the receiver adapter for one parity-safe pair:
  - `Self.Use = (*triggerEntity).use` via bound method value (`te.use`)
  - `Self.Think = (*triggerEntity).wait` when scheduling wait behavior in `trigger`
- `trigger_multiple` continues to initialize trigger entities through `InitTrigger` and optional health/notouch setup.

## Contracts

- Receiver-style refactors must preserve externally visible function names and callback assignments expected by translated gameplay code.
- Callback fields must remain callable as plain `quake.Func` while preserving `Activator`/`Other` semantics when invoked through `Entity.Use` and subsequent `Entity.Think`.
- Touch and use dispatch must continue to propagate through `Activator`/`Other` globals in parity with existing quakego control flow.
- Trigger setup defaults (`Wait` fallback, sound precache selection, trigger solidity/movetype) must remain unchanged.

## Failure modes

- Missing world/activator globals can panic during trigger dispatch in runtime-executable tests; tests must initialize required globals before invoking callback fields.
