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
// # Components
//
//   - HUD: Main manager that orchestrates all HUD elements
//   - StatusBar: Renders the bottom-of-screen status bar with health/armor/ammo
//   - Centerprint: Displays temporary centered messages for events
//   - Drawing helpers: DrawNumber and DrawString for text rendering
//
// # Rendering
//
// The HUD uses the renderer.RenderContext interface to draw:
//   - DrawFill: Colored rectangles (bars, backgrounds)
//   - DrawPic: Images from WAD files (status bar background)
//   - DrawCharacter: Individual font glyphs for numbers and text
//
// All coordinates use screen pixel space. The status bar scales from
// Quake's 320-unit virtual coordinate system to modern resolutions.
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
//
// Recent enhancements include numeric displays (DrawNumber), status bar
// background image loading (ibar.lmp), and improved positioning/scaling for
// modern resolutions.
package hud
