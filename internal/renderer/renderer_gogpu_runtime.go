package renderer

import (
	"fmt"
	stdimage "image"
	"image/color"
	"image/png"
	"log/slog"
	"os"

	"github.com/gogpu/gogpu"
	"github.com/gogpu/gogpu/input"
)

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

	// Apply VSync setting from engine config
	gpuCfg = gpuCfg.WithVSync(cfg.VSync)

	// Log GPU preference. gogpu doesn't yet expose PowerPreference on its
	// windowed runtime path, so our explicit runtime preference currently
	// works by force-applying loader hints before renderer initialization.
	// The headless Core path uses RequestAdapter PowerPreference directly.
	switch cfg.GPUPreference {
	case GPUPreferHighPerformance:
		slog.Info("GPU preference: high-performance (discrete)")
	case GPUPreferLowPower:
		slog.Info("GPU preference: low-power (integrated)")
	default:
		slog.Info("GPU preference: auto")
	}
	applyGPUPreferenceRuntimeEnv(cfg.GPUPreference)

	// Use Pure Go backend (no CGO required)
	gpuCfg = gpuCfg.WithBackend(gogpu.BackendGo)

	// Create the gogpu application
	app := gogpu.NewApp(gpuCfg)

	r := &Renderer{
		app:                 app,
		config:              cfg,
		textureCache:        make(map[cacheKey]*cachedTexture),
		lightPool:           NewGLLightPool(512),
		brushModelGeometry:  make(map[int]*WorldGeometry),
		brushModelLightmaps: make(map[int][]*gpuWorldTexture),
		aliasModels:         make(map[string]*gpuAliasModel),
		spriteModels:        make(map[string]*gpuSpriteModel),
		aliasEntityStates:   make(map[int]*AliasEntity),
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

func applyGPUPreferenceRuntimeEnv(pref GPUPreference) {
	var overrides []struct {
		key   string
		value string
	}
	switch pref {
	case GPUPreferHighPerformance:
		overrides = []struct {
			key   string
			value string
		}{
			{key: "DRI_PRIME", value: "1"},
			{key: "__NV_PRIME_RENDER_OFFLOAD", value: "1"},
			{key: "__VK_LAYER_NV_optimus", value: "NVIDIA_only"},
		}
	case GPUPreferLowPower:
		overrides = []struct {
			key   string
			value string
		}{
			{key: "DRI_PRIME", value: "0"},
			{key: "__NV_PRIME_RENDER_OFFLOAD", value: "0"},
			{key: "__VK_LAYER_NV_optimus", value: "non_NVIDIA_only"},
		}
	default:
		return
	}
	for _, override := range overrides {
		prev, hadPrev := os.LookupEnv(override.key)
		if hadPrev && prev == override.value {
			slog.Info("GPU preference override already applied", "env", override.key, "value", override.value)
			continue
		}
		if err := os.Setenv(override.key, override.value); err != nil {
			slog.Warn("failed to apply GPU preference override", "env", override.key, "value", override.value, "error", err)
			continue
		}
		if hadPrev {
			slog.Info("Overrode GPU preference hint", "env", override.key, "previous", prev, "value", override.value)
			continue
		}
		slog.Info("Applied GPU preference override", "env", override.key, "value", override.value)
	}
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
			slog.Debug(
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
	if r == nil {
		return 0, 0
	}
	if r.app == nil {
		return r.config.Width, r.config.Height
	}
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

// CaptureScreenshot exports a minimal deterministic PNG for GoGPU builds.
// Full swapchain readback is intentionally deferred until the backend exposes
// a stable cross-platform texture readback path.
func (r *Renderer) CaptureScreenshot(filename string) error {
	width, height := r.Size()
	if width <= 0 {
		width = r.config.Width
	}
	if height <= 0 {
		height = r.config.Height
	}
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}

	img := stdimage.NewNRGBA(stdimage.Rect(0, 0, width, height))
	fill := color.NRGBA{R: 20, G: 20, B: 46, A: 255}
	for y := 0; y < height; y++ {
		rowStart := y * img.Stride
		row := img.Pix[rowStart : rowStart+width*4]
		for x := 0; x < width; x++ {
			idx := x * 4
			row[idx+0] = fill.R
			row[idx+1] = fill.G
			row[idx+2] = fill.B
			row[idx+3] = fill.A
		}
	}

	f, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("capture screenshot: create file: %w", err)
	}
	defer f.Close()

	if err := png.Encode(f, img); err != nil {
		return fmt.Errorf("capture screenshot: encode png: %w", err)
	}
	return nil
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
	r.destroyParticleResourcesLocked()
	r.destroyDecalResourcesLocked()
	r.destroyPolyBlendResourcesLocked()
	r.destroyOverlayCompositeResourcesLocked()
	r.destroyWorldRenderTargetLocked()
	r.destroySceneCompositeResourcesLocked()
	if r.overlayTexture != nil {
		r.overlayTexture.Destroy()
		r.overlayTexture = nil
	}
	r.overlayPixelBuf = nil
	r.overlayUploadBuf = nil
	r.overlayTextureDirtyX = 0
	r.overlayTextureDirtyY = 0
	r.overlayTextureDirtyW = 0
	r.overlayTextureDirtyH = 0
	r.overlayTextureDirtyValid = false
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
