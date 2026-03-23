# Internals

## Logic

`Cache` wraps a plain Go map behind an `RWMutex`. `Get` and `Len` take the read lock, mutation methods take the write lock, and `GetOrSet` serializes the check/store sequence under one write lock so only one stored value wins for a given key. `Clear` swaps in a fresh map, which drops references to stale values in one operation.

## Constraints

- `Range` holds the read lock for the entire iteration, so callbacks must not call other `Cache` methods or they can deadlock.
- Iteration order is Go map order and therefore nondeterministic.
- The mutable zero value is not safely initialized for writes; callers are expected to construct caches via `NewCache`.

## Decisions

### Prefer simple lock-protected maps over heavier cache policy machinery

Observed decision:
- The cache abstraction only provides concurrency-safe storage and lookup primitives, leaving eviction and lifecycle policy to callers.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- The helper stays broadly reusable across subsystems, but callers must own any higher-level invalidation or memory-management policy.
