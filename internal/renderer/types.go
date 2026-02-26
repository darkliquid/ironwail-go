package renderer

import (
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/image"
)
// Package renderer provides GPU-accelerated rendering for the Ironwail-Go engine.
// It supports multiple rendering backends selected at build time via build tags.
//
// Build tags:
//   - gogpu: Uses gogpu library for GPU acceleration (default, no CGO required)
//   - opengl: Uses go-gl/gl for OpenGL rendering (requires CGO)
//   - (no tags): Returns error, requires explicit backend selection
//
// To build with gogpu backend:
//   go build -tags=gogpu ./...
//
// To build with OpenGL backend (requires CGO):
//   go build -tags=opengl ./...
//
// Architecture:
//
//	The renderer uses a backend-agnostic design with three main interfaces:
//	- RenderContext: Frame-specific rendering operations (clear, draw, surface view)
//	- Backend: Manages the graphics backend and window (lifecycle, callbacks, state)
//	- Core: Manages GPU resources (headless capable, adapter info, frame data)
//
// Pure Go game logic (screen, particle, surface, model packages) remains untagged
// and works with any backend implementation.
//
// Platform-specific requirements for OpenGL:
//   - Linux: apt-get install libglfw3-dev libgl1-mesa-dev
//   - macOS: No additional dependencies (OpenGL framework available)
//   - Windows: No additional dependencies (OpenGL available)
// RenderContext provides frame-specific rendering operations.
// It is passed to the OnDraw callback and provides access to
// the current frame's rendering state.
type RenderContext interface {
	// Clear fills the screen with the specified color.
	Clear(r, g, b, a float32)

	// DrawTriangle renders a simple colored triangle.
	// This is primarily useful for testing the rendering pipeline.
	DrawTriangle(r, g, b, a float32)

	// SurfaceView returns the current frame's GPU texture view.
	// This enables zero-copy rendering for advanced use cases like
	// post-processing effects and compositing.
	SurfaceView() interface{}

	// Gamma returns the current gamma correction value.
	Gamma() float32

	// 2D Drawing API for menus and HUD

	// DrawPic renders a QPic image at the specified position.
	DrawPic(x, y int, pic *image.QPic)

	// DrawFill fills a rectangle with a Quake palette color.
	DrawFill(x, y, w, h int, color byte)

	// DrawCharacter renders a single character from the font.
	DrawCharacter(x, y int, num int)
}

// Backend manages the graphics backend and window.
// It handles the main game loop, callbacks, and window state.
type Backend interface {
	// Lifecycle
	New(cfg Config) error
	Shutdown()
	Run() error
	Stop()

	// Callbacks
	OnDraw(func(RenderContext))
	OnUpdate(func(dt float64))
	OnClose(func())

	// Input returns the input state for keyboard and mouse polling.
	Input() interface{}

	// State
	Size() (width, height int)
	ScaleFactor() float64
	Config() Config
	SetConfig(Config)
	IsRunning() bool
}


// Video-related console variable names used by the renderer.
// These cvars must be registered by the host application before
// creating a Renderer instance.
const (
	CvarVidWidth      = "vid_width"      // Video width in pixels (default: 1920)
	CvarVidHeight     = "vid_height"     // Video height in pixels (default: 1080)
	CvarVidFullscreen = "vid_fullscreen" // Fullscreen mode: 0=windowed, 1=fullscreen (default: 1)
	CvarVidVsync      = "vid_vsync"      // Vertical sync: 0=off, 1=on (default: 1)
	CvarHostMaxFPS    = "host_maxfps"    // Maximum frames per second (default: 250)
	CvarRGamma        = "r_gamma"        // Gamma correction value (default: 1.0)
)

// Config holds the video configuration for the renderer.
// It is initialized from console variables and can be modified
// at runtime to change video modes.
type Config struct {
	// Width is the window/surface width in pixels.
	Width int

	// Height is the window/surface height in pixels.
	Height int

	// Fullscreen determines whether the window is borderless fullscreen.
	Fullscreen bool

	// VSync enables vertical synchronization to prevent tearing.
	VSync bool

	// MaxFPS limits the frame rate when VSync is disabled.
	MaxFPS int

	// Gamma controls the overall brightness/gamma correction.
	// Values >1.0 brighten the image, <1.0 darken it.
	Gamma float32

	// Title is the window title displayed in the title bar.
	Title string
}

// DefaultConfig returns a Config with sensible defaults for a modern game.
// These defaults can be overridden by cvars after registration.
func DefaultConfig() Config {
	return Config{
		Width:      1920,
		Height:     1080,
		Fullscreen: true,
		VSync:      true,
		MaxFPS:     250,
		Gamma:      1.0,
		Title:      "Ironwail-Go",
	}
}

// ConfigFromCvars creates a Config by reading values from registered cvars.
// This allows video settings to be controlled via the console and config files.
func ConfigFromCvars() Config {
	cfg := DefaultConfig()

	if cv := cvar.Get(CvarVidWidth); cv != nil {
		cfg.Width = cv.Int
	}
	if cv := cvar.Get(CvarVidHeight); cv != nil {
		cfg.Height = cv.Int
	}
	if cv := cvar.Get(CvarVidFullscreen); cv != nil {
		cfg.Fullscreen = cv.Bool()
	}
	if cv := cvar.Get(CvarVidVsync); cv != nil {
		cfg.VSync = cv.Bool()
	}
	if cv := cvar.Get(CvarHostMaxFPS); cv != nil {
		cfg.MaxFPS = cv.Int
	}
	if cv := cvar.Get(CvarRGamma); cv != nil {
		cfg.Gamma = cv.Float32()
	}

	return cfg
}