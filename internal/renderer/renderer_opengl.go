//go:build opengl
// +build opengl

package renderer

import (
	"fmt"
	"log/slog"
	"runtime"
	"sync"
	"time"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/ironwail/ironwail-go/internal/image"
)
func init() {
	// OpenGL must run on main OS thread
	runtime.LockOSThread()
}

type glDrawContext struct {
	window   *glfw.Window
	gamma    float32
	viewport struct {
		width  int
		height int
	}
}

func (dc *glDrawContext) Clear(r, g, b, a float32) {
	gl.ClearColor(r, g, b, a)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
}

func (dc *glDrawContext) DrawTriangle(r, g, b, a float32) {
	// TODO: Implement modern OpenGL triangle rendering with shaders and VBOs
	// For now, just clear to the triangle color as a simple test
	gl.ClearColor(r, g, b, a)
	gl.Clear(gl.COLOR_BUFFER_BIT)
}

func (dc *glDrawContext) SurfaceView() interface{} {
	// In a full implementation, this would return an OpenGL texture view
	return nil
}

func (dc *glDrawContext) Gamma() float32 {
	return dc.gamma
}

// 2D Drawing API implementation

// DrawPic renders a QPic image at the specified position.
// TODO: This is a simplified implementation that needs proper texture support.
func (dc *glDrawContext) DrawPic(x, y int, pic *image.QPic) {
	// For now, just log the call
	slog.Debug("DrawPic called", "x", x, "y", y, "pic", pic.Width, "x", pic.Height)
	// TODO: Implement proper texture rendering with palette lookup
}

// DrawFill fills a rectangle with a Quake palette color.
// TODO: This is a simplified implementation that needs proper palette support.
func (dc *glDrawContext) DrawFill(x, y, w, h int, color byte) {
	// For now, just log the call
	// Modern OpenGL core profile doesn't support immediate mode (glBegin/glEnd)
	// TODO: Implement proper 2D quad rendering with shaders and VBOs
	slog.Debug("DrawFill called", "x", x, "y", y, "w", w, "h", h, "color", color)
}

// DrawCharacter renders a single character from the font.
// TODO: This is a simplified implementation that needs proper font texture support.
func (dc *glDrawContext) DrawCharacter(x, y int, num int) {
	// For now, just log the call
	slog.Debug("DrawCharacter called", "x", x, "y", y, "char", num)
	// TODO: Implement proper character rendering from CONCHARS texture
}

type Renderer struct {
	mu sync.RWMutex

	window *glfw.Window
	config Config

	drawCallback   func(RenderContext)
	updateCallback func(dt float64)
	closeCallback  func()

	running bool
}

// New creates a new Renderer with configuration from cvars.
func New() (*Renderer, error) {
	return NewWithConfig(ConfigFromCvars())
}

// NewWithConfig creates a new Renderer with the specified configuration.
func NewWithConfig(cfg Config) (*Renderer, error) {
	slog.Debug("Creating OpenGL renderer", "width", cfg.Width, "height", cfg.Height, "fullscreen", cfg.Fullscreen)

	if err := glfw.Init(); err != nil {
		return nil, fmt.Errorf("failed to initialize glfw: %w", err)
	}

	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)
	glfw.WindowHint(glfw.Visible, glfw.False) // Hide initially

	window, err := glfw.CreateWindow(cfg.Width, cfg.Height, cfg.Title, nil, nil)
	if err != nil {
		glfw.Terminate()
		return nil, fmt.Errorf("failed to create window: %w", err)
	}

	if cfg.Fullscreen {
		monitor := glfw.GetPrimaryMonitor()
		mode := monitor.GetVideoMode()
		window.SetMonitor(monitor, 0, 0, mode.Width, mode.Height, mode.RefreshRate)
	}

	window.MakeContextCurrent()

	// Initialize Glow (go function bindings)
	if err := gl.Init(); err != nil {
		window.Destroy()
		glfw.Terminate()
		return nil, fmt.Errorf("failed to initialize gl: %w", err)
	}

	// Enable VSync if requested
	if cfg.VSync {
		glfw.SwapInterval(1)
	} else {
		glfw.SwapInterval(0)
	}

	// Set up OpenGL state
	gl.Enable(gl.DEPTH_TEST)
	gl.DepthFunc(gl.LESS)
	gl.Enable(gl.CULL_FACE)
	gl.CullFace(gl.BACK)

	// Show the window
	window.Show()

	r := &Renderer{
		window: window,
		config: cfg,
	}

	slog.Info("OpenGL renderer created",
		"width", cfg.Width,
		"height", cfg.Height,
		"fullscreen", cfg.Fullscreen,
		"vsync", cfg.VSync,
		"maxfps", cfg.MaxFPS,
		"gl_version", gl.GoStr(gl.GetString(gl.VERSION)),
	)

	return r, nil
}

