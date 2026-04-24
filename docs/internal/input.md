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
