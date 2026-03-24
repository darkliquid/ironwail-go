# Internals

## Logic

The node introduces a narrow receiver adapter (`type triggerEntity quake.Entity`) to cluster one cohesive trigger family slice without changing global entrypoints. Wrapper functions (`multi_trigger`, `multi_touch`, `multi_use`, `multi_killed`, `multi_wait`, `trigger_multiple`, `trigger_once`) delegate to receiver methods on `*triggerEntity`.

The delegated methods keep existing global-state behavior:

- `(*triggerEntity).setupMultiple` handles trigger setup (`InitTrigger`, callback registration, wait default, health/notouch branch).
- `(*triggerEntity).touch` performs player + movedir checks before dispatching activation.
- `(*triggerEntity).trigger` performs activation flow (`Activator` assignment, `SUB_UseTargets`, wait/removal scheduling, secret/dopefish side effects).
- `(*triggerEntity).setupOnce` keeps one-shot setup by setting `Wait = -1` then reusing multiple setup.

For the think/use bridge, callback assignment now binds one handler pair directly to receiver methods:

- Setup path writes `Self.Use = te.use` from `(*triggerEntity).setupMultiple`.
- Activation wait path writes `Self.Think = te.wait` from `(*triggerEntity).trigger` when `Wait > 0`.

This keeps callback type compatibility (`quake.Func`) while routing callback invocation through typed receiver methods.

## Constraints

The change is intentionally narrow to parity-safe trigger setup/touch/activation flow and does not convert unrelated trigger families (`teleport`, `hurt`, `push`, `monsterjump`) in the same file. This follows the existing buttons/doors pilot strategy: preserve top-level QuakeC function shape while grouping internals via a receiver alias.

## Decisions

- Chose a local adapter type over editing `quake.Entity` directly because gameplay conversion should stay within quakego files and mirror prior pilot scope.
- Chose to keep all existing wrapper entrypoints callable because translated QuakeC code and callback assignment sites depend on those names and signatures.
- Chose method-value callback binding for only one think/use pair over broad conversion because the task requires a narrow regression-safe bridge that does not alter general dispatch policy.
