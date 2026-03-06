// Package input provides engine-wide keyboard, mouse, and gamepad handling
// behind a backend abstraction.
//
// # Purpose
//
// The package translates platform events into Quake key codes, text input,
// mouse deltas, bindings, and key-destination routing.
//
// # High-level design
//
// Backend-neutral types and system logic live alongside build-tag-selected
// implementations such as the SDL3 backend and a stub fallback. The package
// models Quake key constants, input destinations like game or menu, modifier
// state, cursor modes, and accumulated per-frame input state.
//
// # Role in the engine
//
// This subsystem feeds client gameplay, console editing, and menu navigation,
// and cooperates with rendering/window code when mouse capture is needed.
//
// # Original C lineage
//
// The closest concepts are keys.c, keys.h, input.h, and SDL/window handling
// paths such as gl_vidsdl.c.
//
// # Deviations and improvements
//
// The Go port uses pure-Go SDL3 bindings and cleaner backend boundaries than
// the original C code, and it treats gamepad support as a first-class part of
// the abstraction. Typed event structs and build-tag-selected backends replace
// raw globals and platform-specific ifdef tangles.
package input