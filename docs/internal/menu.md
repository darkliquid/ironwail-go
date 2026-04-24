# Package menu

## Purpose

The `menu` package implements the Quake in-game menu system. It manages the menu's finite state machine (navigating between screens like Options, Multiplayer, and Video), handles UI-specific input, and orchestrates the drawing of menu elements.

## Key Types & Interfaces

- **`Manager`**: The central object that tracks the current `MenuState`, cursor positions, and internal buffers (like the server address field).
- **`MenuState`**: An enum representing each distinct menu page (e.g., `MenuMain`, `MenuOptions`, `MenuVideo`).
- **`DrawManager` Interface**: An abstraction used to retrieve graphics (`QPic`) without tying the menu package directly to the filesystem or a specific renderer.

## Core Workflow

1.  **Activation**: The menu is toggled via `Manager.ToggleMenu()`, which switches the input system to `KeyMenu` mode.
2.  **Navigation**: Keyboard and gamepad events are passed to `Manager.M_Key()`. This method routes the key to a page-specific handler (e.g., `mainKey`, `optionsKey`), which updates the cursor or transitions to a new `MenuState`.
3.  **Input**: Character events for text fields (like the Player Name) are handled via `Manager.M_Char()`.
4.  **Rendering**: Each frame, the host calls `Manager.M_Draw()`. The manager uses a `renderer.RenderContext` to draw pictures and text on a virtual 320×200 grid.

## Integration

- **`internal/input`**: The menu package relies on the input system to receive events and to set the `KeyDest`.
- **`internal/renderer`**: All menu drawing is performed through the `RenderContext` provided by the renderer.
- **`internal/cvar`**: Many menu items (sliders, toggles) directly read and write console variables.
- **`internal/mods`**: The Mods menu uses the `downloader` from this package to browse and install addons.

## Learning Tips

- **Gamepad Mapping**: Look at `normalizeMenuKey` to see how gamepad buttons (like the 'A' button) are mapped to standard keyboard equivalents (like 'Enter') for the menu.
- **Virtual Resolution**: Observe how all coordinates in `menu_draw.go` assume a 320x200 screen, regardless of the actual window size.
- **Hierarchical Navigation**: Notice how `MenuQuit` or `MenuMain` often store a `prev` state to allow the Escape key to function as a "Back" button.
