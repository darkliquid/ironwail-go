# Interface

## Main consumers

- Runtime systems that need shared mutable lookup tables such as asset/model/texture/sound caches.
- Callers that want thread-safe reads with occasional writes.

## Main surface

- `NewCache`
- `Get`
- `Set`
- `GetOrSet`
- `Delete`
- `Clear`
- `Len`
- `Range`

## Contracts

- Reads and writes are protected by an internal `sync.RWMutex`.
- `GetOrSet` returns the existing cached value when present and reports whether the value already existed.
- `Range` callbacks run while the cache is read-locked.
