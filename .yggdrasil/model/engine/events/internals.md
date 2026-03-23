# Internals

## Logic

`EventBus` stores subscribers in a slice protected by an `RWMutex`. `Subscribe` appends a callback, remembers its index, and returns a closure that later nils that slot instead of compacting the slice. `Publish` snapshots the slice header under a read lock, then iterates synchronously over the stored callbacks, skipping nil tombstones so unsubscribed slots no longer fire.

## Constraints

- Delivery order is registration order.
- Unsubscribe leaves nil tombstones behind, so heavy subscribe/unsubscribe churn can grow backing storage without compaction.
- The code documents `Publish` for game-loop/owner-thread use; stronger concurrent publish/unsubscribe guarantees are not part of the explicit contract.

## Decisions

### Favor synchronous same-frame delivery over asynchronous event dispatch

Observed decision:
- Event handlers run immediately in the publishing call path rather than being deferred to another goroutine or queue.

Rationale:
- **unknown — inferred from code and comments, not confirmed by a developer**

Observed effect:
- Call ordering is easy to reason about and debug, but publishers and subscribers share latency and reentrancy concerns in the same call stack.
