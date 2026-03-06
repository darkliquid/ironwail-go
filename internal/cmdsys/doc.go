// Package cmdsys provides the console command buffer, command registry,
// aliases, and execution helpers.
//
// # Purpose
//
// The package collects command text, tokenizes it in Quake style, expands
// aliases, and dispatches registered handlers by name.
//
// # High-level design
//
// CmdSystem owns command and alias tables plus a buffered stream of pending
// text. It supports queued command text, insertion at the front of the buffer,
// simple semicolon/newline splitting, and prefix-based completion.
//
// # Role in the engine
//
// This is the glue between console input, config scripts, menu actions, host
// commands, and cvar manipulation.
//
// # Original C lineage
//
// The package is the direct Go counterpart of cmd.c and its Quake command
// parsing/buffering model.
//
// # Deviations and improvements
//
// The current Go implementation is intentionally narrower than the original C
// system and omits some wider engine coupling while the port is still in
// progress. Mutex-protected maps, strings.Builder, and plain function callbacks
// replace shared globals, linked command lists, and function-pointer plumbing.
package cmdsys