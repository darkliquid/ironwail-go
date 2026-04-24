# Package: host

## Purpose
The `host` package is the engine's top-level orchestrator. It is responsible for the overall lifecycle of the application: starting up subsystems, managing the main execution loop, handling frame timing, and ensuring clean shutdown. It acts as the "glue" that binds together the various independent subsystems like the renderer, audio, and game logic.

## Key Types & Interfaces
- **`Host`**: The main controller struct. It tracks global state like `realtime`, `frametime`, and the current client connection state.
- **`FrameCallbacks`**: An interface that defines the steps of a single engine frame (`GetEvents`, `ProcessConsoleCommands`, `ProcessServer`, `ProcessClient`, `UpdateScreen`, `UpdateAudio`).
- **`Subsystems`**: A container for the various engine components (filesystem, commands, console, etc.) that the host manages.
- **`MainThreadQueue`**: (In `mainthread.go`) A mechanism to ensure that certain operations (like window management or renderer calls) are executed on the main OS thread.

## Core Workflow
1. **Init**: The host initializes the filesystem, console variables (CVars), and command system.
2. **Frame Loop**: In each frame:
    - **Timing**: It filters and accumulates `dt` (delta time) to maintain consistent simulation speed, respecting CVars like `host_maxfps`.
    - **Commands**: Executes any pending console commands.
    - **Simulation**: Calls back into the game logic to update the server and client states.
    - **Rendering**: Triggers the screen update.
3. **Main Thread Marshalling**: Throughout the loop, it drains the `MainThreadQueue` to handle thread-sensitive OS tasks.

## Integration
- **`game`**: The `game` package implements the `FrameCallbacks` interface, which the `host` calls during its main loop.
- **`fs`**: The host uses the filesystem to load initial configuration files (`ironwail.cfg`, `autoexec.cfg`).
- **`cmdsys` & `cvar`**: The host initializes and manages the systems for console commands and variables.
- **`audio` & `renderer`**: The host coordinates the timing and updates for these output systems.

## Learning Tips
- **Frame Timing**: Look at `Frame()` in `frame.go` to see how the engine handles variable vs. fixed framerates and how it prevents simulation "speeding" during high FPS.
- **Main Thread Pattern**: Examine `mainthread.go` to understand how the engine works around the requirement that many GUI/Renderer APIs must be called from the thread that initialized them.
- **Subsystem Isolation**: Notice how `host` uses interfaces (like `FrameCallbacks`) rather than direct dependencies on the `game` package, making the core engine more modular and testable.
- **Autosave Logic**: Check `autosave.go` to see how the host implements the modern quality-of-life feature of automatically saving the game when changing levels.
