# Input Handling and Event Loop Comparison

This document describes how input events are processed in both codebases.

## 1. Input Architecture

- **Ironwail (C)**: Uses `in_sdl.c` to interface with SDL2. It handles raw mouse events (`SDL_MOUSEMOTION`), keyboard events (`SDL_KEYDOWN`), and gamepad events (`SDL_CONTROLLERBUTTONDOWN`).
- **Ironwail-Go (Go)**: Uses `internal/input/` as a backend-neutral abstraction layer. The active runtime backend is supplied by the executable/renderer integration rather than by a package-local SDL implementation.

## 2. Event Dispatching

### C-Implementation (`Key_Event` in `keys.c`)
In C, events are dispatched globally through `Key_Event(int key, qboolean down)`. The code checks `key_dest` to decide if the event goes to the console, the menu, or the game logic.

### Go-Implementation (`HandleKeyEvent` in `types.go`)
Go uses callback functions. Subsystems like `menu` register their interest in keys through:
```go
gameInput.OnMenuKey = handleMenuKeyEvent
gameInput.OnKey = handleGameKeyEvent
```
The input system itself decides which callback to trigger based on the current `KeyDest`.

## 3. Comparison of Features

| Feature | Ironwail (C) | Ironwail-Go (Go) |
| :--- | :--- | :--- |
| **SDL Version** | SDL2 (C headers) | SDL3 (Pure-Go) |
| **Mouse Acceleration** | Handled manually with Mac hacks | Handled by SDL3 and OS settings |
| **Gamepad Support** | Extensive (Gyro, Rumble, Deadzones) | Initial support (Deadzones only) |
| **Text Mode** | `K_TEXTMODE` and manual buffer handling | `HandleCharEvent()` with `rune` support |

## 4. Key Takeaways for Parity

- **Mouse Look**: Both implementations accumulate `deltaX` and `deltaY` and apply sensitivity scaling in `applyGameplayMouseLook()`.
- **Key Bindings**: The Go implementation maintains identical Quake keycodes (e.g., `KMWheelUp`, `KMouse1`) to ensure compatibility with `config.cfg` and `autoexec.cfg`.
- **Text Entry**: Go's use of `rune` for character events is more robust for internationalized text entry compared to the ASCII-focused approach of original Quake.
