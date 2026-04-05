package renderer

import (
	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/image"
)

// Package renderer provides GPU-accelerated rendering for the Ironwail-Go engine.
// The supported runtime backend is selected at build time via build tags.
//
// Build tags:
//   - gogpu: Uses gogpu library for GPU acceleration (no CGO required)
//   - (no tags): Returns an explicit no-backend error
//
// To build with gogpu backend:
//
//	go build -tags=gogpu ./...
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

	// DrawPic renders a QPic image at the specified screen-space position.
	DrawPic(x, y int, pic *image.QPic)

	// DrawMenuPic renders a QPic image in 320x200 menu-space coordinates.
	DrawMenuPic(x, y int, pic *image.QPic)

	// DrawFill fills a rectangle with a Quake palette color.
	DrawFill(x, y, w, h int, color byte)

	// DrawFillAlpha fills a rectangle with a Quake palette color and explicit alpha.
	DrawFillAlpha(x, y, w, h int, color byte, alpha float32)

	// DrawCharacter renders a single character from the font.
	DrawCharacter(x, y int, num int)

	// DrawMenuCharacter renders a single menu character in 320x200 menu-space coordinates.
	DrawMenuCharacter(x, y int, num int)

	// SetCanvas switches the active 2D canvas coordinate system.
	// Subsequent draw calls use coordinates in the canvas's logical space.
	SetCanvas(ct CanvasType)

	// Canvas returns the current canvas state including type, transform, and bounds.
	Canvas() CanvasState
}

// Backend manages the graphics backend and window.
// It handles the main game loop, callbacks, and window state.
type Backend interface {
	// Lifecycle
	New(cfg Config) error
	Shutdown()
	Run() error
	Stop()
	CaptureScreenshot(filename string) error

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
	CvarVidWidth       = "vid_width"       // Video width in pixels (default: 1920)
	CvarVidHeight      = "vid_height"      // Video height in pixels (default: 1080)
	CvarVidFullscreen  = "vid_fullscreen"  // Fullscreen mode: 0=windowed, 1=fullscreen (default: 1)
	CvarVidVsync       = "vid_vsync"       // Vertical sync: 0=off, 1=on (default: 1)
	CvarHostMaxFPS     = "host_maxfps"     // Maximum frames per second (default: 250)
	CvarRGamma         = "r_gamma"         // Gamma correction value (default: 1.0)
	CvarRAlphaSort     = "r_alphasort"     // Alpha surface sorting (0=basic, 1=sorted)
	CvarROIT           = "r_oit"           // Order-independent transparency mode (0=off, 1=on)
	CvarRWaterAlpha    = "r_wateralpha"    // Water alpha (0..1, default 1.0)
	CvarRLavaAlpha     = "r_lavaalpha"     // Lava alpha (0 uses water alpha)
	CvarRSlimeAlpha    = "r_slimealpha"    // Slime alpha (0 uses water alpha)
	CvarRTeleAlpha     = "r_telealpha"     // Teleport alpha (0 uses water alpha)
	CvarRParticles     = "r_particles"     // Particle blend mode (1=alpha, 2=opaque)
	CvarRDynamic       = "r_dynamic"       // Dynamic lights (0=off, 1=on)
	CvarRFastSky       = "r_fastsky"       // Fast sky mode (flat sky color, no scrolling layers)
	CvarRProceduralSky = "r_proceduralsky" // Procedural sky baseline for embedded fast-sky rendering (0=off, 1=on)
	CvarRSkyFog        = "r_skyfog"        // Sky fog mix factor (0..1, default 0.5)
	CvarRSkySolidSpeed = "r_skysolidspeed" // Embedded sky solid-layer speed scale (default: 1.0)
	CvarRSkyAlphaSpeed = "r_skyalphaspeed" // Embedded sky alpha-layer speed scale (default: 1.0)
	CvarRShadows       = "r_shadows"       // Entity shadows (0=off, 1=on)
	CvarRNoLerpList    = "r_nolerp_list"
	CvarRNoshadowList  = "r_noshadow_list"
	CvarGLTextureMode  = "gl_texturemode"
	CvarGLLodBias      = "gl_lodbias"
	CvarGLAnisotropy   = "gl_texture_anisotropy"
	// CvarRWaterwarp controls the underwater visual warp effect.
	// 0 = off (no underwater visual effect)
	// 1 = screen-space sinusoidal warp (post-process distortion of the rendered scene)
	// 2 = FOV-based warp (oscillates horizontal/vertical FOV while underwater)
	// Mirrors C Ironwail r_waterwarp: values >1 use FOV modulation, value 1 uses screen warp.
	CvarRWaterwarp   = "r_waterwarp"
	CvarRLitWater    = "r_litwater"        // Lit water: 1=lightmapped water surfaces, 0=unlit (default: 1)
	CvarVidGPUPrefer = "vid_gpupreference" // GPU preference: 0=high-performance (discrete), 1=low-power (integrated), 2=auto
)

// GPUPreference controls which adapter type is preferred when multiple GPUs
// are available (e.g. integrated + discrete on a laptop).
type GPUPreference int

const (
	// GPUPreferHighPerformance prefers discrete GPUs over integrated (default).
	GPUPreferHighPerformance GPUPreference = iota
	// GPUPreferLowPower prefers integrated GPUs for battery life.
	GPUPreferLowPower
	// GPUPreferAuto lets the driver/runtime choose.
	GPUPreferAuto
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

	// GPUPreference controls adapter selection when multiple GPUs are
	// available (e.g. integrated + discrete on a laptop).
	// 0 = high-performance (prefer discrete GPU, default)
	// 1 = low-power (prefer integrated GPU)
	// 2 = auto (let driver choose)
	GPUPreference GPUPreference
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
	if cv := cvar.Get(CvarVidGPUPrefer); cv != nil {
		switch cv.Int {
		case 1:
			cfg.GPUPreference = GPUPreferLowPower
		case 2:
			cfg.GPUPreference = GPUPreferAuto
		default:
			cfg.GPUPreference = GPUPreferHighPerformance
		}
	}

	return cfg
}
