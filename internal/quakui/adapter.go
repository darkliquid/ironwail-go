package quakui

import "github.com/gogpu/gpucontext"

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
// (ADR-0009, AC7). Its types are gogpu/gpucontext types and plain Go values —
// never internal/game or internal/renderer types. The engine implements this
// interface and calls Run(host).
type Host interface {
	// WorldTexture returns the gpuview texture view the engine renders the
	// world into (research 0006 §2-4). Nil/zero until the gpuview widget is
	// mounted.
	WorldTexture() gpucontext.TextureView

	// RenderIntoWorldTexture retargets the world render (world + entities +
	// polyblend) into the given view. Called by the gpuview OnRender callback
	// (research 0006 §4).
	RenderIntoWorldTexture(view gpucontext.TextureView) error

	// CVar returns the value of an engine cvar as a plain float64. Unknown
	// cvars return 0. The adapter reads only (the engine owns writes).
	CVar(name string) float64

	// KeyDest returns the current input routing destination (plain enum).
	KeyDest() KeyDest

	// ExecuteCommandText queues an engine console command (command text sink).
	ExecuteCommandText(text string)

	// PlaySound plays a local sound effect by name (sound sink).
	PlaySound(name string)
}
