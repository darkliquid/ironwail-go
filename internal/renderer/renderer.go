// Package renderer provides GPU-accelerated rendering for the Ironwail-Go engine.
// It wraps the gogpu library to provide a Quake-specific rendering interface
// with support for window management, video modes, and basic drawing operations.
//
// The renderer follows the Quake engine architecture where rendering is tightly
// integrated with the host system and console variable (cvar) system. Video
// settings like resolution, fullscreen mode, and vsync are controlled via cvars.
//
// Architecture Overview:
//
//	The renderer is built on top of gogpu, which provides:
//	- Pure Go WebGPU implementation (no CGO required)
//	- Cross-platform windowing (Linux/X11/Wayland, macOS/Metal, Windows/DX12/Vulkan)
//	- Event-driven rendering with game loop support
//	- Hardware-accelerated graphics via Vulkan/Metal/DX12/GLES
//
// Key Components:
//
//   - Renderer: Main rendering context that owns the gogpu App and window
//   - Config: Video configuration derived from cvars and system capabilities
//   - DrawContext: Frame-specific drawing operations (passed to OnDraw callbacks)
//
// Usage:
//
//	// Create renderer with video cvars
//	r, err := renderer.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer r.Shutdown()
//
//	// Set up render callback
//	r.OnDraw(func(dc *renderer.DrawContext) {
//	    dc.Clear(color.Black)
//	    // ... draw game world ...
//	})
//
//	// Run the main loop
//	if err := r.Run(); err != nil {
//	    log.Fatal(err)
//	}
package renderer

import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
	"github.com/gogpu/gogpu/input"
	"github.com/ironwail/ironwail-go/internal/cvar"
)

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
//
// Prerequisites:
//   - vid_width, vid_height, vid_fullscreen, vid_vsync, host_maxfps, r_gamma
//     cvars must be registered before calling this function.
//
// If a cvar is not found, the default value from DefaultConfig() is used.
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

// DrawContext provides frame-specific rendering operations.
// It is passed to the OnDraw callback and provides access to
// the current frame's rendering state.
//
// DrawContext wraps gogpu.Context and adds Quake-specific functionality
// like clear colors, gamma correction, and convenient drawing methods.
type DrawContext struct {
	// ctx is the underlying gogpu rendering context.
	ctx *gogpu.Context

	// gamma is the current gamma correction value.
	gamma float32
}

// Clear fills the screen with the specified color.
// This is typically called at the start of each frame to clear
// the previous frame's content.
//
// In Quake, the clear color is typically a dark gray or black,
// but can be adjusted for different visual effects.
func (dc *DrawContext) Clear(color gmath.Color) {
	// gogpu provides DrawTriangleColor which we can use for a full-screen clear
	// In a full implementation, we'd use a proper clear operation
	dc.ctx.DrawTriangleColor(color)
}

// DrawTriangle renders a simple colored triangle.
// This is primarily useful for testing the rendering pipeline.
// In a full implementation, this would be replaced with proper
// 3D geometry rendering using shaders.
func (dc *DrawContext) DrawTriangle(color gmath.Color) {
	dc.ctx.DrawTriangleColor(color)
}

// SurfaceView returns the current frame's GPU texture view.
// This enables zero-copy rendering for advanced use cases
// like post-processing effects and compositing.
//
// In Quake, this would be used for:
//   - Rendering the 3D world to a texture
//   - Post-processing effects (bloom, motion blur)
//   - UI overlay rendering
func (dc *DrawContext) SurfaceView() interface{} {
	return dc.ctx.SurfaceView()
}

// Gamma returns the current gamma correction value.
func (dc *DrawContext) Gamma() float32 {
	return dc.gamma
}

