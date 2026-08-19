# Ironwail Go: Architecture & Learning Guide

Welcome to the Ironwail Go codebase! This guide is designed to help you navigate and learn the engine, whether you're an experienced developer or a newcomer to game engines and graphics programming.

## Core Mental Model: The Client-Server Architecture

The most important thing to understand about the Quake engine (and thus Ironwail Go) is that it is fundamentally a **client-server application**, even when playing single-player.

- **The Server** is the "source of truth." It runs the physics, executes the game logic (QuakeC), and decides what happens in the world.
- **The Client** is a "dumb terminal" with a lot of predictive polish. It gathers your input, sends it to the server, and renders the state updates it receives back.

In single-player, these two components run in the same process and talk over a high-speed "loopback" connection. In multiplayer, they talk over the network.

---

## 🗺️ Codebase Map: Where is everything?

The project is organized into `internal/` packages, each with a specific responsibility.

| Package | Responsibility | Analogous to... | Guide |
| :--- | :--- | :--- | :--- |
| `cmd/ironwailgo` | **Entry Point & Wiring.** This is where everything is initialized and tied together. | The "glue" logic. | N/A |
| `internal/async` | **Async Coordination.** Marshalling background tasks to the main thread. | Task Queue | [Guide](internal/async.md) |
| `internal/audio` | **The Soundscape.** Mixing, spatialization, and backend abstractions. | Audio Mixer | [Guide](internal/audio.md) |
| `internal/bsp` | **The Map.** Parsing world geometry and spatial data. | World Loader | [Guide](internal/bsp.md) |
| `internal/client` | **The Player View.** Parses server messages, predicts movement, and manages local state. | The "front-end." | [Guide](internal/client.md) |
| `internal/cmdsys` | **The Command System.** Registry for console commands and alias expansion. | Command Shell | [Guide](internal/cmdsys.md) |
| `internal/common` | **Utilities.** Shared types and helper functions. | Utility Belt | [Guide](internal/common.md) |
| `internal/compatrand` | **Deterministic Random.** Random number generation with parity. | Dice Roller | [Guide](internal/compatrand.md) |
| `internal/console` | **The Console.** Command input, logging, and completion. | Terminal | [Guide](internal/console.md) |
| `internal/cvar` | **Console Variables.** Persistent configuration and flags. | Settings Registry | [Guide](internal/cvar.md) |
| `internal/draw` | **Drawing Primitives.** Font rendering and 2D UI drawing. | Painter | [Guide](internal/draw.md) |
| `internal/engine` | **Lifecycle.** Core data structures (Cache, Registry, Set, Queue, EventBus). | Engine Heart | [Guide](internal/engine.md) |
| `internal/fs` | **The Filesystem.** Handles `.pak` files and virtual paths (`id1/`, etc). | The "data loader." | [Guide](internal/fs.md) |
| `internal/game` | **Coordinator.** Central game state and loop management. | The Director | [Guide](internal/game.md) |
| `internal/game/camera` | **View Calculation.** Cvar-only view bob/roll, idle sway, viewmodel fudge, chase-camera math. | Camera Math | — |
| `internal/game/csqc` | **CSQC Helpers.** Pure image/rect helpers for client-side QuakeC drawing. | CSQC Brush | — |
| `internal/game/audio` | **Audio Helpers.** Pure sound name/volume formatting. | Audio Format | — |
| `internal/game/ui` | **UI Math.** Pure overlay/UI clamp + format helpers. | UI Brush | — |
| `internal/host` | **The Scheduler.** Manages the main loop, timing, and session lifecycle. | The "orchestra conductor." | [Guide](internal/host.md) |
| `internal/hud` | **The HUD.** In-game overlays, status bars, and centerprints. | Heads-up Display | [Guide](internal/hud.md) |
| `internal/image` | **Graphics Processing.** Image parsing and palette handling. | Image Processor | [Guide](internal/image.md) |
| `internal/input` | **The Senses.** Normalizes keyboard/mouse/gamepad into engine commands. | The "input translator." | [Guide](internal/input.md) |
| `internal/menu` | **The Menus.** State machine for navigation and game options. | UI System | [Guide](internal/menu.md) |
| `internal/quakeui` | **gogpu/ui Host.** `ui_backend` gate, widget host, input gateway, canvas bridge, surface stacking. | UI Host | — |
| `internal/quakeui/theme` | **Quake Theme.** Palette tokens + ThemeExtension + pic bridge. | UI Theme | — |
| `internal/quakeui/widgets` | **Quake Widgets.** QuakeText (conchars atlas) + custom widgets. | UI Widgets | — |
| `internal/quakeui/menu` | **Menu Widget.** gogpu/ui menu root + page row models (path 1). | UI Menu | — |
| `internal/quakeui/console` | **Console Widget.** Scrollback + input + completion bridge (path 1). | UI Console | — |
| `internal/quakeui/hud` | **HUD Widgets.** Status bar, crosshair, centerprint (path 1). | UI HUD | — |
| `internal/model` | **Assets.** Loader for MDL, SPR, and alias models. | Asset Manager | [Guide](internal/model.md) |
| `internal/mods` | **Addons.** Addon downloader and installation system. | Mod Loader | [Guide](internal/mods.md) |
| `internal/net` | **Networking.** Low-level transport and protocol handling. | Network Stack | [Guide](internal/net.md) |
| `internal/qc` | **The Game Rules.** A Virtual Machine that runs QuakeC (`progs.dat`) bytecode. | The "scripting engine." | [Guide](internal/qc.md) |
| `internal/renderer` | **The Visuals.** Uses WebGPU to draw the world, models, and UI. | The "painting engine." | [Guide](internal/renderer.md) |
| `internal/renderer/alias` | **Alias Models.** MDL vertex interpolation and rotation math (CPU path). | Model Animator | — |
| `internal/renderer/gogpu` | **Input Mapping.** Translates gogpu input events to Quake key codes. | Input Bridge | — |
| `internal/renderer/lightmap` | **Lightmap Atlas.** CPU compositing, dirty tracking, page stacking. | Lightmap Painter | — |
| `internal/renderer/decal` | **Decal Marks.** Mark lifetime, quad geometry, atlas seeding. | Decal System | — |
| `internal/renderer/particle` | **Particle Math.** Shared particle state/geometry helpers. | Particle Helpers | — |
| `internal/renderer/pipeline` | **Pipeline Constructors.** World render-pipeline + layout builders. | Pipeline Factory | — |
| `internal/renderer/scrap` | **Scrap Atlas.** Skyline bin-packing for small UI textures. | Texture Packer | — |
| `internal/renderer/sky` | **External Skybox.** Loads and renders 6-face PNG skyboxes. | Skybox Manager | — |
| `internal/renderer/surface` | **Lightmap Allocator.** 2D bin-packing for lightmap samples. | Lightmap Packer | — |
| `internal/renderer/world` | **Shared World Types.** WorldVertex, WorldFace, WorldGeometry, fog math, BSP texture helpers. | World Data Layer | — |
| `internal/renderer/world/gogpu` | **GoGPU World Helpers.** Brush entity building, vertex/alias byte packing, resource creation, WGSL shaders. | Shader Source | — |
| `internal/renderer/oit` | **OIT Transparency.** Order-independent transparency config (currently disabled). | Transparency Config | — |
| `internal/renderer/warpscale` | **Water Warp.** Scene render target, FOV warp, composite pass. | Post-Process | — |
| `internal/server` | **The Truth.** Facade/orchestrator: runs the session, QC hooks, and delegates to sub-packages for physics, collision, and wire encoding. | The "simulation engine." | [Guide](internal/server.md) |
| `internal/server/physics` | **Physics Engine.** `physics.System` leaf algorithms + frame loop (`StepFrame`), client move simulation. | Physics Simulator | — |
| `internal/server/collision` | **Collision System.** BSP model builders, hulls, areanodes, trace queries. | Collision Detect | — |
| `internal/server/net` | **Wire Encoding.** Entity delta encoding, client-data bit packing, signon writers. | Protocol Encoder | — |
| `internal/server/edict` | **Edict Pool.** Entity allocation, map/savegame parsing, spatial unlink. | Entity Manager | — |
| `internal/server/state` | **Session/Signon.** Server session + signon stage manager. | Session Manager | — |
| `internal/server/commands` | **Command Dispatch.** Server console command parse/dispatch. | Command Parser | — |
| `internal/server/debug` | **Server Debug.** Telemetry + svdbg logging. | Debug Logger | — |
| `internal/server/qc` | **QC Hooks.** Server-side QuakeC hook helpers. | QC Bridge | — |
| `internal/server/savegame` | **Save/Load.** Save-game serialization. | Save Manager | — |
| `internal/server/types` | **Server Types + Seams.** Edict, Client, MessageBuffer, EntityState, protocol stages, and the narrow interfaces (PhysicsFacade, FrameDriver, CollisionWorld) sub-packages depend on. | Wire Types | — |
| `internal/testutil` | **Testing.** Utilities for synthetic assets and integration tests. | Test Harness | [Guide](internal/testutil.md) |

