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

	// renderer is the parent Renderer instance.
	renderer *Renderer
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
// Coordinates are in Quake's virtual 320-wide space and are scaled to physical pixels.
func (dc *DrawContext) DrawPic(x, y int, pic *image.QPic) {
	if pic == nil {
		return
	}

	tex := dc.renderer.getOrCreateTexture(dc.ctx, pic)
	if tex == nil {
		return
	}

	screenW, screenH := dc.renderer.Size()
	scale, xOff, yOff := menuScale(screenW, screenH)
	err := dc.ctx.DrawTextureScaled(tex,
		float32(x)*scale+xOff, float32(y)*scale+yOff,
		float32(pic.Width)*scale, float32(pic.Height)*scale)
	if err != nil {
		slog.Error("Failed to draw texture", "error", err)
	}
}

// DrawFill fills a rectangle with a Quake palette color.
func (dc *DrawContext) DrawFill(x, y, w, h int, color byte) {
	tex := dc.renderer.getOrCreateColorTexture(dc.ctx, color)
	if tex == nil {
		return
	}

	err := dc.ctx.DrawTextureScaled(tex, float32(x), float32(y), float32(w), float32(h))
	if err != nil {
		slog.Error("Failed to draw color texture", "error", err)
	}
}

// DrawCharacter renders a single 8×8 character from the conchars font.
// Falls back to a coloured square if conchars is not loaded.
func (dc *DrawContext) DrawCharacter(x, y int, num int) {
	if num < 0 || num > 255 {
		return
	}
	pic := dc.renderer.getCharPic(num)
	if pic == nil {
		return
	}
	tex := dc.renderer.getOrCreateCharTexture(dc.ctx, num, pic)
	if tex == nil {
		return
	}
	if err := dc.ctx.DrawTextureScaled(tex, float32(x), float32(y), 8, 8); err != nil {
		slog.Error("DrawCharacter: draw failed", "num", num, "error", err)
	}
}

// SetConchars stores the raw 128×128 conchars pixel data and clears the
// per-character texture cache so that DrawCharacter uses the real font.
func (r *Renderer) SetConchars(data []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(data) < 128*128 {
		return
	}
	r.concharsData = data
	r.charCache = [256]*image.QPic{}
}

// getCharPic returns (or lazily creates) an 8×8 QPic for character num extracted
// from the 128×128 conchars bitmap (16 chars per row).
func (r *Renderer) getCharPic(num int) *image.QPic {
	r.mu.RLock()
	if len(r.concharsData) < 128*128 {
		r.mu.RUnlock()
		return nil
	}
	if r.charCache[num] != nil {
		pic := r.charCache[num]
		r.mu.RUnlock()
		return pic
	}
	r.mu.RUnlock()

	col := num % 16
	row := num / 16
	pixels := make([]byte, 8*8)
	for y := 0; y < 8; y++ {
		src := (row*8+y)*128 + col*8
		copy(pixels[y*8:y*8+8], r.concharsData[src:src+8])
	}
	pic := &image.QPic{Width: 8, Height: 8, Pixels: pixels}

	r.mu.Lock()
	r.charCache[num] = pic
	r.mu.Unlock()
	return pic
}

// charCacheKey is a cache key for character textures (separate from pic-based cache).
type charCacheKey struct{ num int }

// getOrCreateCharTexture returns a GPU texture for a character, uploading it if needed.
// Uses ConvertConcharsToRGBA so index-0 pixels are transparent.
func (r *Renderer) getOrCreateCharTexture(ctx *gogpu.Context, num int, pic *image.QPic) *gogpu.Texture {
	key := cacheKey{pic: pic}
	r.mu.RLock()
	if entry, ok := r.textureCache[key]; ok {
		r.mu.RUnlock()
		return entry.texture
	}
	r.mu.RUnlock()

	rgba := ConvertConcharsToRGBA(pic.Pixels, r.palette)
	tex, err := ctx.Renderer().NewTextureFromRGBA(int(pic.Width), int(pic.Height), rgba)
	if err != nil {
		slog.Error("getOrCreateCharTexture: upload failed", "num", num, "error", err)
		return nil
	}

	r.mu.Lock()
	r.textureCache[key] = &cachedTexture{texture: tex, width: int(pic.Width), height: int(pic.Height)}
	r.mu.Unlock()
	return tex
}

