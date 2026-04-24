// Copyright (C) 2024 Ironwail Go Port Authors
// SPDX-License-Identifier: GPL-2.0-or-later

// Package ironwail provides a Go implementation of the Ironwail Quake engine.
//
// Ironwail is a high-performance fork of QuakeSpasm, featuring modern OpenGL
// rendering techniques, improved physics handling, and enhanced mod support.
// This Go port aims to provide:
//
//   - Idiomatic Go code with extensive documentation
//   - Modular, testable architecture
//   - Modern rendering via WebGPU
//   - Pure Go implementation (no CGO) for Linux
//
// # Architecture Overview
//
// The engine follows the classic Quake architecture with some modernizations:
//
//	┌─────────────────────────────────────────────────────────────┐
//	│                         Host                                 │
//	│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐  │
//	│  │    Server    │  │    Client    │  │     Renderer     │  │
//	│  │              │  │              │  │                  │  │
//	│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────────┐ │  │
//	│  │ │QuakeC VM │ │  │ │  Input   │ │  │ │   WebGPU     │ │  │
//	│  │ └──────────┘ │  │ └──────────┘ │  │ └──────────────┘ │  │
//	│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────────┐ │  │
//	│  │ │  Physics │ │  │ │  Parse   │ │  │ │    BSP       │ │  │
//	│  │ └──────────┘ │  │ └──────────┘ │  │ └──────────────┘ │  │
//	│  │ ┌──────────┐ │  │ ┌──────────┐ │  │ ┌──────────────┐ │  │
//	│  │ │   World  │ │  │ │  Demo    │ │  │ │   Models     │ │  │
//	│  │ └──────────┘ │  │ └──────────┘ │  │ └──────────────┘ │  │
//	│  └──────────────┘  └──────────────┘  └──────────────────┘  │
//	│                                                              │
//	│  ┌────────────────────────────────────────────────────────┐ │
//	│  │                   Shared Systems                        │ │
//	│  │  Console │ CVar │ Command │ FileSystem │ Network       │ │
//	│  └────────────────────────────────────────────────────────┘ │
//	└─────────────────────────────────────────────────────────────┘
//
// # Core Systems
//
// The engine is organized into several major subsystems:
//
// ## Host (internal/host)
//
// The host is the central coordinator that manages the main game loop,
// initialization, shutdown, and inter-system communication. It runs
// at a configurable frame rate (default 250 FPS) and coordinates:
//   - Server physics ticks (72 Hz for network play)
//   - Client input accumulation and command sending
//   - Renderer frame presentation
//   - Audio mixing
//
// ## Server (internal/server)
//
// The server manages the game world simulation:
//   - Entity physics (movement, collision)
//   - QuakeC VM execution for game logic
//   - World spatial partitioning (area nodes)
//   - Client connection handling
//   - Network message broadcasting
//
// ## Client (internal/client)
//
// The client handles player interaction:
//   - Input processing (keyboard, mouse)
//   - Server message parsing
//   - Prediction and interpolation
//   - Demo playback and recording
//   - Local entity management
//
// ## QuakeC VM (internal/qc)
//
// QuakeC is the scripting language used for game logic. The VM:
//   - Loads compiled .dat progs files
//   - Executes bytecode instructions
//   - Provides builtin functions for engine integration
//   - Manages entity field access
//
// ## Renderer (internal/renderer)
//
// The rendering pipeline handles visual output:
//   - BSP world geometry
//   - Alias models (characters, items)
//   - Sprite rendering
//   - Particle effects
//   - Dynamic lighting
//   - Texture management
//
// # Quake Engine Concepts
//
// ## Edicts
//
// An "edict" is Quake's term for a game entity. Edicts contain:
//   - Physics state (origin, velocity, angles)
//   - Rendering state (model, frame, skin)
//   - Game state (health, items, flags)
//   - QuakeC fields (defined by the progs)
//
// ## BSP Trees
//
// Binary Space Partitioning trees organize world geometry for:
//   - Efficient visibility determination (PVS)
//   - Collision detection (hull tracing)
//   - Spatial queries (point contents, area searches)
//
// ## Lightmaps
//
// Quake uses precomputed lighting stored in lightmaps:
//   - 16x16 texel blocks per surface
//   - Multiple light styles for animated effects
//   - Blended with textures during rendering
//
// ## Networking
//
// The original Quake protocol uses:
//   - Unreliable datagrams for position updates
//   - Reliable messages for important state
//   - Delta compression for bandwidth efficiency
//   - Client-side prediction for responsiveness
//
// # Package Organization
//
// The codebase follows standard Go project layout:
//
//	pkg/                    # Public packages (importable)
//	  types/                # Core type definitions (Vec3, Plane, etc.)
//
//	internal/               # Private implementation packages
//	  host/                 # Main game loop and coordination
//	  server/               # Server-side game logic
//	  client/               # Client-side processing
//	  qc/                   # QuakeC virtual machine
//	  bsp/                  # BSP map loader
//	  model/                # Model loaders (MDL, SPR)
//	  renderer/             # Rendering pipeline
//	  audio/                # Sound mixing
//	  input/                # Input handling
//	  net/                  # Networking
//	  console/              # Console system
//	  cvar/                 # Console variables
//	  cmdsys/               # Command system
//	  fs/                   # Virtual filesystem
//	  common/               # Shared utilities
//	  async/                # Asynchronous task coordination
//	  compatrand/           # Compatibility-focused random number generation
//	  draw/                 # Core drawing and font primitives
//	  engine/               # Core engine lifecycle and services
//	  game/                 # High-level gameplay state and logic
//	  hud/                  # Heads-up display and in-game overlays
//	  image/                # Image loading and manipulation
//	  menu/                 # Main menu and UI systems
//	  mods/                 # Mod loading and management support
//
//	cmd/                    # Main applications
//	  ironwailgo/           # Main game executable
//
// # Usage Example
//
// Basic engine initialization:
//
//	func main() {
//	    // Initialize core systems
//	    console.InitGlobal(0)
//
//	    // Create host (which owns the cvar registry)
//	    h := host.NewHost()
//	    h.CVar.Register("host_maxfps", "250", cvar.FlagArchive, "Maximum FPS")
//	    h.SetBaseDir("/path/to/quake")
//	    h.SetGameDir("id1")
//
//	    // Initialize
//	    if err := h.Init(); err != nil {
//	        log.Fatal(err)
//	    }
//	    defer h.Shutdown()
//
//	    // Run main loop
//	    h.Run()
//	}
//
// # Performance Considerations
//
// The original Ironwail achieves high performance through:
//   - GPU-based culling (compute shaders)
//   - Instanced rendering
//   - Indirect multi-draw
//   - Persistent buffer mapping
//   - Bindless textures
//
// This Go port aims to leverage similar techniques via WebGPU while
// maintaining code clarity and idiomatic Go patterns.
//
// # References
//
//   - Quake Source Code: https://github.com/id-Software/Quake
//   - QuakeSpasm: https://sourceforge.net/projects/quakespasm/
//   - Ironwail: https://github.com/andrei-drexler/ironwail
//   - Quake Wiki: https://quakewiki.org/
//   - Unofficial Quake Specs: https://www.gamers.org/dEngine/quake/spec/
package ironwail
