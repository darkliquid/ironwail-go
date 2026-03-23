# Responsibility

## Purpose

`engine/cache` owns the mutable concurrent key/value cache abstraction used for runtime object lookup, replacement, and invalidation.

## Owns

- `Cache[K, V]` construction and storage.
- Concurrent `Get`, `Set`, `GetOrSet`, `Delete`, `Clear`, `Len`, and `Range` behavior.
- The lazy-population contract exposed by `GetOrSet`.

## Does not own

- Eviction policies, reference counting, expiration, or memory budgeting.
- Domain-specific cache invalidation triggers.
