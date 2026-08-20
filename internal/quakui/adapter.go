package quakui

import (
	"github.com/darkliquid/ironwail-go/internal/input"
)

// Forwarder is the engine-facing input route into the ui widget tree (M1.5,
// ADR-0007). The engine calls ForwardKey/ForwardChar for menu/console input it
// decides the ui should see; gameplay/HUD-only input never reaches it. KeyForwarder
// is the concrete production implementation; tests substitute a recording stub.
// The interface stays in quakui so the engine (internal/game) depends only on
// the narrow route, not the gogpu/ui event translation details.
type Forwarder interface {
	ForwardKey(ev input.KeyEvent, mods input.ModifierState)
	ForwardChar(r rune, mods input.ModifierState)
	ForwardText(text string, mods input.ModifierState)
}

// KeyDest mirrors the engine's input routing destination as a plain enum so
// the adapter does not import internal/input (ADR-0009 isolation boundary).
type KeyDest int

const (
	// KeyDestGame is gameplay-only input (HUD active, no UI input element).
	KeyDestGame KeyDest = iota
	// KeyDestConsole is the dropdown console (captures input).
	KeyDestConsole
	// KeyDestMenu is the menu (captures input).
	KeyDestMenu
)

// Host is the narrow engine-facing adapter that internal/quakui consumes
// (ADR-0009, AC7). Its types are plain Go values — never internal/game or
// internal/renderer types.
type Host interface {
	// CVar returns the value of an engine cvar as a plain float64. Unknown
	// cvars return 0. The adapter reads only (the engine owns writes).
	CVar(name string) float64

	// ExecuteCommandText queues an engine console command (command text sink).
	ExecuteCommandText(text string)

	// PlaySound plays a local sound effect by name (sound sink).
	PlaySound(name string)

	// Quit requests a clean engine shutdown from the ui loop.
	Quit()
}
