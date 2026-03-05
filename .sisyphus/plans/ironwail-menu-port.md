# Plan: Ironwail Menu Port

This plan outlines the steps to implement the main menu and basic boot sequence for Ironwail-Go, mirroring the functionality of the original C codebase.

## Status
- [ ] Foundation: 2D Render API
- [ ] Asset Pipeline: gfx.wad
- [ ] Input Wiring
- [ ] Menu Subsystem
- [ ] Boot Sequence

## Tasks

### 1. Foundation: 2D Render API
Extend the renderer to support basic 2D drawing operations required for the menu and HUD.

- [ ] **Task 1.1: Update `RenderContext` interface**
  - File: `internal/renderer/types.go`
  - Add methods:
    - `DrawPic(x, y int, pic *image.QPic)`
    - `DrawFill(x, y, w, h int, color byte)` (Quake palette index)
    - `DrawCharacter(x, y int, num int)`
  - *Testing*: `go build ./internal/renderer` passes (after updating implementations).

- [ ] **Task 1.2: Implement 2D API in OpenGL backend**
  - File: `internal/renderer/renderer_opengl.go`
  - Create a simple GLSL shader for 2D quads with palette lookup.
  - Implement `DrawPic`, `DrawFill`, `DrawCharacter`.
  - *Testing*: Create a temporary test in `internal/renderer/renderer_test.go` that calls these methods.

- [ ] **Task 1.3: Implement 2D API in GoGPU backend**
  - File: `internal/renderer/renderer_gogpu.go`
  - Use `gogpu`'s drawing primitives or a custom pipeline for 2D.
  - *Testing*: Same as Task 1.2 but with `-tags=gogpu`.

### 2. Asset Pipeline: gfx.wad
Load and manage 2D assets from `gfx.wad`.

- [ ] **Task 2.1: Implement `DrawManager`**
  - File: `internal/draw/manager.go`
  - Responsibilities:
    - Load `gfx.wad` using `internal/image/LoadWad`.
    - Cache `QPic` objects.
    - Provide `GetPic(name string) *image.QPic`.
    - Handle `palette.lmp` for color translation.
  - *Testing*: Unit test in `internal/draw/manager_test.go` that mocks a WAD and verifies pic retrieval.

- [ ] **Task 2.2: Texture Uploading**
  - File: `internal/renderer/texture.go`
  - Implement a way to upload `QPic` pixels to GPU textures.
  - Handle transparency (index 255 in Quake palette).
  - *Testing*: Verify textures are created without errors in logs.

### 3. Input Wiring
Connect engine input to the menu system.

- [ ] **Task 3.1: Update `input.System` to handle Menu destination**
  - File: `internal/input/types.go`
  - Ensure `HandleKeyEvent` and `HandleCharEvent` check `s.keyDest == KeyMenu`.
  - Add `OnMenuKey` and `OnMenuChar` callbacks to `input.System`.
  - *Testing*: `go test ./internal/input` passes.

- [ ] **Task 3.2: Wire GLFW/GoGPU events to `input.System`**
  - Files: `internal/renderer/renderer_opengl.go`, `internal/renderer/renderer_gogpu.go`
  - Implement the `Input()` method to return a valid input state.
  - Ensure window events are correctly translated to `input.KeyEvent`.
  - *Testing*: Pressing keys in the window triggers logs in the input system.

### 4. Menu Subsystem
Implement the Quake menu state machine and drawing logic.

- [ ] **Task 4.1: Implement Menu State Machine**
  - File: `internal/menu/menu.go`
  - Define `MenuState` enum (Main, SinglePlayer, MultiPlayer, Options, etc.).
  - Implement `M_Key(key int)` and `M_Draw(dc renderer.RenderContext)`.
  - *Testing*: Can switch between menu states via mock key events.

- [ ] **Task 4.2: Implement Main Menu Drawing**
  - File: `internal/menu/main.go`
  - Implement `M_Main_Draw` using `DrawPic` and `DrawCharacter`.
  - Use the 8px grid system for positioning.
  - *Testing*: Main menu items (Single Player, Multi Player, etc.) appear correctly.

- [ ] **Task 4.3: Implement Quit Logic**
  - File: `internal/menu/quit.go`
  - Implement the "Quit" menu option.
  - Trigger `Host.Stop()` or equivalent when confirmed.
  - *Testing*: Selecting "Quit" closes the application.

### 5. Boot Sequence
Wire everything together to boot into the menu.

- [ ] **Task 5.1: Register Menu Commands**
  - File: `internal/host/commands.go`
  - Register `togglemenu`, `menu_main`, `menu_quit` commands.
  - *Testing*: Commands can be executed from the console.

- [ ] **Task 5.2: Initialize Menu in Host**
  - File: `internal/host/init.go`
  - Add `Menu` to `Subsystems` struct.
  - Initialize menu in `Host.Init`.
  - *Testing*: No initialization errors.

- [ ] **Task 5.3: Trigger Menu on Startup**
  - File: `cmd/ironwailgo/main.go`
  - If no map is provided in args, call `M_ToggleMenu_f`.
  - *Testing*: Engine boots directly to the main menu.



## Implementation Details

### 2D Coordinate System
- Quake uses a virtual 320x200 resolution for the menu, which is then scaled to the actual window size.
- The menu layout is based on an 8x8 pixel grid.

### Key Assets from `gfx.wad`
- `CONCHARS`: 128x128 font texture.
- `MAINMENU`: Main menu background/logo.
- `M_SURF`: Menu cursor/selection bar.
- `P_SINGLE`, `P_MULTI`, `P_LOAD`, `P_SAVE`, `P_OPTION`, `P_QUIT`: Menu item labels.

### Input Routing
- When `KeyDest == KeyMenu`, all keyboard events except `~` (console toggle) should be sent to `M_Key`.
#TR|- Mouse movement should be ignored by the game when the menu is active.
#WY|
#KH|## Final Verification Wave
#QJ|Verify the complete integration and stability of the menu system.
#HQ|
#BJ|- [ ] **Task 6.1: Verify Full Boot Sequence**
#RW|  - Ensure the engine starts, initializes all subsystems, and lands on the main menu without errors.
#BK|  - *Testing*: Run `go run ./cmd/ironwailgo` and observe the boot process.
#XM|
#TT|- [ ] **Task 6.2: Test Menu Navigation**
#RM|  - Verify that keyboard input correctly navigates the menu items and sub-menus.
#XX|  - *Testing*: Use arrow keys and Enter to navigate; Esc to go back.
#SH|
#JQ|- [ ] **Task 6.3: Test Quit Functionality**
#TZ|  - Confirm that the "Quit" option in the menu correctly shuts down the engine.
#JN|  - *Testing*: Select "Quit" and confirm; the process should exit cleanly.
#TR|
#BJ|- [ ] **Task 6.4: Verify 2D Rendering Consistency**
#RW|  - Check that menu elements are correctly scaled and positioned across different window sizes.
#BK|  - *Testing*: Resize the window and verify the menu remains centered and readable.
#XM|
#TT|- [ ] **Task 6.5: Check for Resource Leaks**
#RM|  - Ensure that `gfx.wad` assets are loaded once and properly managed.
#XX|  - *Testing*: Monitor memory usage during extended menu navigation.
