//go:build gogpu
// +build gogpu

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

// DrawContext provides frame-specific rendering operations.
import (
	"fmt"
	"log/slog"
	"sync"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
	"github.com/gogpu/gogpu/input"
	"github.com/ironwail/ironwail-go/internal/image"
)

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

// Clear fills the screen with the specified RGBA color.
// This is typically called at the start of each frame to clear
// the previous frame's content.
//
// In Quake, the clear color is typically a dark gray or black,
// but can be adjusted for different visual effects.
func (dc *DrawContext) Clear(r, g, b, a float32) {
	// gogpu provides DrawTriangleColor which we can use for a full-screen clear
	// In a full implementation, we'd use a proper clear operation
	color := gmath.Color{R: r, G: g, B: b, A: a}
	dc.ctx.DrawTriangleColor(color)
}

// DrawTriangle renders a simple colored triangle.
// This is primarily useful for testing the rendering pipeline.
// In a full implementation, this would be replaced with proper
// 3D geometry rendering using shaders.
func (dc *DrawContext) DrawTriangle(r, g, b, a float32) {
	color := gmath.Color{R: r, G: g, B: b, A: a}
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

// 2D Drawing API implementation

// DrawPic renders a QPic image at the specified position.
// TODO: This is a simplified implementation that needs proper texture support.
func (dc *DrawContext) DrawPic(x, y int, pic *image.QPic) {
	// For now, just log the call
	slog.Debug("DrawPic called", "x", x, "y", y, "pic", pic.Width, "x", pic.Height)
	// TODO: Implement proper texture rendering with palette lookup using gogpu
}

// DrawFill fills a rectangle with a Quake palette color.
// TODO: This is a simplified implementation that needs proper palette support.
func (dc *DrawContext) DrawFill(x, y, w, h int, color byte) {
	// For now, just log the call
	// TODO: Implement proper 2D quad rendering using gogpu's drawing primitives
	slog.Debug("DrawFill called", "x", x, "y", y, "w", w, "h", h, "color", color)
}

// DrawCharacter renders a single character from the font.
// TODO: This is a simplified implementation that needs proper font texture support.
func (dc *DrawContext) DrawCharacter(x, y int, num int) {
	// For now, just log the call
	// TODO: Implement proper character rendering from CONCHARS texture using gogpu
	slog.Debug("DrawCharacter called", "x", x, "y", y, "char", num)
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
	drawCallback func(dc RenderContext)
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
	gogpu.SetLogger(slog.Default())
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
func (r *Renderer) OnDraw(callback func(dc RenderContext)) {
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
