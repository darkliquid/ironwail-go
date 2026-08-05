// Package main provides the entry point binary for the Ironwail Go Quake engine port.
//
// # Purpose
//
// Construct and execute the main engine lifecycle, parsing command-line flags,
// initializing logging and performance profiling, setting up working directories,
// and invoking the core game coordinator (internal/game).
//
// # Original C lineage
//
// Mirrored from original Quake / Ironwail C sources:
//   - sys_sdl.c: OS entry point, main loop invocation, command line parsing.
//   - host.c: Host initialization and system shutdown sequence.
//
// # Role in the engine
//
// This is the top-level binary entry point constructed when building `ironwailgo`.
// It handles startup flags (-basedir, -game, -width, -height, -window, -headless,
// -screenshot, -loglvl, -pprof), configures slog logging handlers, and hands control
// over to internal/game.Game.Run().
//
// # Key flags
//
//   - `-basedir <dir>`: Sets the root Quake data directory containing id1/ pak files.
//   - `-game <dir>`: Specifies a game mod directory (e.g. hipnotic, rogue).
//   - `-headless`: Runs the engine without opening a WebGPU rendering window (used for CI and smoke tests).
//   - `-screenshot <path>`: Renders a single frame to a PNG screenshot and exits.
//   - `-pprof`: Enables CPU/memory pprof profiling servers.
//
// # Testing
//
//	mise run build
//	mise run run
package main
