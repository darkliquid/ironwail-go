# Package: game

## Purpose
The `game` package is the central coordination point for the engine's runtime state. It consolidates what was previously a collection of global variables into a structured `Game` object. It manages the lifecycle of the game loop, routes input, coordinates between the client and server, and maintains caches for various game assets (models, sprites, sounds).

## Key Types & Interfaces
- **`Game`**: The primary struct that owns almost all major subsystem references (`Host`, `Server`, `Client`, `Renderer`, `QC`, `HUD`, etc.).
- **`Renderer`**: An interface used by the game package to interact with the rendering backend without being tied to a specific implementation.
- **`State`**: (In `state.go`) Represents the snapshot of the world state used for rendering and logic.
- **`ViewCalc`**: (In `viewcalc.go`) Handles the calculation of the player's view position, including bobbing, rolling, and damage kicks.
- **`PendingRendererAssets`**: A queue for assets that need to be uploaded to the renderer during the next frame.

## Core Workflow
1. **Startup**: The `New()` function initializes the `Game` instance and its associated caches.
2. **Main Loop**: The `Run()` or `Frame()` methods are called by the `host` package every frame.
3. **Update Cycle**:
    - Process console commands.
    - Update the server (if hosting).
    - Update the client.
    - Synchronize audio.
    - Compute the final camera/view position.
4. **Rendering**: The game logic prepares the scene and calls the `Renderer` to draw the frame.

## Integration
- **`host`**: Owns and drives the `Game` instance.
- **`qc`**: The game package manages the QuakeC Virtual Machine (both server-side and client-side CSQC) and handles the interface between Go and QuakeC.
- **`client` & `server`**: The `Game` struct acts as the bridge between the client and server subsystems, especially in single-player mode.
- **`renderer`**: The game package feeds world geometry, entities, and UI elements to the renderer.

## Learning Tips
- **Centralized State**: Examine the `Game` struct in `game.go` to see how all the disparate parts of a Quake engine are wired together.
- **QuakeC Bridge**: Look at `runtime_csqc.go` and `runtime_ui.go` to understand how client-side QuakeC and UI logic are integrated.
- **View Interpolation**: Check `viewcalc.go` to see how Quake's classic movement "feel" (bobbing, tilting) is implemented in Go.
- **Asset Caching**: Review how `AliasModelCache` and `SoundSFXByIndex` are used to avoid redundant loading of assets during gameplay.