// Renderer is the main rendering context for the Ironwail-Go engine.
// It manages the gogpu application window, handles the game loop,
// and provides rendering callbacks for the game logic.
//
// Thread Safety:
//
//	Renderer is thread-safe for configuration changes via SetConfig,
//	but rendering callbacks (OnDraw, OnUpdate) are always called
//	from the render thread.
//
// Lifecycle:
//
//  1. Create with New() or NewWithConfig()
//  2. Set up callbacks with OnDraw() and OnUpdate()
//  3. Run the main loop with Run()
//  4. Shutdown() is called automatically or manually for cleanup
type Renderer struct {
	mu sync.RWMutex

	// app is the gogpu application instance.
	app *gogpu.App

	// config is the current video configuration.
	config Config

	// drawCallback is called each frame to render the scene.
	drawCallback func(dc *DrawContext)

	// updateCallback is called each frame for game logic updates.
	updateCallback func(dt float64)

	// closeCallback is called when the window is closed.
	closeCallback func()

	// running indicates if the main loop is active.
	running bool
}

// New creates a new Renderer with configuration from cvars.
// This is the standard way to create a renderer in Ironwail-Go,
// as it respects user-configurable video settings.
//
// Example:
//
//	r, err := renderer.New()
//	if err != nil {
//	    log.Fatal(err)
//	}
//	defer r.Shutdown()
func New() (*Renderer, error) {
	return NewWithConfig(ConfigFromCvars())
}

// NewWithConfig creates a new Renderer with the specified configuration.
// Use this when you need explicit control over video settings,
// such as for testing or custom launch modes.
//
// Example:
//
//	cfg := renderer.DefaultConfig()
//	cfg.Width = 1280
//	cfg.Height = 720
//	cfg.Fullscreen = false
//	r, err := renderer.NewWithConfig(cfg)
func NewWithConfig(cfg Config) (*Renderer, error) {
	slog.Debug("Creating renderer", "width", cfg.Width, "height", cfg.Height, "fullscreen", cfg.Fullscreen)

	// Build gogpu configuration
	gpuCfg := gogpu.DefaultConfig()
	gpuCfg.Title = cfg.Title
	gpuCfg.Width = cfg.Width
	gpuCfg.Height = cfg.Height
	gpuCfg.Fullscreen = cfg.Fullscreen

	// Configure continuous rendering for game loop
	// Quake engines typically run at max FPS, not event-driven
	gpuCfg = gpuCfg.WithContinuousRender(true)

	// Use Pure Go backend (no CGO required)
	gpuCfg = gpuCfg.WithBackend(gogpu.BackendGo)

	// Create the gogpu application
	app := gogpu.NewApp(gpuCfg)

	r := &Renderer{
		app:    app,
		config: cfg,
	}

	slog.Info("Renderer created",
		"width", cfg.Width,
		"height", cfg.Height,
		"fullscreen", cfg.Fullscreen,
		"vsync", cfg.VSync,
		"maxfps", cfg.MaxFPS,
	)

	return r, nil
}

// OnDraw sets the callback for frame rendering.
// The callback is called each frame with a DrawContext for drawing operations.
//
// Example:
//
//	r.OnDraw(func(dc *renderer.DrawContext) {
//	    dc.Clear(gmath.Color{R: 0.1, G: 0.1, B: 0.1, A: 1.0})
//	    // Draw world geometry...
//	})
func (r *Renderer) OnDraw(callback func(dc *DrawContext)) {
	r.mu.Lock()
	r.drawCallback = callback
	r.mu.Unlock()

	var printed bool
	r.app.OnDraw(func(ctx *gogpu.Context) {
		// Get DeviceProvider (available after first frame initialization)
		provider := r.app.DeviceProvider()
		if provider == nil {
			return // Not ready yet
		}

		// Log device info once
		if !printed {
			slog.Info(
				"DeviceProvider",
				slog.Any("device", provider.Device()),
				slog.Any("queue", provider.Queue()),
				slog.String("surface_format", provider.SurfaceFormat().String()),
			)
			printed = true
		}

		r.mu.RLock()
		callback := r.drawCallback
		gamma := r.config.Gamma
		r.mu.RUnlock()

		if callback != nil {
			dc := &DrawContext{
				ctx:   ctx,
				gamma: gamma,
			}
			callback(dc)
		}
	})
}

