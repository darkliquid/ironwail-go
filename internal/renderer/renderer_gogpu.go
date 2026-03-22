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
	"github.com/gogpu/gogpu/gmath" // retained only for gogpu API boundary (Color type)
	"github.com/gogpu/gogpu/input"
	"github.com/gogpu/wgpu/hal"
	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/model"
	"github.com/ironwail/ironwail-go/pkg/types"
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

	// Canvas coordinate system state.
	canvas            CanvasState
	canvasParams      CanvasTransformParams
	sceneRenderActive bool
	sceneRenderTarget hal.TextureView
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
	// Convert to gmath.Color at the gogpu API boundary.
	dc.ctx.ClearColor(gmath.Color{R: r, G: g, B: b, A: a})
}

// DrawTriangle renders a simple colored triangle.
// This is primarily useful for testing the rendering pipeline.
// In a full implementation, this would be replaced with proper
// 3D geometry rendering using shaders.
func (dc *DrawContext) DrawTriangle(r, g, b, a float32) {
	// Convert to gmath.Color at the gogpu API boundary.
	dc.ctx.DrawTriangleColor(gmath.Color{R: r, G: g, B: b, A: a})
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

// SetCanvas switches the active 2D canvas coordinate system.
func (dc *DrawContext) SetCanvas(ct CanvasType) {
	if dc == nil {
		return
	}
	params := dc.canvasParams
	if dc.renderer != nil {
		screenW, screenH := dc.renderer.Size()
		if params.GUIWidth <= 0 {
			params.GUIWidth = float32(screenW)
		}
		if params.GUIHeight <= 0 {
			params.GUIHeight = float32(screenH)
		}
		if params.GLWidth <= 0 {
			params.GLWidth = float32(screenW)
		}
		if params.GLHeight <= 0 {
			params.GLHeight = float32(screenH)
		}
	}
	if params.ConWidth <= 0 {
		params.ConWidth = params.GUIWidth
	}
	if params.ConHeight <= 0 {
		params.ConHeight = params.GUIHeight
	}
	if params.GUIWidth <= 0 || params.GUIHeight <= 0 || params.GLWidth <= 0 || params.GLHeight <= 0 {
		dc.canvas.Type = ct
		return
	}
	SetCanvas(&dc.canvas, ct, params)
}

// Canvas returns the current canvas state.
func (dc *DrawContext) Canvas() CanvasState {
	return dc.canvas
}

// SetCanvasParams updates the per-frame canvas transform parameters.
func (dc *DrawContext) SetCanvasParams(p CanvasTransformParams) {
	dc.canvasParams = p
	dc.canvas.Type = CanvasNone
}

// 2D Drawing API implementation

// DrawPic renders a QPic image at the specified screen-space position.
func (dc *DrawContext) DrawPic(x, y int, pic *image.QPic) {
	dc.DrawPicAlpha(x, y, pic, 1)
}

// DrawPicAlpha renders a QPic image at a screen-space position with explicit alpha.
func (dc *DrawContext) DrawPicAlpha(x, y int, pic *image.QPic, alpha float32) {
	if pic == nil {
		return
	}
	if alpha <= 0 {
		return
	}

	tex := dc.renderer.getOrCreateTexture(dc.ctx, pic)
	if tex == nil {
		return
	}

	rect := dc.screenPicRect(x, y, int(pic.Width), int(pic.Height))
	if alpha >= 1 {
		err := dc.ctx.DrawTextureScaled(tex, rect.x, rect.y, rect.w, rect.h)
		if err != nil {
			slog.Error("Failed to draw texture", "error", err)
		}
		return
	}
	err := dc.ctx.DrawTextureEx(tex, gogpu.DrawTextureOptions{
		X:      rect.x,
		Y:      rect.y,
		Width:  rect.w,
		Height: rect.h,
		Alpha:  alpha,
	})
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

	rect := dc.screenPicRect(x, y, int(pic.Width), int(pic.Height))
	err := dc.ctx.DrawTextureScaled(tex, rect.x, rect.y, rect.w, rect.h)
	if err != nil {
		slog.Error("Failed to draw texture", "error", err)
	}
}

// DrawFill fills a rectangle with a Quake palette color.
func (dc *DrawContext) DrawFill(x, y, w, h int, color byte) {
	dc.DrawFillAlpha(x, y, w, h, color, 1)
}

// DrawFillAlpha fills a rectangle with a Quake palette color and explicit alpha.
func (dc *DrawContext) DrawFillAlpha(x, y, w, h int, color byte, alpha float32) {
	if w <= 0 || h <= 0 || alpha <= 0 {
		return
	}
	tex := dc.renderer.getOrCreateColorTexture(dc.ctx, color)
	if tex == nil {
		return
	}

	if alpha >= 1 {
		alpha = 1
	}
	rect := dc.screenPicRect(x, y, w, h)
	err := dc.ctx.DrawTextureEx(tex, gogpu.DrawTextureOptions{
		X:      rect.x,
		Y:      rect.y,
		Width:  rect.w,
		Height: rect.h,
		Alpha:  alpha,
	})
	if err != nil {
		slog.Error("Failed to draw color texture", "error", err)
	}
}

// DrawCharacter renders a single 8×8 character from the conchars font.
// Falls back to a coloured square if conchars is not loaded.
func (dc *DrawContext) DrawCharacter(x, y int, num int) {
	dc.DrawCharacterAlpha(x, y, num, 1)
}

// DrawCharacterAlpha renders a single 8×8 character from the conchars font with explicit alpha.
func (dc *DrawContext) DrawCharacterAlpha(x, y int, num int, alpha float32) {
	if num < 0 || num > 255 {
		return
	}
	if alpha <= 0 {
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
	rect := dc.screenPicRect(x, y, 8, 8)
	if alpha >= 1 {
		if err := dc.ctx.DrawTextureScaled(tex, rect.x, rect.y, rect.w, rect.h); err != nil {
			slog.Error("DrawCharacter: draw failed", "num", num, "error", err)
		}
		return
	}
	if err := dc.ctx.DrawTextureEx(tex, gogpu.DrawTextureOptions{
		X:      rect.x,
		Y:      rect.y,
		Width:  rect.w,
		Height: rect.h,
		Alpha:  alpha,
	}); err != nil {
		slog.Error("DrawCharacterAlpha: draw failed", "num", num, "alpha", alpha, "error", err)
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
	rect := dc.screenPicRect(x, y, 8, 8)
	if err := dc.ctx.DrawTextureScaled(tex, rect.x, rect.y, rect.w, rect.h); err != nil {
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

type pixelRect struct {
	x float32
	y float32
	w float32
	h float32
}

func (dc *DrawContext) screenPicRect(x, y, w, h int) pixelRect {
	screenX, screenY, screenW, screenH := dc.canvasRectToScreen(x, y, w, h)
	return pixelRect{
		x: float32(screenX),
		y: float32(screenY),
		w: float32(screenW),
		h: float32(screenH),
	}
}

func (dc *DrawContext) canvasRectToScreen(x, y, w, h int) (screenX, screenY, screenW, screenH int) {
	if dc == nil || w <= 0 || h <= 0 || dc.canvas.Type == CanvasNone {
		return x, y, w, h
	}
	params := dc.canvasParams
	if dc.renderer != nil {
		rendererW, rendererH := dc.renderer.Size()
		if params.GUIWidth <= 0 {
			params.GUIWidth = float32(rendererW)
		}
		if params.GUIHeight <= 0 {
			params.GUIHeight = float32(rendererH)
		}
	}
	if params.GUIWidth <= 0 || params.GUIHeight <= 0 {
		return x, y, w, h
	}
	left, top := transformCanvasPointToScreen(dc.canvas.Transform, params.GUIWidth, params.GUIHeight, float32(x), float32(y))
	right, bottom := transformCanvasPointToScreen(dc.canvas.Transform, params.GUIWidth, params.GUIHeight, float32(x+w), float32(y+h))
	if left > right {
		left, right = right, left
	}
	if top > bottom {
		top, bottom = bottom, top
	}
	return int(left), int(top), int(right - left), int(bottom - top)
}

func transformCanvasPointToScreen(transform DrawTransform, screenW, screenH, x, y float32) (screenX, screenY float32) {
	ndcX := x*transform.Scale[0] + transform.Offset[0]
	ndcY := y*transform.Scale[1] + transform.Offset[1]
	screenX = (ndcX + 1) * 0.5 * screenW
	screenY = (1 - (ndcY+1)*0.5) * screenH
	return screenX, screenY
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
	worldVertexBuffer       hal.Buffer
	worldIndexBuffer        hal.Buffer
	worldIndexCount         uint32
	worldPipeline           hal.RenderPipeline
	worldTurbulentPipeline  hal.RenderPipeline
	worldSkyPipeline        hal.RenderPipeline
	worldPipelineLayout     hal.PipelineLayout
	worldBindGroup          hal.BindGroup
	worldShader             hal.ShaderModule
	uniformBuffer           hal.Buffer
	uniformBindGroup        hal.BindGroup
	uniformBindGroupLayout  hal.BindGroupLayout
	textureBindGroupLayout  hal.BindGroupLayout
	worldTextureSampler     hal.Sampler
	worldTextures           map[int32]*gpuWorldTexture
	worldFullbrightTextures map[int32]*gpuWorldTexture
	worldSkySolidTextures   map[int32]*gpuWorldTexture
	worldSkyAlphaTextures   map[int32]*gpuWorldTexture
	worldTextureAnimations  []*SurfaceTexture
	whiteTextureBindGroup   hal.BindGroup
	transparentTexture      hal.Texture
	transparentTextureView  hal.TextureView
	transparentBindGroup    hal.BindGroup
	worldLightmapSampler    hal.Sampler
	worldLightmapPages      []*gpuWorldTexture
	whiteLightmapBindGroup  hal.BindGroup
	worldLightStyleValues   [64]float32

	// 1x1 white texture for fallback
	whiteTexture          hal.Texture
	whiteTextureView      hal.TextureView
	worldDepthTexture     hal.Texture
	worldDepthTextureView hal.TextureView

	// Offscreen render target for world rendering
	worldRenderTexture            hal.Texture
	worldRenderTextureView        hal.TextureView
	worldRenderTextureGogpu       *gogpu.Texture // gogpu-wrapped version for compositing
	worldRenderWidth              int
	worldRenderHeight             int
	sceneCompositePipeline        hal.RenderPipeline
	sceneCompositePipelineLayout  hal.PipelineLayout
	sceneCompositeVertexShader    hal.ShaderModule
	sceneCompositeFragmentShader  hal.ShaderModule
	sceneCompositeBindGroupLayout hal.BindGroupLayout
	sceneCompositeSampler         hal.Sampler
	sceneCompositeUniformBuffer   hal.Buffer
	sceneCompositeBindGroup       hal.BindGroup

	// Alias-model resources for the gogpu backend.
	brushModelGeometry          map[int]*WorldGeometry
	aliasModels                 map[string]*gpuAliasModel
	spriteModels                map[string]*gpuSpriteModel
	aliasEntityStates           map[int]*AliasEntity
	viewModelAliasState         *AliasEntity
	aliasShadowSkin             *gpuAliasSkin
	aliasScratchBuffer          hal.Buffer
	aliasScratchBufferSize      uint64
	aliasPipeline               hal.RenderPipeline
	aliasPipelineLayout         hal.PipelineLayout
	aliasVertexShader           hal.ShaderModule
	aliasFragmentShader         hal.ShaderModule
	aliasUniformBuffer          hal.Buffer
	aliasUniformBindGroup       hal.BindGroup
	aliasUniformBindGroupLayout hal.BindGroupLayout
	aliasTextureBindGroupLayout hal.BindGroupLayout
	aliasSampler                hal.Sampler
	spriteUniformBuffer         hal.Buffer
	spriteUniformBindGroup      hal.BindGroup
	spritePipeline              hal.RenderPipeline
	spriteVertexShader          hal.ShaderModule
	spriteFragmentShader        hal.ShaderModule
	decalPipeline               hal.RenderPipeline
	decalPipelineLayout         hal.PipelineLayout
	decalVertexShader           hal.ShaderModule
	decalFragmentShader         hal.ShaderModule
	decalUniformBuffer          hal.Buffer
	decalUniformBindGroup       hal.BindGroup
	decalUniformLayout          hal.BindGroupLayout
	decalAtlasTextureHAL        hal.Texture
	decalAtlasView              hal.TextureView
	decalBindGroup              hal.BindGroup
	polyBlendPipeline           hal.RenderPipeline
	polyBlendPipelineLayout     hal.PipelineLayout
	polyBlendVertexShader       hal.ShaderModule
	polyBlendFragmentShader     hal.ShaderModule
	polyBlendUniformBuffer      hal.Buffer
	polyBlendBindGroupLayout    hal.BindGroupLayout
	polyBlendBindGroup          hal.BindGroup
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
		app:                app,
		config:             cfg,
		textureCache:       make(map[cacheKey]*cachedTexture),
		brushModelGeometry: make(map[int]*WorldGeometry),
		aliasModels:        make(map[string]*gpuAliasModel),
		spriteModels:       make(map[string]*gpuSpriteModel),
		aliasEntityStates:  make(map[int]*AliasEntity),
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
	r.clearAliasModelsLocked()
	r.clearSpriteModelsLocked()
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
//	    dc.Clear(0.1, 0.1, 0.1, 1.0)
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
	r.mu.Lock()
	r.brushModelGeometry = nil
	r.destroyAliasResourcesLocked()
	r.destroySpriteResourcesLocked()
	r.destroyDecalResourcesLocked()
	r.destroyPolyBlendResourcesLocked()
	r.destroyWorldRenderTargetLocked()
	r.destroySceneCompositeResourcesLocked()
	r.mu.Unlock()
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

	// CSQCDrawHud indicates CSQC is drawing HUD this frame.
	CSQCDrawHud bool

	// Particles is the active particle system to render
	Particles *ParticleSystem

	// Palette for color conversion
	Palette []byte

	// WaterWarp, WaterWarpTime, ForceUnderwater: see stubs_opengl.go for semantics.
	// GoGPU uses these for scene-target waterwarp/composite behavior and related runtime parity checks.
	WaterWarp       bool
	WaterWarpTime   float32
	ForceUnderwater bool

	// VBlend is the composite RGBA screen tint from client color shifts.
	// Applied after the 3D scene and entity passes, before the 2D overlay.
	VBlend [4]float32
}

// RenderFrame executes the complete frame pipeline in order:
// 1. Clear screen
// 2. Draw 3D world / scene target
// 3. Draw entities
// 4. Draw particles / viewmodel / polyblend
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
	sceneTargetActive := state.DrawWorld && state.WaterWarp && dc.enableSceneRenderTarget()

	// Phase 2: Draw 3D world directly to surface view (zero-copy)
	// HAL renders to dc.ctx.SurfaceView() which is the current frame's swapchain texture.
	// Then gogpu draws 2D overlay on top with LoadOpLoad to preserve the world.
	if state.DrawWorld {
		dc.renderer.setGoGPUWorldLightStyleValues(state.LightStyles)
		slog.Info("RenderFrame: rendering world to surface")
		dc.renderWorld(state)
		slog.Info("RenderFrame: surface view (after world)", "id", debugSurfaceViewID(dc.ctx.SurfaceView()))
		if !sceneTargetActive && dc.markGoGPUFrameContentForOverlay() {
			slog.Info("RenderFrame: marked gogpu frame as pre-populated (HAL world rendered)")
			if frameCleared, hasPendingClear, ok := dc.getGoGPUFrameStateForDebug(); ok {
				slog.Info("RenderFrame: gogpu frame state (after mark)", "frameCleared", frameCleared, "hasPendingClear", hasPendingClear)
			}
		} else if !sceneTargetActive {
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

	if shouldClearGoGPUSharedDepthStencil(gogpuSharedDepthStencilClearInputs{
		drawWorld:         state.DrawWorld,
		drawEntities:      state.DrawEntities,
		hasAliasEntities:  len(state.AliasEntities) > 0,
		hasSpriteEntities: len(state.SpriteEntities) > 0,
		hasDecalMarks:     len(state.DecalMarks) > 0,
		hasViewModel:      state.ViewModel != nil,
	}) {
		dc.clearGoGPUSharedDepthStencil()
	}

	// Phase 3: Draw entities, decals, and mode-placed particles.
	if state.DrawEntities || len(state.DecalMarks) > 0 || (state.DrawParticles && state.Particles != nil) {
		dc.renderEntities(state)
	}

	if state.DrawEntities && state.ViewModel != nil {
		dc.renderViewModelHAL(*state.ViewModel, state.FogColor, state.FogDensity)
	}
	if sceneTargetActive {
		if dc.compositeSceneRenderTarget(state.WaterWarp, state.WaterWarpTime, state.ClearColor) {
			if dc.markGoGPUFrameContentForOverlay() {
				slog.Info("RenderFrame: marked gogpu frame as pre-populated (scene composite rendered)")
			} else {
				slog.Warn("RenderFrame: unable to mark gogpu frame state after scene composite")
			}
		} else {
			slog.Warn("RenderFrame: failed to composite scene render target")
		}
		dc.disableSceneRenderTarget()
	}
	if state.VBlend[3] > 0 {
		dc.renderPolyBlendHAL(state.VBlend)
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

// renderEntities draws runtime entities. Alias models, sprites, and decals use
// HAL-backed paths.
func (dc *DrawContext) renderEntities(state *RenderFrameState) {
	if dc == nil || dc.renderer == nil || state == nil {
		return
	}
	particlePhase, hasParticlePhase := classifyGoGPUParticlePhase(readGoGPUParticleModeCvar(), particleCount(state.Particles))
	plan := planGoGPUEntityDrawOrder(state.DrawEntities, state.BrushEntities, state.AliasEntities, state.SpriteEntities, state.DecalMarks, particlePhase, hasParticlePhase)
	for _, phase := range plan.phases {
		switch phase {
		case gogpuEntityPhaseOpaqueBrush:
			dc.renderBrushEntityMarkers(plan.opaqueBrush, true, false)
		case gogpuEntityPhaseOpaqueAlias:
			for _, step := range gogpuOpaqueAliasPassSteps() {
				switch step {
				case gogpuOpaqueAliasStepEntities:
					dc.renderAliasEntitiesHAL(plan.opaqueAlias, state.FogColor, state.FogDensity)
				case gogpuOpaqueAliasStepShadows:
					dc.renderAliasShadowsHAL(plan.opaqueAlias, state.FogColor, state.FogDensity)
				}
			}
		case gogpuEntityPhaseOpaqueParticles:
			if state.DrawParticles && state.Particles != nil {
				dc.renderParticles(state)
			}
		case gogpuEntityPhaseSkyBrush:
			dc.renderBrushEntityMarkers(plan.skyBrush, true, true)
		case gogpuEntityPhaseTranslucentBrush:
			dc.renderBrushEntityMarkers(plan.translucentBrush, false, false)
		case gogpuEntityPhaseDecals:
			dc.renderDecalMarksHAL(state.DecalMarks)
		case gogpuEntityPhaseTranslucentAlias:
			dc.renderAliasEntitiesHAL(plan.translucentAlias, state.FogColor, state.FogDensity)
		case gogpuEntityPhaseSprites:
			dc.renderSpriteEntitiesHAL(state.SpriteEntities, state.FogColor, state.FogDensity)
		case gogpuEntityPhaseTranslucentParticles:
			if state.DrawParticles && state.Particles != nil {
				dc.renderParticles(state)
			}
		}
	}
}

const (
	gogpuBrushMarkerColor = byte(250)
	gogpuBrushMarkerSize  = 2
)

func (dc *DrawContext) drawProjectedEntityMarker(pos [3]float32, vp types.Mat4, screenW, screenH, size int, color byte) {
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

func projectWorldPointToScreen(pos [3]float32, vp types.Mat4, screenW, screenH int) (x int, y int, ok bool) {
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

type projectedParticleMarker struct {
	x     int
	y     int
	color byte
	size  int
	alpha float32
}

func projectParticleMarkers(particles []Particle, verts []ParticleVertex, vp types.Mat4, screenW, screenH int) []projectedParticleMarker {
	if len(particles) == 0 || len(verts) == 0 {
		return nil
	}
	count := len(particles)
	if len(verts) < count {
		count = len(verts)
	}
	markers := make([]projectedParticleMarker, 0, count)
	for i := 0; i < count; i++ {
		x, y, ok := projectWorldPointToScreen(verts[i].Pos, vp, screenW, screenH)
		if !ok {
			continue
		}
		markers = append(markers, projectedParticleMarker{
			x:     x,
			y:     y,
			color: particles[i].Color,
			size:  4,
			alpha: 1,
		})
	}
	return markers
}

func (r *Renderer) ensureBrushModelGeometry(submodelIndex int) *WorldGeometry {
	if submodelIndex <= 0 {
		return nil
	}
	r.mu.RLock()
	if geom := r.brushModelGeometry[submodelIndex]; geom != nil {
		r.mu.RUnlock()
		return geom
	}
	tree := (*bsp.Tree)(nil)
	if r.worldData != nil && r.worldData.Geometry != nil {
		tree = r.worldData.Geometry.Tree
	}
	r.mu.RUnlock()
	if tree == nil {
		return nil
	}
	geom, err := BuildModelGeometry(tree, submodelIndex)
	if err != nil {
		slog.Debug("GoGPU brush model build skipped", "submodel", submodelIndex, "error", err)
		return nil
	}
	if geom == nil || len(geom.Vertices) == 0 {
		return nil
	}
	r.mu.Lock()
	if r.brushModelGeometry == nil {
		r.brushModelGeometry = make(map[int]*WorldGeometry)
	}
	if existing := r.brushModelGeometry[submodelIndex]; existing != nil {
		r.mu.Unlock()
		return existing
	}
	r.brushModelGeometry[submodelIndex] = geom
	r.mu.Unlock()
	return geom
}

func brushMarkerMatchesPhase(face WorldFace, entityAlpha float32, opaque, sky bool) bool {
	switch {
	case face.Flags&model.SurfDrawSky != 0:
		return sky
	case sky:
		return false
	case face.Flags&model.SurfDrawFence != 0:
		return opaque
	case face.Flags&(model.SurfDrawLava|model.SurfDrawSlime|model.SurfDrawTele|model.SurfDrawWater) != 0:
		return !opaque
	case entityAlpha < 1:
		return !opaque
	default:
		return opaque
	}
}

func (r *Renderer) projectBrushMarkers(entities []BrushEntity, vp types.Mat4, screenW, screenH int, opaque, sky bool) []projectedParticleMarker {
	if len(entities) == 0 || screenW <= 0 || screenH <= 0 {
		return nil
	}
	markers := make([]projectedParticleMarker, 0, len(entities)*8)
	for _, entity := range entities {
		geom := r.ensureBrushModelGeometry(entity.SubmodelIndex)
		if geom == nil || len(geom.Vertices) == 0 || len(geom.Faces) == 0 || len(geom.Indices) == 0 {
			continue
		}
		rotation := buildBrushRotationMatrix(entity.Angles)
		seen := make(map[uint32]struct{})
		for _, face := range geom.Faces {
			if !brushMarkerMatchesPhase(face, entity.Alpha, opaque, sky) {
				continue
			}
			first := int(face.FirstIndex)
			last := first + int(face.NumIndices)
			if first < 0 {
				first = 0
			}
			if last > len(geom.Indices) {
				last = len(geom.Indices)
			}
			for _, vertexIndex := range geom.Indices[first:last] {
				if _, ok := seen[vertexIndex]; ok {
					continue
				}
				seen[vertexIndex] = struct{}{}
				if int(vertexIndex) >= len(geom.Vertices) {
					continue
				}
				vertex := geom.Vertices[vertexIndex]
				worldPos := transformModelSpacePoint(vertex.Position, entity.Origin, rotation, entity.Scale)
				x, y, ok := projectWorldPointToScreen(worldPos, vp, screenW, screenH)
				if !ok {
					continue
				}
				markers = append(markers, projectedParticleMarker{
					x:     x,
					y:     y,
					color: gogpuBrushMarkerColor,
					size:  gogpuBrushMarkerSize,
					alpha: clamp01(entity.Alpha),
				})
			}
		}
	}
	return markers
}

func (dc *DrawContext) renderBrushEntityMarkers(entities []BrushEntity, opaque, sky bool) {
	if dc == nil || dc.renderer == nil || len(entities) == 0 {
		return
	}
	screenW, screenH := dc.renderer.Size()
	markers := dc.renderer.projectBrushMarkers(entities, dc.renderer.viewMatrices.VP, screenW, screenH, opaque, sky)
	for _, marker := range markers {
		size := marker.size
		if size < 1 {
			size = 1
		}
		x := marker.x - size/2
		y := marker.y - size/2
		if opaque || marker.alpha >= 1 {
			dc.DrawFill(x, y, size, size, marker.color)
			continue
		}
		if marker.alpha > 0 {
			dc.DrawFillAlpha(x, y, size, size, marker.color, marker.alpha)
		}
	}
}

// renderParticles draws the particle system.
func (dc *DrawContext) renderParticles(state *RenderFrameState) {
	if state.Particles == nil || state.Particles.ActiveCount() == 0 {
		return
	}
	if !shouldDrawGoGPUParticles(readGoGPUParticleModeCvar(), state.Particles.ActiveCount()) {
		return
	}

	// Get active particles
	particles := state.Particles.ActiveParticles()

	// Build particle vertices
	palette := buildParticlePalette(state.Palette)
	verts := BuildParticleVertices(particles, palette, false) // false = not showtris mode

	screenW, screenH := dc.renderer.Size()
	markers := projectParticleMarkers(particles, verts, dc.renderer.viewMatrices.VP, screenW, screenH)
	if len(markers) == 0 {
		return
	}

	// Draw each particle as a small colored quad
	// This is a simplified implementation - a proper implementation would use
	// instanced rendering or a point sprite shader
	for _, marker := range markers {
		size := marker.size
		if size < 1 {
			size = 1
		}
		if marker.alpha >= 1 {
			dc.DrawFill(marker.x-size/2, marker.y-size/2, size, size, marker.color)
			continue
		}
		if marker.alpha > 0 {
			dc.DrawFillAlpha(marker.x-size/2, marker.y-size/2, size, size, marker.color, marker.alpha)
		}
	}
}

func readGoGPUParticleModeCvar() int {
	cv := cvar.Get(CvarRParticles)
	if cv == nil {
		return 1
	}
	return cv.Int
}

func shouldDrawGoGPUParticles(mode, activeParticles int) bool {
	return ShouldDrawParticles(mode, false, false, activeParticles) || ShouldDrawParticles(mode, true, false, activeParticles)
}

func particleCount(ps *ParticleSystem) int {
	if ps == nil {
		return 0
	}
	return ps.ActiveCount()
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
		DrawParticles: false,                          // Opt-in per frame like the primary renderer
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

	r.viewMatrices.Projection = ComputeProjectionMatrix(projectionFOVForCamera(camera), aspect, nearPlane, farPlane)

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
	r.viewMatrices.VP = types.Mat4Multiply(r.viewMatrices.Projection, r.viewMatrices.View)

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
func (r *Renderer) GetViewMatrix() types.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.View
}

// GetProjectionMatrix returns the currently cached projection matrix.
// Thread-safe read.
func (r *Renderer) GetProjectionMatrix() types.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.Projection
}

// GetViewProjectionMatrix returns the combined View × Projection matrix.
// This is the matrix typically used in vertex shaders for world-to-NDC transformation.
// Thread-safe read.
func (r *Renderer) GetViewProjectionMatrix() types.Mat4 {
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

func (r *Renderer) SpawnKeyedDynamicLight(light DynamicLight) bool {
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
