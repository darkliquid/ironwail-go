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
- `registerConsoleCompletionProviders` is also responsible for wiring console completion against executable-owned registries: commands, cvars, aliases, and the live VFS file listing callback when `g.Subs.Files` is a real `*fs.FileSystem`.
- Gameplay bind command registration also owns small client-facing convenience commands such as `sizeup`/`sizedown`, which mirror C by bumping the mirrored `scr_viewsize`/`viewsize` cvars in ±10 steps.
- Gameplay bind command registration also owns the `entities` client-debug command, which prints an indexed snapshot of the current client entity table and treats missing/zero-model slots as `EMPTY`.
