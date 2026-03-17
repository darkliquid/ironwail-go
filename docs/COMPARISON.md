# Codebase Comparison: Ironwail (C) vs. Ironwail-Go (Go)

This document provides a high-level comparison between the original Ironwail C codebase and the Ironwail-Go port.

## 1. Architectural Philosophy

### Memory Management
- **Ironwail (C)**: Uses a manual `Hunk` and `Zone` memory management system inherited from the original Quake. It manages a large pre-allocated heap and performs manual pointer arithmetic.
- **Ironwail-Go (Go)**: Relies on the Go runtime's garbage collector. Replaces `Hunk_Alloc` with standard `make()` or `new()` and utilizes slices instead of raw pointers for collections.

### Concurrency
- **Ironwail (C)**: Primarily single-threaded, with some use of SDL mutexes and threads for specific tasks like async loading or background music.
- **Ironwail-Go (Go)**: Leverages Go's goroutines for background tasks (e.g., filesystem loading, audio streaming) while maintaining a strict `runtime.LockOSThread()` for the main rendering thread to satisfy OpenGL requirements.

## 2. Subsystem Organization

| System | C Implementation (Quake/) | Go Implementation (internal/) |
| :--- | :--- | :--- |
| **Boot/Host** | `main_sdl.c`, `host.c`, `common.c` | `cmd/ironwailgo/main.go`, `internal/host/` |
| **Filesystem** | `common.c` (VFS) | `internal/fs/` |
| **Renderer** | `gl_*.c`, `r_*.c` | `internal/renderer/` |
| **Input** | `in_sdl.c`, `keys.c` | `internal/input/` |
| **QuakeC VM** | `pr_exec.c`, `pr_edict.c` | `internal/qc/` |
| **Server** | `sv_main.c`, `sv_phys.c` | `internal/server/` |
| **Client** | `cl_main.c`, `cl_parse.c` | `internal/client/` |

## 3. Key Divergences

Detailed analysis of specific divergences can be found in the following sub-documents:

1. [Boot and Start Sequence](BOOT_SEQUENCE.md)
2. [Input Handling and Event Loop](INPUT_HANDLING.md)
3. [Rendering Pipeline (OpenGL)](RENDERING.md)

## 4. Parity Goals

The Go port aims for "high-fidelity parity," meaning:
- Identical `progs.dat` (QuakeC) execution behavior.
- Identical physics and movement logic.
- Visual parity with the canonical OpenGL renderer.
- Support for standard Quake data files (PAK, BSP, MDL, SPR).

## 5. Technology Stack

- **C**: C99, SDL2, OpenGL 1.x-3.x (Legacy/Core mix).
- **Go**: Go 1.26, SDL3 (pure-Go via `go-sdl3`), OpenGL 4.6 (Core Profile), GLM-style math via `pkg/types`.
