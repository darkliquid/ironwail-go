# Responsibility

## Purpose

`engine/events` owns the typed synchronous publish/subscribe primitive used for decoupled event delivery without abandoning deterministic same-thread ordering.

## Owns

- `EventBus[T]` subscriber storage and publish behavior.
- Subscription ordering semantics.
- Unsubscribe closures and active-subscriber counting.

## Does not own

- Asynchronous event queues, worker pools, or cross-thread scheduling.
- Event persistence, replay, or filtering policies beyond subscriber registration.
