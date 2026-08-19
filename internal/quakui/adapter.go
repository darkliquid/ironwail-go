package quakui

import (
	"github.com/gogpu/gogpu"
	"github.com/gogpu/gpucontext"
)

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
	// GogpuApp returns the engine's gogpu.App, which desktop.Run takes over
	// as the window/render-loop owner on the ui_backend=1 path (ADR-0006,
	// research 0006 §1).
	GogpuApp() *gogpu.App

	// WorldTexture returns the gpuview texture view the engine renders the
	// world into (research 0006 §2-4). Nil/zero until the gpuview widget is
	// mounted.
	WorldTexture() gpucontext.TextureView

	// RenderIntoWorldTexture retargets the world render (world + entities +
	// polyblend) into the given view. Called by the gpuview OnRender callback
	// (research 0006 §4, M1.4a).
	RenderIntoWorldTexture(view gpucontext.TextureView) error

	// RenderFrame renders the current engine frame (world + entities) into
	// the view most recently set by RenderIntoWorldTexture. Called by the
	// gpuview OnRender callback after RenderIntoWorldTexture; the engine runs
	// its renderer with the gpuview target active and does NOT composite back
	// to the surface.
	RenderFrame() error

	// CVar returns the value of an engine cvar as a plain float64. Unknown
	// cvars return 0. The adapter reads only (the engine owns writes).
	CVar(name string) float64

	// KeyDest returns the current input routing destination (plain enum).
	KeyDest() KeyDest

	// ExecuteCommandText queues an engine console command (command text sink).
	ExecuteCommandText(text string)

	// PlaySound plays a local sound effect by name (sound sink).
	PlaySound(name string)

	// Quit requests a clean engine shutdown from the ui loop.
	Quit()
}
