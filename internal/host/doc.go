// Package host coordinates engine startup, shutdown, timing, frame execution,
// and subsystem wiring.
//
// # Purpose
//
// The package is the top-level orchestrator that owns directories, frame
// timing, command processing, server/client sequencing, and subsystem
// lifecycle.
//
// # High-level design
//
// Host drives the main loop and uses small interfaces for filesystem,
// commands, console, audio, client, server, and renderer. That lets the port
// wire together real or stub implementations without hard-coding concrete
// subsystems into one global control block.
//
// # Role in the engine
//
// This is the control center that binds all major subsystems into one running
// executable.
//
// # Original C lineage
//
// The direct counterparts are host.c and host_cmd.c, along with the startup
// ordering concepts traditionally rooted in quakedef.h.
//
// # Deviations and improvements
//
// This package is one of the clearest departures from the original C layout:
// it keeps Quake's host responsibilities but replaces much of the global
// singleton wiring with explicit interfaces, structured init parameters, and
// ordinary error returns. That makes the engine core easier to extend and test.
//
// Recent additions include autosave functionality (automatic quicksave before
// entering new levels, matching C Host_Changelevel_f) and demo playback
// commands with frame-accurate seeking.
package host
