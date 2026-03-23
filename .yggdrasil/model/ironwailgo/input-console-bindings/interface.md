# Interface

## Main consumers

- the runtime loop and app shell, which call into input sync and command registration helpers.
- input/menu/chat flows inside the command package.

## Main surface

- input routing/sync helpers such as `syncGameplayInputMode`, `applyGameplayMouseLook`, `applyMenuMouseMove`, `applyStartupGameplayInputMode`
- gameplay bind/command registration helpers
- chat/console runtime key handling helpers

## Contracts

- Input destination changes also control mouse grab and may force gameplay button release.
- Menu mouse handling prefers absolute menu-space hit testing and falls back to relative movement.
- Chat/console text editing includes held-backspace repeat behavior in the command package.
