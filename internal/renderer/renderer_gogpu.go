//go:build gogpu && !cgo
// +build gogpu,!cgo

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
	"os"
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/gmath"
	"github.com/gogpu/gogpu/input"
	"github.com/gogpu/wgpu/hal"
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

var halOnlyFrameConsumed atomic.Bool

// Clear fills the screen with the specified RGBA color.
// This is typically called at the start of each frame to clear
// the previous frame's content.
//
// In Quake, the clear color is typically a dark gray or black,
// but can be adjusted for different visual effects.
func (dc *DrawContext) Clear(r, g, b, a float32) {
	// Use gogpu's proper clear operation
	color := gmath.Color{R: r, G: g, B: b, A: a}
	dc.ctx.ClearColor(color)
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

// DrawPic renders a QPic image at the specified screen-space position.
func (dc *DrawContext) DrawPic(x, y int, pic *image.QPic) {
	if pic == nil {
		return
	}

	tex := dc.renderer.getOrCreateTexture(dc.ctx, pic)
	if tex == nil {
		return
	}

	rect := screenPicRect(x, y, pic)
	err := dc.ctx.DrawTextureScaled(tex, rect.x, rect.y, rect.w, rect.h)
	if err != nil {
		slog.Error("Failed to draw texture", "error", err)
	}
}

// DrawMenuPic renders a QPic image in 320x200 menu-space coordinates.
func (dc *DrawContext) DrawMenuPic(x, y int, pic *image.QPic) {
	if pic == nil {
		return
	}

	tex := dc.renderer.getOrCreateTexture(dc.ctx, pic)
	if tex == nil {
		return
	}

	screenW, screenH := dc.renderer.Size()
	rect := menuPicRect(screenW, screenH, x, y, pic)
	err := dc.ctx.DrawTextureScaled(tex, rect.x, rect.y, rect.w, rect.h)
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

// DrawMenuCharacter renders a single 8×8 character in menu-space coordinates.
func (dc *DrawContext) DrawMenuCharacter(x, y int, num int) {
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
	screenW, screenH := dc.renderer.Size()
	scale, xOff, yOff := menuScale(screenW, screenH)
	if err := dc.ctx.DrawTextureScaled(
		tex,
		float32(x)*scale+xOff,
		float32(y)*scale+yOff,
		8*scale,
		8*scale,
	); err != nil {
		slog.Error("DrawMenuCharacter: draw failed", "num", num, "error", err)
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

	// Camera state and matrices for view/projection
	// cameraState holds the current camera position and orientation.
	cameraState CameraState
	// viewMatrices caches computed view and projection matrices.
	viewMatrices ViewMatrixData

	// worldData holds GPU-side resources for BSP world rendering.
	// Set via UploadWorld() when a map is loaded.
	worldData *WorldRenderData

	// GPU resources for world rendering
	worldVertexBuffer      hal.Buffer
	worldIndexBuffer       hal.Buffer
	worldIndexCount        uint32
	worldPipeline          hal.RenderPipeline
	worldPipelineLayout    hal.PipelineLayout
	worldBindGroup         hal.BindGroup
	worldShader            hal.ShaderModule
	uniformBuffer          hal.Buffer
	uniformBindGroup       hal.BindGroup
	uniformBindGroupLayout hal.BindGroupLayout
	textureBindGroupLayout hal.BindGroupLayout

	// 1x1 white texture for fallback
	whiteTexture          hal.Texture
	whiteTextureView      hal.TextureView
	worldDepthTexture     hal.Texture
	worldDepthTextureView hal.TextureView

	// Offscreen render target for world rendering
	worldRenderTexture      hal.Texture
	worldRenderTextureView  hal.TextureView
	worldRenderTextureGogpu *gogpu.Texture // gogpu-wrapped version for compositing
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
	width, height = r.app.Size()
	scale := r.app.ScaleFactor()
	if scale > 0 && scale != 1.0 {
		width = int(float64(width)*scale + 0.5)
		height = int(float64(height)*scale + 0.5)
	}
	return width, height
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

	// BrushEntities contains inline BSP submodels for parity with the OpenGL path.
	BrushEntities []BrushEntity

	// AliasEntities contains world-space MDL entities for parity with the OpenGL path.
	AliasEntities []AliasModelEntity

	// SpriteEntities contains sprite (billboard) entities for parity with the OpenGL path.
	SpriteEntities []SpriteEntity

	// DecalMarks contains projected world-space mark entities for parity with the OpenGL path.
	DecalMarks []DecalMarkEntity

	// ViewModel contains the first-person weapon model when active.
	ViewModel *AliasModelEntity

	// LightStyles contains evaluated lightstyle scalars for the current frame.
	LightStyles [64]float32

	// FogColor and FogDensity mirror the authoritative renderer state for parity tracking.
	FogColor   [3]float32
	FogDensity float32

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

	// WaterWarp, WaterWarpTime, ForceUnderwater: see stubs_opengl.go for semantics.
	// These fields are parsed by the authoritative OpenGL path only; gogpu ignores them.
	WaterWarp       bool
	WaterWarpTime   float32
	ForceUnderwater bool
}

// RenderFrame executes the complete frame pipeline in order:
// 1. Clear screen
// 2. Draw 3D world (stub)
// 3. Draw entities (baseline projected markers)
// 4. Draw particles
// 5. Draw 2D overlay (HUD, menu, console)
func (dc *DrawContext) RenderFrame(state *RenderFrameState, draw2DOverlay func(dc RenderContext)) {
	if state == nil {
		return
	}

	slog.Debug("RenderFrame called", "draw_world", state.DrawWorld, "draw_particles", state.DrawParticles, "draw_2d_overlay", state.Draw2DOverlay)
	slog.Info("RenderFrame: surface view (start)", "id", debugSurfaceViewID(dc.ctx.SurfaceView()))
	if frameCleared, hasPendingClear, ok := dc.getGoGPUFrameStateForDebug(); ok {
		slog.Info("RenderFrame: gogpu frame state (start)", "frameCleared", frameCleared, "hasPendingClear", hasPendingClear)
	}

	// Phase 1: Clear screen
	// Skip clear when world rendering is active - the world render pass will handle clearing,
	// and gogpu will use LoadOpLoad to preserve our world rendering when drawing the overlay.
	if !state.DrawWorld {
		dc.Clear(state.ClearColor[0], state.ClearColor[1], state.ClearColor[2], state.ClearColor[3])
	}

	// Phase 2: Draw 3D world directly to surface view (zero-copy)
	// HAL renders to dc.ctx.SurfaceView() which is the current frame's swapchain texture.
	// Then gogpu draws 2D overlay on top with LoadOpLoad to preserve the world.
	if state.DrawWorld {
		slog.Info("RenderFrame: rendering world to surface")
		dc.renderWorld(state)
		slog.Info("RenderFrame: surface view (after world)", "id", debugSurfaceViewID(dc.ctx.SurfaceView()))
		if dc.markGoGPUFrameContentForOverlay() {
			slog.Info("RenderFrame: marked gogpu frame as pre-populated (HAL world rendered)")
			if frameCleared, hasPendingClear, ok := dc.getGoGPUFrameStateForDebug(); ok {
				slog.Info("RenderFrame: gogpu frame state (after mark)", "frameCleared", frameCleared, "hasPendingClear", hasPendingClear)
			}
		} else {
			slog.Warn("RenderFrame: unable to mark gogpu frame state; first 2D draw may clear world")
		}
	}

	// Nuclear debug option: skip all gogpu-side drawing for exactly one frame.
	// This isolates "HAL world render + present" from all 2D/overlay paths.
	if state.DrawWorld && shouldRunHALOnlyFrame() {
		slog.Warn("RenderFrame: HAL-only debug frame enabled; skipping entities/particles/2D overlay")
		dc.logPrePresentState("hal-only")
		return
	}

	// Phase 3: Draw entities (baseline projected markers)
	if state.DrawEntities {
		dc.renderEntities(state)
	}

	// Phase 4: Draw particles
	if state.DrawParticles && state.Particles != nil {
		dc.renderParticles(state)
	}

	// Phase 5: Draw 2D overlay (HUD, menu, console)
	// Re-enable to show menu + fallback dots
	// IMPORTANT: When we skip dc.Clear() above (because state.DrawWorld=true),
	// gogpu should use LoadOpLoad for its internal 2D render pass, preserving
	// the world rendering we just submitted via HAL. This relies on gogpu's
	// internal behavior to detect that Clear() was not called.
	if state.Draw2DOverlay && draw2DOverlay != nil {
		if frameCleared, hasPendingClear, ok := dc.getGoGPUFrameStateForDebug(); ok {
			slog.Info("RenderFrame: gogpu frame state (pre-overlay)", "frameCleared", frameCleared, "hasPendingClear", hasPendingClear)
		}
		if shouldDrawWorldFallbackDots() {
			slog.Info("RenderFrame: debug world fallback dots enabled")
			dc.renderWorldFallbackTopDown()
		}
		slog.Debug("Drawing 2D overlay on top of world", "menu_active", state.MenuActive)
		draw2DOverlay(dc)
	}

	dc.logPrePresentState("normal")
}

func shouldRunHALOnlyFrame() bool {
	if os.Getenv("IRONWAIL_DEBUG_HAL_ONLY_FRAME") != "1" {
		return false
	}

	return !halOnlyFrameConsumed.Swap(true)
}

func (dc *DrawContext) logPrePresentState(mode string) {
	if dc == nil || dc.ctx == nil {
		return
	}

	if frameCleared, hasPendingClear, ok := dc.getGoGPUFrameStateForDebug(); ok {
		slog.Info("RenderFrame: gogpu frame state (pre-present)",
			"mode", mode,
			"frameCleared", frameCleared,
			"hasPendingClear", hasPendingClear,
		)
	} else {
		slog.Warn("RenderFrame: unable to read gogpu frame state (pre-present)", "mode", mode)
	}

	slog.Info("RenderFrame: surface view (pre-present)", "mode", mode, "id", debugSurfaceViewID(dc.ctx.SurfaceView()))
}

func debugSurfaceViewID(view any) string {
	if view == nil {
		return "nil"
	}

	value := reflect.ValueOf(view)
	if value.Kind() == reflect.Pointer || value.Kind() == reflect.UnsafePointer {
		if value.IsNil() {
			return fmt.Sprintf("%T@nil", view)
		}
		return fmt.Sprintf("%T@0x%x", view, value.Pointer())
	}

	return fmt.Sprintf("%T@%p", view, &view)
}

func shouldDrawWorldFallbackDots() bool {
	return os.Getenv("IRONWAIL_DEBUG_WORLD_DOTS") == "1"
}

// markGoGPUFrameContentForOverlay sets gogpu renderer's internal frame state
// so the first 2D draw pass uses LoadOpLoad (preserve existing surface content)
// rather than defaulting to LoadOpClear.
func (dc *DrawContext) markGoGPUFrameContentForOverlay() bool {
	if dc == nil || dc.ctx == nil {
		return false
	}

	ctxValue := reflect.ValueOf(dc.ctx)
	if ctxValue.Kind() != reflect.Pointer || ctxValue.IsNil() {
		return false
	}

	ctxElem := ctxValue.Elem()
	rendererField := ctxElem.FieldByName("renderer")
	if !rendererField.IsValid() || rendererField.Kind() != reflect.Pointer || rendererField.IsNil() {
		return false
	}

	rendererPtr := reflect.NewAt(rendererField.Type(), unsafe.Pointer(rendererField.UnsafeAddr())).Elem()
	rendererElem := rendererPtr.Elem()

	frameClearedField := rendererElem.FieldByName("frameCleared")
	hasPendingClearField := rendererElem.FieldByName("hasPendingClear")
	if !frameClearedField.IsValid() || !hasPendingClearField.IsValid() {
		return false
	}

	frameClearedWritable := reflect.NewAt(frameClearedField.Type(), unsafe.Pointer(frameClearedField.UnsafeAddr())).Elem()
	hasPendingClearWritable := reflect.NewAt(hasPendingClearField.Type(), unsafe.Pointer(hasPendingClearField.UnsafeAddr())).Elem()

	if frameClearedWritable.Kind() != reflect.Bool || hasPendingClearWritable.Kind() != reflect.Bool {
		return false
	}

	frameClearedWritable.SetBool(true)
	hasPendingClearWritable.SetBool(false)

	return true
}

func (dc *DrawContext) getGoGPUFrameStateForDebug() (frameCleared bool, hasPendingClear bool, ok bool) {
	if dc == nil || dc.ctx == nil {
		return false, false, false
	}

	ctxValue := reflect.ValueOf(dc.ctx)
	if ctxValue.Kind() != reflect.Pointer || ctxValue.IsNil() {
		return false, false, false
	}

	ctxElem := ctxValue.Elem()
	rendererField := ctxElem.FieldByName("renderer")
	if !rendererField.IsValid() || rendererField.Kind() != reflect.Pointer || rendererField.IsNil() {
		return false, false, false
	}

	rendererPtr := reflect.NewAt(rendererField.Type(), unsafe.Pointer(rendererField.UnsafeAddr())).Elem()
	rendererElem := rendererPtr.Elem()

	frameClearedField := rendererElem.FieldByName("frameCleared")
	hasPendingClearField := rendererElem.FieldByName("hasPendingClear")
	if !frameClearedField.IsValid() || !hasPendingClearField.IsValid() {
		return false, false, false
	}

	frameClearedReadable := reflect.NewAt(frameClearedField.Type(), unsafe.Pointer(frameClearedField.UnsafeAddr())).Elem()
	hasPendingClearReadable := reflect.NewAt(hasPendingClearField.Type(), unsafe.Pointer(hasPendingClearField.UnsafeAddr())).Elem()
	if frameClearedReadable.Kind() != reflect.Bool || hasPendingClearReadable.Kind() != reflect.Bool {
		return false, false, false
	}

	return frameClearedReadable.Bool(), hasPendingClearReadable.Bool(), true
}

// renderWorldFallbackTopDown draws a sparse top-down projection of world
// vertices using the 2D API. It is used as a visibility fallback while the
// GPU world pass is being stabilized across platforms.
func (dc *DrawContext) renderWorldFallbackTopDown() {
	worldData := dc.renderer.GetWorldData()
	if worldData == nil || worldData.Geometry == nil || len(worldData.Geometry.Vertices) == 0 {
		return
	}

	verts := worldData.Geometry.Vertices
	minX, maxX := verts[0].Position[0], verts[0].Position[0]
	minY, maxY := verts[0].Position[1], verts[0].Position[1]
	for index := 1; index < len(verts); index++ {
		x := verts[index].Position[0]
		y := verts[index].Position[1]
		if x < minX {
			minX = x
		}
		if x > maxX {
			maxX = x
		}
		if y < minY {
			minY = y
		}
		if y > maxY {
			maxY = y
		}
	}

	rangeX := maxX - minX
	rangeY := maxY - minY
	if rangeX < 1 || rangeY < 1 {
		return
	}

	screenW, screenH := dc.renderer.Size()
	if screenW <= 0 || screenH <= 0 {
		return
	}

	const margin = 24.0
	drawW := float32(screenW) - margin*2
	drawH := float32(screenH) - margin*2
	if drawW <= 0 || drawH <= 0 {
		return
	}

	scaleX := drawW / rangeX
	scaleY := drawH / rangeY
	scale := scaleX
	if scaleY < scale {
		scale = scaleY
	}

	step := len(verts) / 5000
	if step < 1 {
		step = 1
	}

	// Draw a visible frame so non-black background is obvious.
	frameColor := byte(250)
	panelColor := byte(48)
	dc.DrawFill(int(margin), int(margin), int(drawW), int(drawH), panelColor)
	dc.DrawFill(int(margin), int(margin), int(drawW), 2, frameColor)
	dc.DrawFill(int(margin), int(margin+drawH)-2, int(drawW), 2, frameColor)
	dc.DrawFill(int(margin), int(margin), 2, int(drawH), frameColor)
	dc.DrawFill(int(margin+drawW)-2, int(margin), 2, int(drawH), frameColor)

	for index := 0; index < len(verts); index += step {
		v := verts[index]
		sx := int(margin + (v.Position[0]-minX)*scale)
		sy := int(float32(screenH) - (margin + (v.Position[1]-minY)*scale))
		if sx < 0 || sx >= screenW || sy < 0 || sy >= screenH {
			continue
		}
		dc.DrawFill(sx, sy, 2, 2, 251)
	}
}

// renderWorld draws the 3D world geometry (BSP surfaces).
// Implementation delegates to renderWorldInternal in world.go.
func (dc *DrawContext) renderWorld(state *RenderFrameState) {
	// Render world directly to the current surface view (zero-copy approach)
	// The surface view is available from dc.ctx.SurfaceView() per gogpu's design
	dc.renderWorldInternal(state)
}

// Note: ensureWorldRenderTarget removed - we now render directly to the surface view
// provided by gogpu via dc.ctx.SurfaceView() for zero-copy rendering.

// renderEntities draws a bounded fallback for runtime entities.
// Instead of a full model pipeline, it projects entity origins to screen-space
// and draws small colored markers so gameplay-relevant entities are visible.
func (dc *DrawContext) renderEntities(state *RenderFrameState) {
	if dc == nil || dc.renderer == nil || state == nil {
		return
	}

	screenW, screenH := dc.renderer.Size()
	if screenW <= 0 || screenH <= 0 {
		return
	}

	vpMatrix := dc.renderer.GetViewProjectionMatrix()

	for _, entity := range state.AliasEntities {
		dc.drawProjectedEntityMarker(entity.Origin, vpMatrix, screenW, screenH, 4, 248)
	}
	for _, entity := range state.SpriteEntities {
		dc.drawProjectedEntityMarker(entity.Origin, vpMatrix, screenW, screenH, 6, 220)
	}
	for _, mark := range state.DecalMarks {
		dc.drawProjectedEntityMarker(mark.Origin, vpMatrix, screenW, screenH, 2, 180)
	}
	if state.ViewModel != nil {
		dc.drawProjectedEntityMarker(state.ViewModel.Origin, vpMatrix, screenW, screenH, 5, 251)
	}
}

func (dc *DrawContext) drawProjectedEntityMarker(pos [3]float32, vp gmath.Mat4, screenW, screenH, size int, color byte) {
	x, y, ok := projectWorldPointToScreen(pos, vp, screenW, screenH)
	if !ok {
		return
	}

	if size < 1 {
		size = 1
	}
	half := size / 2
	dc.DrawFill(x-half, y-half, size, size, color)
}

func projectWorldPointToScreen(pos [3]float32, vp gmath.Mat4, screenW, screenH int) (x int, y int, ok bool) {
	if screenW <= 0 || screenH <= 0 {
		return 0, 0, false
	}

	clipPos := TransformVertex(pos, vp)
	if clipPos.W <= 0.001 {
		return 0, 0, false
	}

	invW := float32(1.0) / clipPos.W
	ndcX := clipPos.X * invW
	ndcY := clipPos.Y * invW
	ndcZ := clipPos.Z * invW

	if ndcX < -1 || ndcX > 1 || ndcY < -1 || ndcY > 1 || ndcZ < -1 || ndcZ > 1 {
		return 0, 0, false
	}

	screenX := (ndcX*0.5 + 0.5) * float32(screenW-1)
	screenY := (1 - (ndcY*0.5 + 0.5)) * float32(screenH-1)

	return int(screenX), int(screenY), true
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

// UpdateCamera updates the camera state and recomputes view/projection matrices.
// This should be called once per frame with the current player position and orientation
// from client prediction.
func (r *Renderer) UpdateCamera(camera CameraState, nearPlane, farPlane float32) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cameraState = camera

	// Compute view matrix from camera state
	r.viewMatrices.View = ComputeViewMatrix(camera)

	// Compute projection matrix (aspect ratio from window size)
	// Use a default aspect ratio if the app is not initialized
	aspect := float32(16.0 / 9.0)
	if r.app != nil {
		w, h := r.Size()
		if w > 0 && h > 0 {
			aspect = float32(w) / float32(h)
		}
	}

	r.viewMatrices.Projection = ComputeProjectionMatrix(camera.FOV, aspect, nearPlane, farPlane)

	// Log individual matrices before multiplication
	slog.Info("Camera matrices computed",
		"view_m00", r.viewMatrices.View[0],
		"view_m11", r.viewMatrices.View[5],
		"view_m22", r.viewMatrices.View[10],
		"view_m33", r.viewMatrices.View[15],
		"proj_m00", r.viewMatrices.Projection[0],
		"proj_m11", r.viewMatrices.Projection[5],
		"proj_m22", r.viewMatrices.Projection[10],
		"proj_m33", r.viewMatrices.Projection[15])

	// Compute combined VP matrix
	r.viewMatrices.VP = r.viewMatrices.Projection.Mul(r.viewMatrices.View)

	// Log VP matrix for debugging
	slog.Debug("Camera updated",
		"position", camera.Origin,
		"angles", camera.Angles,
		"near", nearPlane,
		"far", farPlane,
		"aspect", aspect,
		"fov", camera.FOV,
		"vp_matrix_0_0", r.viewMatrices.VP[0],
		"vp_matrix_3_2", r.viewMatrices.VP[14])
}

// GetViewMatrix returns the currently cached view matrix.
// Thread-safe read.
func (r *Renderer) GetViewMatrix() gmath.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.View
}

// GetProjectionMatrix returns the currently cached projection matrix.
// Thread-safe read.
func (r *Renderer) GetProjectionMatrix() gmath.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.Projection
}

// GetViewProjectionMatrix returns the combined View × Projection matrix.
// This is the matrix typically used in vertex shaders for world-to-NDC transformation.
// Thread-safe read.
func (r *Renderer) GetViewProjectionMatrix() gmath.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.VP
}

