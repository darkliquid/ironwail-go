# Package `console`

## Purpose
The `console` package implements the engine's text-based interface. It maintains a scrollable buffer of messages printed by the engine and gameplay code, handles user input for commands, and manages "notify" lines displayed briefly during gameplay. It serves as the primary diagnostic and configuration tool for both players and developers.

## Key Types & Interfaces

- **`Console`**: The central state struct. It manages a large byte slice as a ring buffer for scrollback, an input line for current typing, and notification timestamps.
- **`globalConsole`**: A singleton instance of `Console` used by most of the engine.
- **`DefaultTextSize`**: A constant (1MB) defining the default scrollback buffer capacity.

## Core Workflow

1.  **Printing**: Code calls `Printf`, `DPrintf` (developer-only), or `Warning`. These funnel into `printRaw`, which handles:
    *   **Word Wrapping**: Breaking lines to fit the current `lineWidth`.
    *   **Special Characters**: Handling newlines, carriage returns, and Quake's "bronze" text prefix (0x01/0x02).
    *   **Notifications**: Recording the timestamp of the new line so it can be shown on-screen.
2.  **Scrolling**: The `backScroll` field tracks how far up the user has scrolled. Functions like `Scroll` and `SetBackScroll` manipulate this state.
3.  **Resizing**: When the window dimensions change, `Resize` re-calculates the buffer layout. This involves re-mapping the flat ring buffer contents to a new line width while preserving the most recent history.
4.  **Logging**: If enabled via `condebug`, all console output is mirrored to a file on disk.

## Integration

- **`internal/cvar`**: The console uses the cvar system to look up flags like `developer` and `con_logcenterprint`.
- **`internal/draw`**: Higher-level rendering packages read from the `Console` buffer and use characters from `draw.GetConcharsData()` to render the text to the screen.
- **`internal/cmdsys`**: When the user presses Enter, the console's `inputLine` is typically passed to the command system for execution (this interaction usually happens in the `host` or `input` packages).
- **Filesystem**: `Dump` and `EnableDebugLog` interact with the OS filesystem to save logs and buffer captures.

## Learning Tips

- **Ring Buffer Implementation**: Look at how `c.current` and `c.totalLines` are used with modular arithmetic (`%`) to manage the flat `text` slice.
- **Thread Safety**: Notice the use of `sync.RWMutex`. Rendering often happens on a different goroutine than engine updates, so thread-safe access to the text buffer is critical.
- **High-Bit Encoding**: Quake text often sets the 7th bit (0x80) to indicate the "bronze" color variant. Look at `printRaw` and `Dump` to see how this is applied and stripped.
- **Resize Logic**: The `Resize` method is a great example of handling data layout transitions. It's more complex than a simple slice resize because it must preserve line-based readability.