---

> **Test Coverage Docs**: Many package guides above include a **Tests** section that documents notable tests in that package — what they verify, why they matter, and how they achieve their goal. Coverage is still being expanded, so some guides may not have this section yet. Where present, it is a good place to start when you want to understand the expected behavior of a subsystem without reading production code first.

---

## 🚀 Key Feature Deep-Dives

### 1. The Command System (`internal/cmdsys`)
Everything in the engine is a command. When you press a key, it's often bound to a command (e.g., `+forward`). When you click a menu item, it usually just executes a command (e.g., `map start`).

- **Why it matters:** It provides a unified way to control the engine from the console, scripts (`quake.rc`), and the UI.
- **Where to look:** `internal/cmdsys/cmdsys.go` for the core, and `cmd/ironwailgo/game_commands.go` for game-specific commands.

### 2. QuakeC & The VM (`internal/qc`)
Many people are surprised to find that "how the shotgun works" isn't in the Go code. It's in the QuakeC progs. The Go engine provides **builtins** (like "how to trace a line") that the QuakeC code calls.

- **Why it matters:** This separation allowed original Quake modders to change the game without having the C source code. In this port, it keeps the engine "clean" of specific game rules.
- **Where to look:** `internal/qc/vm.go` for the interpreter, and `internal/qc/builtins.go` for the functions Go provides to the scripts.

