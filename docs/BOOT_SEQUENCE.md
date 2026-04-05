# Boot and Start Sequence Comparison

This document details the initialization process of the original C codebase versus the Go port.

## 1. Entry Points

- **Ironwail (C)**: `main_sdl.c` contains the `main()` function. It performs initial command-line parsing, memory allocation, and enters the `while(1)` loop.
- **Ironwail-Go (Go)**: `cmd/ironwailgo/main.go` contains `func main()`. It uses the `flag` package for parsing and manages a callback-based loop via `gameRenderer.OnUpdate`.

## 2. System Initialization Flow

### C-Implementation (`Host_Init` in `host.c`)
1.  **Memory**: `Memory_Init()` sets up the global heap.
2.  **Cvar/Cmd**: `Cvar_Init()` and `Cmd_Init()` register engine-level variables and commands.
3.  **VFS**: `COM_InitFilesystem()` mounts `PAK` files and sets the base game directory.
4.  **Audio/Video**: `S_Init()` and `VID_Init()` start the sound engine and create the window.
5.  **Client/Server**: `SV_Init()` and `CL_Init()` initialize the game simulation and client state.

### Go-Implementation (`initSubsystems` in `main.go`)
1.  **Input**: `input.NewSystem()` creates the abstracted input handler.
2.  **Filesystem**: `fs.NewFileSystem().Init()` performs mounting similarly to the C version.
3.  **QuakeC**: `gameQC.LoadProgs()` reads `progs.dat` and prepares the VM.
4.  **Host**: `initGameHost()` initializes the coordinator.
5.  **Renderer**: `renderer.NewWithConfig()` initializes the GoGPU renderer or the explicit no-backend fallback.
6.  **Audio**: `audio.NewAudioAdapter()` starts the canonical Oto audio backend and falls back to null audio if hardware init fails.

## 3. The Main Loop

| Aspect | Ironwail (C) | Ironwail-Go (Go) |
| :--- | :--- | :--- |
| **Structure** | `while(1) { ... Host_Frame(); ... }` | Callback-driven (OnUpdate/OnDraw) |
| **Timing** | `Sys_Throttle()` and `SDL_Delay()` | Delta-time (`dt`) passed to callbacks |
| **Input Polling** | Manual `SDL_PollEvent()` in the loop | Automated via `gameInput.PollEvents()` |
| **Rendering** | Immediate-mode or manual FBO draws | Encapsulated in `dc.RenderFrame()` |

## 4. Initialization Divergence Summary

- **Memory**: The C version's `parms.membase = malloc(parms.memsize)` is entirely absent in Go.
- **FS Mounting**: Go's filesystem implementation is more modular and uses standard Go `io.Reader` interfaces.
- **Subsystem Isolation**: Go enforces better isolation between subsystems (`internal/` packages), whereas C relies on global headers and `extern` declarations.
