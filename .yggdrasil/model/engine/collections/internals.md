# Internals

## Logic

`Set` is a thin typed wrapper around `map[T]struct{}` that exposes common membership helpers. `Queue` stores elements in a ring buffer backed by a slice with `head`, `tail`, and `len` indices; when full it doubles capacity, linearizes the existing ring into a fresh buffer, and continues pushing without shifting every element.

## Constraints

- `Set` is returned by value from `NewSet`, but the underlying map is reference-backed, so copying a `Set` aliases the same membership store.
- The mutable zero value of `Set` is not safe for `Add` without prior initialization.
- `Queue` has a minimum capacity of 4 and is not safe for concurrent use.
- `Queue.Pop` clears the removed slot to release references for the garbage collector.

## Decisions

### Prefer minimal wrappers over richer collection frameworks

Observed decision:
- The collection helpers expose only the small operations the engine currently needs, rather than a broader generalized collection API.

Rationale:
- **unknown — inferred from code, not confirmed by a developer**

Observed effect:
- The helpers stay easy to audit and cheap to adopt, but callers own any more advanced coordination or ordering behavior beyond membership and FIFO buffering.
