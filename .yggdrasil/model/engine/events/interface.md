# Interface

## Main consumers

- Subsystems that want decoupled typed callbacks while still handling events inside the current frame / owner thread.
- Setup code that installs and later removes subscribers through returned unsubscribe closures.

## Main surface

- `NewEventBus`
- `Subscribe`
- `Publish`
- `Len`

## Contracts

- Subscribers are invoked synchronously in registration order.
- `Subscribe` returns an unsubscribe closure for that exact callback slot.
- `Publish` is intended for deterministic owner-thread/game-loop use even though subscription itself is mutex-protected.
