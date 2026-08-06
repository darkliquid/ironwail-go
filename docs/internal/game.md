# Package: game

## Purpose
The `game` package is the central coordination point for the engine's runtime state. It consolidates what was previously a collection of global variables into a structured `Game` object. It manages the lifecycle of the game loop, routes input, coordinates between the client and server, and maintains caches for various game assets (models, sprites, sounds).

## Key Types & Interfaces
- **`Game`** (`game.go`): The primary struct that owns almost all major subsystem references (`Host`, `Server`, `Client`, `Renderer`, `QC`, `HUD`, etc.).
- **`Renderer`**: An interface used by the game package to interact with the rendering backend without being tied to a specific implementation.
- **`State`** (`state.go`): Represents the snapshot of the world state used for rendering and logic.
- **`RenderFrameState`** (`state.go`): Per-frame render state handed to the renderer.
- **`PendingRendererAssets`**: A queue for assets that need to be uploaded to the renderer during the next frame.

## Sub-packages

Portable leaf logic has been extracted from the root into sub-packages; the root keeps the frame orchestration and the delegators each leaf feeds:

| Sub-package | Owns |
| :--- | :--- |
| `camera` | Cvar-only view computation: view bob/roll, idle sway, viewmodel fudge, view-angle math, chase-camera helpers. |
| `csqc` | Pure CSQC image/rect helpers (nearest palette index, clip draw rect, sub-pic from normalized rect, pic scaling/prep). |
| `audio` | Pure audio helpers (sound name/volume formatting). |
| `ui` | Pure overlay/UI math (clamp, demo name, format helpers). |

Stateful view calculations that latch on the client (`viewCalcGunAngle`, `viewApplyDamageKick`, `viewStairSmoothOffset`) stay on the root because they reference `cl.Client` directly.

## Core Workflow
1. **Startup**: The `New()` function initializes the `Game` instance and its associated caches.
2. **Main Loop**: The `Run()` or `Frame()` methods are called by the `host` package every frame.
3. **Update Cycle**:
    - Process console commands.
    - Update the server (if hosting).
    - Update the client.
    - Synchronize audio.
    - Compute the final camera/view position (`game_camera*.go` in the root and `internal/game/camera`).
4. **Rendering**: The game logic prepares the scene and calls the `Renderer` to draw the frame.

## Integration
- **`host`**: Owns and drives the `Game` instance.
- **`qc`**: The game package manages the QuakeC Virtual Machine (both server-side and client-side CSQC) and handles the interface between Go and QuakeC.
- **`client` & `server`**: The `Game` struct acts as the bridge between the client and server subsystems, especially in single-player mode.
- **`renderer`**: The game package feeds world geometry, entities, and UI elements to the renderer.

## Learning Tips
- **Centralized State**: Examine the `Game` struct in `game.go` to see how all the disparate parts of a Quake engine are wired together.
- **QuakeC Bridge**: Look at `game_runtime_csqc.go` and `game_runtime_ui.go` to understand how client-side QuakeC and UI logic are integrated; the pure helpers behind them live in `internal/game/csqc` and `internal/game/ui`.
- **View Interpolation**: The pure math (`CalcBob`, `CalcRoll`, `AddIdle`, `ApplyViewmodelQuakeFudge`) is in `internal/game/camera/`; the stateful latching view state machines remain in `game_camera_viewcalc.go` at the root.
- **Asset Caching**: Review how `AliasModelCache` and `SoundSFXByIndex` are used to avoid redundant loading of assets during gameplay.