### 3. Rendering with WebGPU (`internal/renderer`)
Ironwail Go uses modern WebGPU primitives (via the `gogpu` library). Unlike older engines that might draw things one-by-one, modern renderers try to "batch" work to the GPU.

- **Key concept: The BSP.** Quake uses Binary Space Partitioning to quickly figure out which parts of a map are visible so it doesn't waste time drawing what you can't see.
- **Where to look:** `internal/renderer/renderer_gogpu_frame.go` for the frame orchestrator (`RenderFrame`), `internal/renderer/renderer_gogpu_world_render.go` for the world BSP render pass, and `internal/bsp` for map loading logic.
- **Renderer sub-packages:** The renderer is split into focused sub-packages:
  - `renderer/alias` — MDL vertex interpolation (CPU path, C: r_alias.c)
  - `renderer/world` — shared world types (WorldVertex, WorldFace, WorldGeometry)
  - `renderer/world/gogpu` — GoGPU brush entity building, sprite uniforms, WGSL shaders
  - `renderer/surface` — lightmap allocator (2D bin-packing, C: gl_rsurf.c)
  - `renderer/lightmap` — CPU lightmap compositing, dirty tracking, page stacking
  - `renderer/decal` — decal mark lifetime, quad geometry, atlas seeding
  - `renderer/pipeline` — world render-pipeline + layout constructors
  - `renderer/scrap` — scrap atlas (skyline bin-packing for UI textures)
  - `renderer/sky` — external skybox loading and rendering (C: gl_sky.c)
  - `renderer/gogpu` — input device mapping (keyboard/mouse to Quake keys)
  - `renderer/warpscale` — water warp post-processing and scene render target
  - `renderer/oit` — order-independent transparency config (currently disabled)
- **Learning the renderer from scratch:** If you are new to graphics programming or WebGPU, see `docs/RENDERER_LEARNING_PLAN.md` — a stage-by-stage curriculum with external citations and build-it-yourself milestones.
- **Debug tools:** See the `ironwail-debugging` skill or `docs/plans/qbj2_zetabyt_diagnostic.md` for a complete guide to GPU validation, debug shaders, and diagnostic tools.

---

## 🛤️ Learning Paths: Walkthroughs

We have several guided walkthroughs that follow specific actions through the entire stack. We recommend reading them in this order:

1.  [**Boot to Menu**](docs/WALKTHROUGH_BOOT_TO_MENU.md)
    *   *What you'll learn:* How the dependency graph is built and how the engine starts up.
2.  [**Start a New Game**](docs/WALKTHROUGH_SINGLEPLAYER_FORWARD.md)
    *   *What you'll learn:* How a local server is spawned and how the first movement key turns into authoritative physics.
3.  [**Multiplayer Combat**](docs/WALKTHROUGH_MULTIPLAYER_SHOOT.md)
    *   *What you'll learn:* How networking works, and how the Go engine and QuakeC VM collaborate to handle a "shot."

---

## 🛠️ Essential Tools for Exploration

- **`mise tasks`**: Run this in your terminal to see all available tasks (build, test, parity checks).
- **`host_speeds 1`**: Type this in the in-game console to see real-time timing info for different engine parts.
- **`sv_debug_telemetry 1`**: Enables detailed server logging to see what entities and QuakeC logic are doing.
- **`profile`**: Prints the top 10 most expensive QuakeC functions.

---

## 💡 Tips for Novices

- **Follow the `+` commands.** If you want to see how jumping works, search the code for `"+jump"`.
- **Look at the Edicts.** An "Edict" is just an entity. Most gameplay involves changing fields on edicts (like `origin`, `health`, or `velocity`).
- **Use the parity guide.** `docs/PARITY.md` explains how we compare against the original C code and track current parity gaps.
- **Trace a network message.** Look at `internal/client/parse.go` to see how the client takes a bunch of bytes from the server and turns them into something you can see on screen.
