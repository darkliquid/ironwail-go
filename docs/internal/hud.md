# Package: hud

## Purpose
The `hud` package is responsible for rendering the 2D Heads-Up Display overlays during gameplay. It translates the internal game state (health, ammo, items) into visual elements like the status bar, crosshair, and centerprint messages. It supports multiple HUD styles, including the classic Quake layout and modern, compact variations.

## Key Types & Interfaces
- **`HUD`**: The main manager that orchestrates the drawing of all HUD components.
- **`StatusBar`**: (In `status.go`) Handles the rendering of the bottom-of-screen inventory and status strip.
- **`Centerprint`**: (In `centerprint.go`) Manages temporary centered messages (e.g., "You got the Silver Key").
- **`Crosshair`**: (In `crosshair.go`) Renders the player's aiming reticle.
- **`State`**: A data transfer object (DTO) that contains all the necessary client state (health, ammo, weapon model, etc.) needed for rendering the HUD.
- **`HUDStyle`**: An enum that selects between Classic, Modern Center, Modern Side, or QuakeWorld layouts.

## Core Workflow
1. **State Update**: Every frame, the game logic gathers the relevant player data and calls `SetState()` on the `HUD`.
2. **Layout Selection**: The `Draw()` method checks the `hud_style` CVar to determine which layout to render.
3. **Canvas Setup**: The HUD uses the `renderer.RenderContext` to set the appropriate canvas (e.g., `CanvasSbar` for the status bar, `CanvasCrosshair` for the crosshair).
4. **Drawing**:
    - The `StatusBar` draws backgrounds and icons.
    - `DrawNumber` and `DrawString` (in `drawing.go`) render numeric and text data using character glyphs from the game's font.
    - `Centerprint` messages are managed with a timer to fade out after their duration expires.

## Integration
- **`game`**: The game package owns the `HUD` instance and feeds it state updates every frame.
- **`renderer`**: The HUD package uses the `RenderContext` interface for all its drawing operations (`DrawPic`, `DrawCharacter`, `DrawFill`).
- **`draw`**: Provides the UI assets (WAD images, font characters) that the HUD renders.
- **`cvar`**: The HUD's behavior (style, scale, crosshair type) is controlled by various console variables.

## Learning Tips
- **Canvas Transforms**: Look at `setHUDCanvasParams` in `hud.go` to see how the engine handles scaling and positioning for different screen resolutions and HUD styles.
- **Numeric Rendering**: Examine `DrawNumber` in `drawing.go` to see how Quake's iconic red and gold numbers are constructed from individual sprite characters.
- **Style Composition**: See how `hud_style` affects the logic in `HUD.Draw()` to switch between entirely different layout implementations.
- **Classic Status Bar**: Read `status.go` to see how the complex, multi-layered classic Quake status bar is faithfully reconstructed in Go.
