# Package `renderer`

## Purpose
The `renderer` package provides the graphics engine for Ironwail-Go. It abstracts the complexities of modern GPU APIs (specifically WebGPU via the `gogpu` library) and provides a unified interface for rendering 3D world geometry, 2D overlays, and special effects like particles and dynamic lighting.

## Key Types & Interfaces
- **`Renderer`**: The main entry point that manages the GPU application window, frame loop, and resource caches (textures, models, shaders).
- **`RenderContext`**: An interface passed to drawing callbacks that provides methods for both 2D (e.g., `DrawPic`, `DrawFill`) and 3D rendering operations.
- **`Backend`**: Defines the lifecycle and window management methods (e.g., `Run`, `Shutdown`, `OnDraw`).
- **`DrawContext`**: The concrete implementation of `RenderContext` for the `gogpu` backend.
- **`WorldRenderData`**: Holds GPU-side resources for a loaded BSP map, including vertex/index buffers and lightmap pages.

## Core Workflow
1. **Setup**: The engine creates a `Renderer` instance, typically via `renderer.New()`, which reads video settings from cvars.
2. **Loop**: The `Renderer.Run()` method starts the main event loop.
3. **Callbacks**: Each frame, the `Renderer` invokes `OnUpdate` (for game logic) followed by `OnDraw` (for rendering).
4. **Scene Rendering**: Inside `OnDraw`, the engine renders the 3D world (BSP), alias models (players/monsters), and sprites.
5. **Compositing**: A 2D overlay phase allows for drawing the HUD and menus. For performance, Ironwail-Go uses an `overlay2D` CPU-side compositor that flushes to the GPU in a single texture upload.

## Integration
- **Host**: The `host` package initializes the renderer and sets up the main callbacks.
- **Client**: The client logic uses the `RenderContext` to draw the game world and the HUD.
- **Image**: The package uses `internal/image` for loading and processing Quake-format images (LMP, PCX).
- **Cvars**: Rendering behavior is heavily influenced by cvars like `r_gamma`, `vid_width`, and `r_dynamic`.

## Learning Tips
- **BSP Rendering**: Study `internal/renderer/world_gogpu.go` to see how the engine handles Quake's complex BSP and lightmap rendering on modern hardware.
- **2D Optimization**: Examine the `overlay2D` struct in `internal/renderer/renderer_gogpu.go` to see how the engine optimizes many small 2D draw calls into a single GPU submission.
- **Dynamic Lighting**: Look at `internal/renderer/dynamic_light_pool.go` for the implementation of Quake's iconic dynamic lights.