// GetCameraState returns the current camera state (position and orientation).
// Thread-safe read. A copy is returned to prevent external modification.
func (r *Renderer) GetCameraState() CameraState {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.cameraState
}

// HasWorldData reports whether GPU world geometry has been uploaded.
func (r *Renderer) HasWorldData() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.worldData != nil && r.worldVertexBuffer != nil && r.worldIndexBuffer != nil && r.worldIndexCount > 0 && r.worldPipeline != nil
}

func (r *Renderer) SpawnDynamicLight(light DynamicLight) bool {
	return false
}

func (r *Renderer) UpdateLights(deltaTime float32) {}

func (r *Renderer) ClearDynamicLights() {}

func (r *Renderer) SetExternalSkybox(name string, loadFile func(string) ([]byte, error)) {}

// NeedsWorldGPUUpload reports whether CPU world geometry exists but GPU buffers
// are not uploaded yet.
func (r *Renderer) NeedsWorldGPUUpload() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.worldData != nil && (r.worldVertexBuffer == nil || r.worldIndexBuffer == nil || r.worldIndexCount == 0)
}

// getHALDevice returns the underlying HAL device from the gogpu renderer.
// This uses reflection to access the private device field from gogpu.Renderer.
// Returns nil if device is not available.
func (r *Renderer) getHALDevice() hal.Device {
	if r.app == nil {
		return nil
	}
	provider := r.app.DeviceProvider()
	if provider == nil {
		return nil
	}
	return provider.Device()
}

// getHALQueue returns the underlying HAL queue from the gogpu renderer.
// This uses reflection to access the private queue field from gogpu.Renderer.
// Returns nil if queue is not available.
func (r *Renderer) getHALQueue() hal.Queue {
	if r.app == nil {
		return nil
	}
	provider := r.app.DeviceProvider()
	if provider == nil {
		return nil
	}
	return provider.Queue()
}