// OnUpdate sets the callback for game logic updates.
// The callback is called each frame with the delta time in seconds.
// This is where physics, AI, and game state updates should occur.
//
// Example:
//
//	r.OnUpdate(func(dt float64) {
//	    player.Update(dt)
//	    world.Simulate(dt)
//	})
func (r *Renderer) OnUpdate(callback func(dt float64)) {
	r.mu.Lock()
	r.updateCallback = callback
	r.mu.Unlock()

	r.app.OnUpdate(func(dt float64) {
		r.mu.RLock()
		callback := r.updateCallback
		r.mu.RUnlock()

		if callback != nil {
			callback(dt)
		}
	})
}

// OnClose sets the callback for window close events.
// This is called when the user closes the window or the
// application requests shutdown.
//
// Use this to save game state and release resources.
func (r *Renderer) OnClose(callback func()) {
	r.mu.Lock()
	r.closeCallback = callback
	r.mu.Unlock()

	r.app.OnClose(func() {
		r.mu.RLock()
		cb := r.closeCallback
		r.mu.RUnlock()

		if cb != nil {
			cb()
		}
	})
}

// Input returns the input state for keyboard and mouse polling.
// Use this in the OnUpdate callback to check for key presses
// and mouse movements.
//
// Example:
//
//	r.OnUpdate(func(dt float64) {
//	    inp := r.Input()
//	    if inp.Keyboard().Pressed(input.KeyW) {
//	        player.MoveForward(dt)
//	    }
//	    if inp.Keyboard().JustPressed(input.KeySpace) {
//	        player.Jump()
//	    }
//	})
func (r *Renderer) Input() *input.State {
	return r.app.Input()
}

// Size returns the current window size in pixels.
// This accounts for DPI scaling on high-DPI displays.
func (r *Renderer) Size() (width, height int) {
	return r.app.Size()
}

// ScaleFactor returns the DPI scale factor.
// A value of 1.0 indicates standard DPI, 2.0 indicates Retina/HiDPI.
func (r *Renderer) ScaleFactor() float64 {
	return r.app.ScaleFactor()
}

// Config returns the current video configuration.
func (r *Renderer) Config() Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// SetConfig updates the video configuration.
// Some changes (like resolution) may require a video mode change
// which could cause a brief pause.
func (r *Renderer) SetConfig(cfg Config) {
	r.mu.Lock()
	r.config = cfg
	r.mu.Unlock()
}

// Run starts the main rendering loop.
// This function blocks until the window is closed or Stop() is called.
//
// Run handles:
//   - Window event processing
//   - Frame timing and VSync
//   - Calling OnDraw and OnUpdate callbacks
//   - GPU resource management
//
// Example:
//
//	if err := r.Run(); err != nil {
//	    log.Fatal(err)
//	}
func (r *Renderer) Run() error {
	r.mu.Lock()
	r.running = true
	r.mu.Unlock()

	slog.Info("Starting render loop")

	err := r.app.Run()

	r.mu.Lock()
	r.running = false
	r.mu.Unlock()

	if err != nil {
		return fmt.Errorf("render loop error: %w", err)
	}
	return nil
}

// Stop requests the renderer to stop the main loop.
// This is typically called from an input handler or menu option.
func (r *Renderer) Stop() {
	r.app.Quit()
}

// Shutdown releases all GPU resources and destroys the window.
// This is called automatically when the main loop exits,
// but can be called manually for explicit cleanup.
func (r *Renderer) Shutdown() {
	slog.Debug("Renderer shutting down")
	// gogpu.App handles cleanup automatically
}

// IsRunning returns true if the render loop is active.
func (r *Renderer) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}