type cacheKey struct {
	pic *image.QPic
}

type cachedTexture struct {
	texture *gogpu.Texture
	width   int
	height  int
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

	// textureCache stores uploaded textures to avoid re-uploading
	textureCache map[cacheKey]*cachedTexture

	// colorTextures stores 1x1 textures for solid colors
	colorTextures [256]*gogpu.Texture

	// palette is the current Quake palette (768 bytes)
	palette []byte

	// concharsData is the raw 128×128 indexed-pixel data for the console font.
	concharsData []byte
	// charCache caches per-character 8×8 QPic objects extracted from concharsData.
	charCache [256]*image.QPic
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
		app:          app,
		config:       cfg,
		textureCache: make(map[cacheKey]*cachedTexture),
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

// SetPalette sets the Quake palette used for rendering.
func (r *Renderer) SetPalette(palette []byte) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.palette = make([]byte, len(palette))
	copy(r.palette, palette)

	// Invalidate texture cache since palette changed
	r.textureCache = make(map[cacheKey]*cachedTexture)
	for i := range r.colorTextures {
		r.colorTextures[i] = nil
	}
}

func (r *Renderer) getOrCreateTexture(ctx *gogpu.Context, pic *image.QPic) *gogpu.Texture {
	r.mu.RLock()
	cached, ok := r.textureCache[cacheKey{pic: pic}]
	palette := r.palette
	r.mu.RUnlock()

	if ok && cached != nil {
		return cached.texture
	}

	// Convert palette to RGBA
	rgba := ConvertPaletteToRGBA(pic.Pixels, palette)

	// Create texture
	tex, err := ctx.Renderer().NewTextureFromRGBA(int(pic.Width), int(pic.Height), rgba)
	if err != nil {
		slog.Error("Failed to create texture", "error", err)
		return nil
	}

	r.mu.Lock()
	r.textureCache[cacheKey{pic: pic}] = &cachedTexture{
		texture: tex,
		width:   int(pic.Width),
		height:  int(pic.Height),
	}
	r.mu.Unlock()

	return tex
}

