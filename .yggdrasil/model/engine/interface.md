# Interface

## Main consumers

- Any internal package that wants reusable generic primitives without introducing circular imports.
- Future subsystem migrations that replace ad-hoc maps, slices, and one-off worker scaffolding with typed helpers.

## Main surface

- `Cache[K, V]`
- `Registry[K, V]`
- `Set[T]` and `Queue[T]`
- `EventBus[T]`
- `ParallelLoad`, `LoadPipeline[T]`, `LoadResult[T]`, `LoadFunc[T]`

## Contracts

- The package stays standard-library-only so it can remain universally importable inside `internal/`.
- Each child primitive has distinct lifecycle and ordering rules; callers must choose the right one instead of treating them as interchangeable containers.
