// Package hud renders Quake heads-up overlays such as the status bar and
// centerprint messages.
//
// # Purpose
//
// The package turns client gameplay state into 2D overlay elements drawn over
// the main rendered view.
//
// # High-level design
//
// HUD composes smaller pieces such as StatusBar and Centerprint, tracks screen
// dimensions and currently visible values, and draws through renderer.RenderContext
// using assets provided by draw.Manager.
//
// # Role in the engine
//
// This package sits above client state and draw resources, and below the
// concrete renderer backend that ultimately presents pixels.
//
// # Original C lineage
//
// The closest concepts come from sbar.c, screen.c, and related overlay drawing
// paths in the original Quake UI code.
//
// # Deviations and improvements
//
// The Go port packages HUD behavior separately from the broader screen and
// renderer code instead of letting overlay logic sprawl across many files. It
// currently implements a clean core rather than full Ironwail parity, and uses
// explicit composition plus time.Duration-based centerprint control.
package hud