// OnDraw sets the callback for frame rendering.
func (r *Renderer) OnDraw(callback func(RenderContext)) {
	r.mu.Lock()
	r.drawCallback = callback
	r.mu.Unlock()
}

// OnUpdate sets the callback for game logic updates.
func (r *Renderer) OnUpdate(callback func(dt float64)) {
	r.mu.Lock()
	r.updateCallback = callback
	r.mu.Unlock()
}

// OnClose sets the callback for window close events.
func (r *Renderer) OnClose(callback func()) {
	r.mu.Lock()
	r.closeCallback = callback
	r.mu.Unlock()
}

// Input returns the input state for keyboard and mouse polling.
func (r *Renderer) Input() interface{} {
	// TODO: Implement input polling for GLFW
	return nil
}

// Size returns the current window size in pixels.
func (r *Renderer) Size() (width, height int) {
	return r.window.GetSize()
}

// ScaleFactor returns the DPI scale factor.
func (r *Renderer) ScaleFactor() float64 {
	monitor := r.window.GetMonitor()
	if monitor == nil {
		monitor = glfw.GetPrimaryMonitor()
	}
	if monitor == nil {
		return 1.0
	}
	xscale, _ := monitor.GetContentScale()
	return float64(xscale)
}

// Config returns the current video configuration.
func (r *Renderer) Config() Config {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.config
}

// SetConfig updates the video configuration.
func (r *Renderer) SetConfig(cfg Config) {
	r.mu.Lock()
	r.config = cfg
	r.mu.Unlock()

	// Apply fullscreen change
	if cfg.Fullscreen {
		monitor := glfw.GetPrimaryMonitor()
		mode := monitor.GetVideoMode()
		r.window.SetMonitor(monitor, 0, 0, mode.Width, mode.Height, mode.RefreshRate)
	} else {
		r.window.SetMonitor(nil, 0, 0, cfg.Width, cfg.Height, 0)
	}

	// Apply VSync change
	if cfg.VSync {
		glfw.SwapInterval(1)
	} else {
		glfw.SwapInterval(0)
	}
}

// Run starts the main rendering loop.
func (r *Renderer) Run() error {
	r.mu.Lock()
	r.running = true
	r.mu.Unlock()

	slog.Info("Starting OpenGL render loop")

	// Frame timing
	lastTime := time.Now()
	frameTime := time.Second / time.Duration(r.config.MaxFPS)

	for !r.window.ShouldClose() {
		// Frame rate limiting
		if r.config.MaxFPS > 0 && !r.config.VSync {
			elapsed := time.Since(lastTime)
			if elapsed < frameTime {
				time.Sleep(frameTime - elapsed)
			}
		}

		now := time.Now()
		dt := now.Sub(lastTime).Seconds()
		lastTime = now

		// Process window events
		glfw.PollEvents()

		// Call update callback
		r.mu.RLock()
		updateCallback := r.updateCallback
		r.mu.RUnlock()
		if updateCallback != nil {
			updateCallback(dt)
		}

		// Render
		width, height := r.window.GetSize()
		gl.Viewport(0, 0, int32(width), int32(height))

		r.mu.RLock()
		drawCallback := r.drawCallback
		gamma := r.config.Gamma
		r.mu.RUnlock()

		if drawCallback != nil {
			dc := &glDrawContext{
				window: r.window,
				gamma:  gamma,
				viewport: struct {
					width  int
					height int
				}{width, height},
			}
			drawCallback(dc)
		}

		// Swap buffers
		r.window.SwapBuffers()
	}

	r.mu.Lock()
	r.running = false
	r.mu.Unlock()

	// Call close callback
	r.mu.RLock()
	closeCallback := r.closeCallback
	r.mu.RUnlock()
	if closeCallback != nil {
		closeCallback()
	}

	slog.Info("OpenGL render loop ended")
	return nil
}

// Stop requests the renderer to stop the main loop.
func (r *Renderer) Stop() {
	r.window.SetShouldClose(true)
}

// Shutdown releases all GPU resources and destroys the window.
func (r *Renderer) Shutdown() {
	slog.Debug("OpenGL renderer shutting down")
	if r.window != nil {
		r.window.Destroy()
	}
	glfw.Terminate()
}

// IsRunning returns true if the render loop is active.
func (r *Renderer) IsRunning() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.running
}
