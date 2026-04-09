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
- Direct runtime menu entry points must route through the same input-sync policy so startup and game-dir reload menus also restore normal mouse mode immediately.
- Menu mouse handling prefers absolute menu-space hit testing and falls back to relative movement.
- Chat/console text editing includes held-backspace repeat behavior in the command package.
- `registerConsoleCompletionProviders` is also responsible for wiring console completion against executable-owned registries: commands, cvars, aliases, and the live VFS file listing callback when `g.Subs.Files` is a real `*fs.FileSystem`.
- Console key routing mirrors C-style edit semantics: tab/shift-tab completion cycling, ctrl-word delete, delete key, left/right cursor motion (with ctrl-word jumps), insert-mode toggle, and Home/End dual behavior (cursor vs scroll with ctrl).
- `registerConsoleCompletionProviders` also wires command-argument and cvar-value completion providers plus console-backed list-print callback for first-tab match listing.
- Gameplay bind command registration also owns small client-facing convenience commands such as `sizeup`/`sizedown`, which mirror C by bumping the mirrored `scr_viewsize`/`viewsize` cvars in ±10 steps while clamping into the gameplay-safe range (`30..110`) so accidental persisted values cannot hide normal runtime HUD elements.
- Gameplay bind command registration also owns the `entities` client-debug command, which prints an indexed snapshot of the current client entity table and treats missing/zero-model slots as `EMPTY`.
- Gameplay bind command registration also owns `centerview`, which mirrors C by calling the existing client pitch-drift restart path rather than mutating view angles directly.
- Gameplay bind command registration also owns manual profiling commands: `profile_cpu_start [filename]`, `profile_cpu_stop`, `profile_dump_heap [filename]`, and `profile_dump_allocs [filename]`. Relative output paths resolve under `<basedir>/<moddir>/`, default filenames go into a `profiles/` subdirectory, failures are reported back through the console, and an active CPU profile may also be closed by orderly executable shutdown.
- `applyGameplayMouseLook` also owns the gameplay gamepad look path: right-stick look is opt-in via `joy_look`, while gyro contribution is separately gated by `joy_gyro_look` so teams can ship stick-only look without forcing gyro behavior.
