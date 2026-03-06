// Package client implements client-side Quake state, input command generation,
// demo support, and server message parsing.
//
// # Purpose
//
// The package tracks everything the player knows about the running server:
// connection state, signon progress, view angles, stats, entities, sounds,
// particles, temp entities, and locally generated user commands.
//
// # High-level design
//
// Client holds persistent state while parser-oriented code decodes protocol
// messages into that state. The package handles serverinfo, clientdata, entity
// updates, temp entities, sounds, centerprints, signon replies, and demos.
//
// # Role in the engine
//
// The client sits between input/network traffic on one side and HUD, audio,
// and rendering consumers on the other. The host drives it each frame and uses
// it for local loopback play as the port grows toward full parity.
//
// # Original C lineage
//
// The main counterparts are cl_main.c, cl_input.c, cl_parse.c, cl_tent.c, and
// cl_demo.c.
//
// # Deviations and improvements
//
// The Go port keeps stronger package boundaries than the original global-heavy
// client code and expresses state with maps and slices instead of ad hoc arrays
// and pointer arithmetic. Some prediction and renderer-facing behavior are
// still partial, but the protocol/state model is already clearer and easier to
// test.
package client