func (r *Renderer) getOrCreateColorTexture(ctx *gogpu.Context, color byte) *gogpu.Texture {
	r.mu.RLock()
	tex := r.colorTextures[color]
	palette := r.palette
	r.mu.RUnlock()

	if tex != nil {
		return tex
	}

	// Create 1x1 RGBA texture
	rgba := make([]byte, 4)
	if IsTransparentIndex(color) {
		rgba[0], rgba[1], rgba[2], rgba[3] = 0, 0, 0, 0
	} else {
		pr, pg, pb := GetPaletteColor(color, palette)
		rgba[0], rgba[1], rgba[2], rgba[3] = pr, pg, pb, 255
	}

	newTex, err := ctx.Renderer().NewTextureFromRGBA(1, 1, rgba)
	if err != nil {
		slog.Error("Failed to create color texture", "error", err)
		return nil
	}

	r.mu.Lock()
	r.colorTextures[color] = newTex
	r.mu.Unlock()

	return newTex
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
				slog.String("device", fmt.Sprintf("%T", provider.Device())),
				slog.String("queue", fmt.Sprintf("%T", provider.Queue())),
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
				ctx:      ctx,
				gamma:    gamma,
				renderer: r,
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

// Frame Pipeline Methods
// These methods orchestrate the rendering phases in the correct order.

// RenderFrameState contains all data needed to render a frame.
type RenderFrameState struct {
	// ClearColor is the RGBA clear color (typically dark gray/black)
	ClearColor [4]float32

	// DrawWorld enables 3D world rendering
	DrawWorld bool

	// DrawEntities enables entity rendering
	DrawEntities bool

	// DrawParticles enables particle rendering
	DrawParticles bool

	// Draw2DOverlay enables 2D overlay (HUD, menu, console)
	Draw2DOverlay bool

	// MenuActive indicates if menu is currently displayed
	MenuActive bool

	// Particles is the active particle system to render
	Particles *ParticleSystem

	// Palette for color conversion
	Palette []byte
}

// RenderFrame executes the complete frame pipeline in order:
// 1. Clear screen
// 2. Draw 3D world (stub)
// 3. Draw entities (stub)
// 4. Draw particles
// 5. Draw 2D overlay (HUD, menu, console)
func (dc *DrawContext) RenderFrame(state *RenderFrameState, draw2DOverlay func(dc RenderContext)) {
	if state == nil {
		return
	}

	// Phase 1: Clear screen
	dc.Clear(state.ClearColor[0], state.ClearColor[1], state.ClearColor[2], state.ClearColor[3])

	// Phase 2: Draw 3D world (stub - M4.3 will implement)
	if state.DrawWorld {
		dc.renderWorld(state)
	}

	// Phase 3: Draw entities (stub - M4.4 will implement)
	if state.DrawEntities {
		dc.renderEntities(state)
	}

	// Phase 4: Draw particles
	if state.DrawParticles && state.Particles != nil {
		dc.renderParticles(state)
	}

	// Phase 5: Draw 2D overlay (HUD, menu, console)
	if state.Draw2DOverlay && draw2DOverlay != nil {
		draw2DOverlay(dc)
	}
}

// renderWorld draws the 3D world geometry (BSP surfaces).
// Stub implementation - M4.3 will implement proper BSP rendering.
func (dc *DrawContext) renderWorld(state *RenderFrameState) {
	// TODO: Implement BSP world rendering (M4.3)
	// For now, this is a stub that does nothing.
	// The world will be rendered using vertex buffers and textures
	// loaded from the BSP file.
}

// renderEntities draws entity models (alias models, brush models).
// Stub implementation - M4.4 will implement proper entity rendering.
func (dc *DrawContext) renderEntities(state *RenderFrameState) {
	// TODO: Implement entity rendering (M4.4)
	// For now, this is a stub that does nothing.
	// Entities will be rendered using MDL/alias model pipeline.
}

// renderParticles draws the particle system.
func (dc *DrawContext) renderParticles(state *RenderFrameState) {
	if state.Particles == nil || state.Particles.ActiveCount() == 0 {
		return
	}

	// Get active particles
	particles := state.Particles.ActiveParticles()

	// Build particle vertices
	palette := buildParticlePalette(state.Palette)
	verts := BuildParticleVertices(particles, palette, false) // false = not showtris mode

	// Draw each particle as a small colored quad
	// This is a simplified implementation - a proper implementation would use
	// instanced rendering or a point sprite shader
	for i, v := range verts {
		if i >= len(particles) {
			break
		}
		// Draw particle as a small quad
		size := float32(4.0) // Particle size in pixels
		x := int(v.Pos[0])
		y := int(v.Pos[1])
		color := particles[i].Color
		dc.DrawFill(x-int(size/2), y-int(size/2), int(size), int(size), color)
	}
}

// buildParticlePalette converts a 768-byte palette to the [256][4]byte format
// expected by BuildParticleVertices.
func buildParticlePalette(palette []byte) [256][4]byte {
	var p [256][4]byte
	if len(palette) < 768 {
		// Grayscale fallback
		for i := range p {
			p[i] = [4]byte{byte(i), byte(i), byte(i), 255}
		}
		return p
	}
	for i := range p {
		offset := i * 3
		p[i] = [4]byte{
			palette[offset],
			palette[offset+1],
			palette[offset+2],
			255,
		}
	}
	return p
}

// DefaultRenderFrameState returns a sensible default RenderFrameState.
func DefaultRenderFrameState() *RenderFrameState {
	return &RenderFrameState{
		ClearColor:    [4]float32{0.0, 0.0, 0.0, 1.0}, // Black
		DrawWorld:     false,                          // Disabled until M4.3
		DrawEntities:  false,                          // Disabled until M4.4
		DrawParticles: false,                          // Disabled until particle system is wired
		Draw2DOverlay: true,                           // Always draw 2D overlay
		MenuActive:    true,                           // Menu is active at startup
	}
}
