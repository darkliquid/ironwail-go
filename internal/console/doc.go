// Package console implements the Quake text console, including scrollback,
// printing, notifications, and log output.
//
// # Purpose
//
// The package maintains the text buffer that gameplay code prints to and UI
// code later renders as the in-game console. It also tracks notify lines and
// optional debug logging.
//
// # High-level design
//
// Console owns a scrollable text buffer, line-width-dependent layout state,
// notify timestamps, and optional log-file output. It focuses on text storage,
// formatting, resize behavior, and message handling rather than drawing.
//
// # Role in the engine
//
// This is the textual front end for diagnostics, developer output, command
// entry, and configuration feedback.
//
// # Original C lineage
//
// The closest sources are console.c and console.h, along with their ties to
// key handling and screen drawing in the original engine.
//
// # Deviations and improvements
//
// The Go port is less entangled with renderer internals and windowing than the
// original C implementation. Mutex-protected state, UTF-8-aware handling, and
// ordinary file I/O replace globally shared buffers and platform-specific log
// plumbing while preserving Quake's console model.
package console