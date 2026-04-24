# Package input

## Purpose

The `input` package provides an engine-wide abstraction for keyboard, mouse, and gamepad input. it translates platform-specific events (e.g., from SDL or GLFW) into a unified set of Quake-compatible key codes and manages the routing of these events to different parts of the engine (game, console, or menu).

## Key Types & Interfaces

- **`System`**: The top-level manager that tracks key states, bindings, and routing destinations.
- **`Backend` Interface**: A platform-neutral interface that must be implemented by the OS-specific layer (e.g., `internal/input/sdl3`) to feed raw events into the engine.
- **`KeyEvent` / `MouseEvent` / `GamepadState`**: Structured event types representing specific input actions.
- **`KeyDest`**: An enum defining where input is currently routed (`KeyGame`, `KeyConsole`, `KeyMessage`, `KeyMenu`).

## Core Workflow

The package operates as a 3-layer pipeline:
1.  **Platform Layer**: The `Backend` implementation polls the OS for hardware events.
2.  **Translation Layer**: Raw hardware codes (scancodes, button IDs) are converted into canonical engine key codes (the `K_*` constants).
3.  **Routing Layer**: The `System` dispatches these events to registered callbacks based on the current `KeyDest`. For example, if the menu is open, events are routed to `OnMenuKey`.

## Integration

- **`internal/host`**: Initializes the input system and provides the concrete `Backend`.
- **`internal/menu`**: Registers callbacks to handle UI navigation.
- **`internal/client`**: Queries the `System` for movement and action bindings during gameplay.
- **`internal/console`**: Uses character events for text entry and command history.

## Learning Tips

- **Gamepad Alt-Modifiers**: Ironwail implements a unique "alt-modifier" system for gamepads. Look at how `KGamepadBegin` and the `_Alt` constants allow every physical button to have two different bindings.
- **Input Routing**: Study `HandleKeyEvent` to see how the engine decides whether to send a key to the menu system or the general game logic.
- **320x200 Coordinate Space**: Note that mouse movement is often accumulated and then scaled to match Quake's virtual resolution.

## Tests

**`TestFunctionKeyStringRoundTrip`** — Verifies `KeyToString(key)==name` and `StringToKey(name)==key` for F1, F9, F10, F11, and F12. Key bindings are stored in `config.cfg` as strings like `"bind F1 impulse 1"`; broken round-trips would lose bindings on restart. Table-driven, calls both functions in each direction.

**`TestNamedKeyStringRoundTrip`** — Same round-trip for named special keys: INS, DEL, PGDN, PGUP, HOME, END, PAUSE, PRINTSCREEN, SEMICOLON, BACKQUOTE, TILDE, and controller keys (LSHOULDER, RTRIGGER_ALT). Covers the full set of named keys that users can bind in the console.

**`TestPrintablePunctuationRoundTrips`** — Verifies that `'`, `\`, `` ` ``, `~` have non-empty string names and that `StringToKey(KeyToString(k))==k`. These characters appear in console commands and cfg files; a broken mapping would prevent binding them.

**`TestHandleCharEventRoutesToMenuCharCallback`** — Sets key destination to `KeyMenu`, fires a char event `'a'`, asserts both `OnMenuChar` and `OnChar` callbacks receive `'a'`. Text input in the menu (player name, connect address) must be routed to the right handler; missing routing means keyboard input is silently dropped. Uses a stub `textModeBackend` with injected closures.

**`TestUpdateTextModeEnablesTextForMenu`** — Verifies that switching to `KeyMenu` calls `backend.SetTextMode(TextModeOn)`, and switching back to `KeyGame` calls `SetTextMode(TextModeOff)`. On some platforms (Android, IME users), `SetTextMode(On)` activates the on-screen keyboard or enables key-repeat; without it, the user cannot type.

**`TestHandleKeyEventFiltersAutorepeatOnlyInGame`** — In `KeyGame`, two consecutive key-down events for `'x'` produce only 1 `OnKey` callback. In `KeyMenu`, the same sequence produces 2 callbacks. Quake gameplay intentionally ignores key-repeat (firing from held key would be cheating); menus need repeat for scrolling through options. Counts callbacks from injected closures.

**`TestHandleKeyEventStopsGeneralDispatchWhenMenuChangesDest`** — The `OnMenuKey` handler changes `KeyDest` to `KeyGame`; asserts that `OnKey` is NOT subsequently called for the same event. When Esc closes the menu, the game shouldn't also process Esc as a game key in the same frame. `OnMenuKey` calls `sys.SetKeyDest(KeyGame)` inside the handler; counts `gameEvents` after.

**`TestHandleKeyEventIgnoresStrayKeyUp`** — A key-up for `'z'` without a prior key-down produces zero callbacks and leaves `IsKeyDown('z')` false. Focus changes (alt-tab, menu transitions) can lose the key-down event; processing the stray up as a down/up pair would trigger unintended game actions. Fires up-event first, checks zero events; then fires down+up and expects 2.
