package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/ironwail/ironwail-go/internal/audio"
	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/host"
	"github.com/ironwail/ironwail-go/internal/hud"
	qimage "github.com/ironwail/ironwail-go/internal/image"
	"github.com/ironwail/ironwail-go/internal/input"
	"github.com/ironwail/ironwail-go/internal/menu"
	"github.com/ironwail/ironwail-go/internal/model"
	inet "github.com/ironwail/ironwail-go/internal/net"
	"github.com/ironwail/ironwail-go/internal/qc"
	"github.com/ironwail/ironwail-go/internal/renderer"
	"github.com/ironwail/ironwail-go/internal/server"
)

const (
	VersionMajor = 0
	VersionMinor = 2
	VersionPatch = 0

	runtimeMaxPredictedXYOffset = 4.0
)

// Game consolidates all top-level engine state into a single struct.
// Previously these were scattered package-level variables; grouping them
// here makes ownership, lifetime, and dependencies explicit.
type Game struct {
	Host       *host.Host
	Server     *server.Server
	QC         *qc.VM
	CSQC       *qc.CSQC // Client-side QuakeC VM (nil when not loaded)
	Renderer   *renderer.Renderer
	Subs       *host.Subsystems
	Client     *cl.Client
	Particles  *renderer.ParticleSystem
	DecalMarks *renderer.DecalMarkSystem

	ParticleRNG  *rand.Rand
	ParticleTime float32
	RuntimeBeams []cl.BeamSegment

	Menu  *menu.Manager
	Input *input.System
	Draw  *draw.Manager
	HUD   *hud.HUD
	Audio *audio.AudioAdapter

	MouseGrabbed     bool
	AliasModelCache  map[string]*model.Model
	SpriteModelCache map[string]*runtimeSpriteModel
	SoundSFXByIndex  map[int]*audio.SFX
	MenuSFXByName    map[string]*audio.SFX
	AmbientSFX       [audio.NumAmbients]*audio.SFX
	SoundPrecacheKey string
	StaticSoundKey   string
	MusicTrackKey    string
	SkyboxNameKey    string
	WorldUploadKey   string
	ShowScores       bool
	ModDir           string

	CameraInLiquid     bool
	CameraLeafContents int32

	// Scope zoom state, updated each frame via renderer.UpdateZoom.
	Zoom    float32
	ZoomDir float32

	ConsoleSlideFraction float32
	FPSOverlay           runtimeFPSOverlay
	SpeedOverlay         runtimeSpeedOverlay
	DemoOverlay          runtimeDemoOverlay
	TurtleOverlayCount   int
	LastServerMessageAt  float64
}

var g Game
var runtimeNow = time.Now

type canvasParamSetter interface {
	SetCanvasParams(renderer.CanvasTransformParams)
}

type defaultBinding struct {
	key     int
	command string
}

type runtimeFPSOverlay struct {
	oldTime       float64
	lastFPS       float64
	oldFrameCount int
}

type runtimeSpeedOverlay struct {
	maxSpeed     float32
	displaySpeed float32
	lastRealTime float64
}

type runtimeDemoOverlay struct {
	prevSpeed     float32
	prevBaseSpeed float32
	showTime      float64
}

type runtimeTelemetryState struct {
	RealTime        float64
	FrameCount      int
	FrameTime       float64
	ViewSize        float32
	HUDStyle        int
	ShowFPS         float32
	ShowClock       int
	ShowSpeed       bool
	ShowTurtle      bool
	ShowSpeedOfs    float32
	ClientTime      float64
	Intermission    int
	InCutscene      bool
	DemoPlayback    bool
	DemoSpeed       float32
	DemoBaseSpeed   float32
	DemoProgress    float64
	DemoName        string
	DemoBarTimeout  float32
	ClientActive    bool
	Velocity        [3]float32
	ConsoleForced   bool
	LastServerMsgAt float64
	SavingActive    bool
	ViewRect        renderer.ViewRect
}

type runtimeSpriteModel struct {
	model  *model.Model
	sprite *model.MSprite
}

var gameplayDefaultBindings = []defaultBinding{
	{key: int('`'), command: "toggleconsole"},
	{key: int('w'), command: "+forward"},
	{key: input.KUpArrow, command: "+forward"},
	{key: int('s'), command: "+back"},
	{key: input.KDownArrow, command: "+back"},
	{key: int('a'), command: "+moveleft"},
	{key: int('d'), command: "+moveright"},
	{key: input.KLeftArrow, command: "+left"},
	{key: input.KRightArrow, command: "+right"},
	{key: input.KShift, command: "+speed"},
	{key: input.KAlt, command: "+strafe"},
	{key: input.KTab, command: "+showscores"},
	{key: input.KCtrl, command: "+attack"},
	{key: input.KMouse1, command: "+attack"},
	{key: input.KSpace, command: "+jump"},
	{key: input.KMouse2, command: "+jump"},
	{key: int('e'), command: "+use"},
	{key: input.KMouse3, command: "+mlook"},
	{key: input.KMWheelUp, command: "impulse 10"},
	{key: input.KMWheelDown, command: "impulse 12"},
}

type globalCommandBuffer struct{}

func (globalCommandBuffer) Init()    {}
func (globalCommandBuffer) Execute() { cmdsys.Execute() }
func (globalCommandBuffer) ExecuteWithSource(source cmdsys.CommandSource) {
	cmdsys.ExecuteWithSource(source)
}
func (globalCommandBuffer) AddText(text string) { cmdsys.AddText(text) }
func (globalCommandBuffer) InsertText(text string) {
	cmdsys.InsertText(text)
}
func (globalCommandBuffer) Shutdown() {}

func runtimeGUIDimensions(framebufferW, framebufferH int) (int, int) {
	guiW := cvar.IntValue("vid_width")
	guiH := cvar.IntValue("vid_height")
	if guiW <= 0 {
		guiW = framebufferW
	}
	if guiH <= 0 {
		guiH = framebufferH
	}
	return guiW, guiH
}

func runtimeConsoleDimensions(guiW, guiH int) (int, int) {
	if guiW <= 0 || guiH <= 0 {
		return 0, 0
	}
	conWidth := guiW
	if override := cvar.FloatValue("scr_conwidth"); override > 0 {
		conWidth = int(override)
	} else if scale := cvar.FloatValue("scr_conscale"); scale > 0 {
		conWidth = int(float64(guiW) / scale)
	}
	if conWidth < 320 {
		conWidth = 320
	}
	if conWidth > guiW {
		conWidth = guiW
	}
	conWidth &^= 7
	if conWidth <= 0 {
		conWidth = guiW
	}
	conHeight := conWidth * guiH / guiW
	if conHeight <= 0 {
		conHeight = guiH
	}
	return conWidth, conHeight
}

func runtimeConsoleCanvasParams(framebufferW, framebufferH int, slideFraction float32) renderer.CanvasTransformParams {
	guiW, guiH := runtimeGUIDimensions(framebufferW, framebufferH)
	conW, conH := runtimeConsoleDimensions(guiW, guiH)
	return renderer.CanvasTransformParams{
		GUIWidth:         float32(guiW),
		GUIHeight:        float32(guiH),
		GLWidth:          float32(framebufferW),
		GLHeight:         float32(framebufferH),
		ConWidth:         float32(conW),
		ConHeight:        float32(conH),
		ConSlideFraction: slideFraction,
	}
}

func runtimeConsoleBackgroundPic() *qimage.QPic {
	if g.Draw == nil {
		return nil
	}
	return g.Draw.GetPic("gfx/conback.lmp")
}

func drawRuntimeConsole(overlay renderer.RenderContext, framebufferW, framebufferH int, full, forcedup bool) {
	slideFraction := clampUnitFloat32(g.ConsoleSlideFraction)
	if forcedup {
		slideFraction = 1
	}
	params := runtimeConsoleCanvasParams(framebufferW, framebufferH, slideFraction)
	if setter, ok := overlay.(canvasParamSetter); ok {
		setter.SetCanvasParams(params)
	}
	overlay.SetCanvas(renderer.CanvasConsole)
	var background *qimage.QPic
	if full {
		background = runtimeConsoleBackgroundPic()
	}
	console.Draw(overlay, int(params.ConWidth), int(params.ConHeight), full, background, forcedup)
}

func updateRuntimeConsoleSlide(dt float64, consoleVisible, forcedup bool) {
	if forcedup {
		g.ConsoleSlideFraction = 1
		return
	}

	target := float32(0)
	if consoleVisible {
		target = 1
	}
	if dt <= 0 {
		g.ConsoleSlideFraction = target
		return
	}

	speed := float32(cvar.FloatValue("scr_conspeed"))
	if speed <= 0 {
		speed = 1e6
	}
	step := speed * float32(dt) / 300
	current := clampUnitFloat32(g.ConsoleSlideFraction)
	if current < target {
		current = min(current+step, target)
	} else if current > target {
		current = max(current-step, target)
	}
	g.ConsoleSlideFraction = clampUnitFloat32(current)
}

func runtimeConsoleAnimating() bool {
	return g.ConsoleSlideFraction > 0
}

func startupMapArg(args []string) string {
	for i := 0; i < len(args); i++ {
		if args[i] == "+map" && i+1 < len(args) {
			return args[i+1]
		}
	}
	if len(args) > 0 && args[0] != "" && !strings.HasPrefix(args[0], "+") {
		return args[0]
	}
	return ""
}

func shouldUploadRuntimeWorld(uploadedKey, targetKey string, hasWorldData bool) bool {
	if targetKey == "" {
		return false
	}
	if !hasWorldData {
		return true
	}
	return uploadedKey != targetKey
}

type csqcClipRect struct {
	enabled bool
	x       float32
	y       float32
	width   float32
	height  float32
}

func clampUnitFloat32(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func lookupCSQCPic(name string) *qimage.QPic {
	if g.Draw == nil {
		return nil
	}
	return g.Draw.GetPic(name)
}

func nearestPaletteIndex(r, g, b float32, palette []byte) byte {
	if len(palette) < 3 {
		return 0
	}

	targetR := int(clampUnitFloat32(r)*255 + 0.5)
	targetG := int(clampUnitFloat32(g)*255 + 0.5)
	targetB := int(clampUnitFloat32(b)*255 + 0.5)

	bestIdx := 0
	bestDist := math.MaxInt
	for i := 0; i+2 < len(palette); i += 3 {
		dr := targetR - int(palette[i])
		dg := targetG - int(palette[i+1])
		db := targetB - int(palette[i+2])
		dist := dr*dr + dg*dg + db*db
		if dist < bestDist {
			bestDist = dist
			bestIdx = i / 3
		}
	}

	return byte(bestIdx)
}

func clipCSQCDrawRect(clip csqcClipRect, x, y, width, height float32) (drawX, drawY, drawW, drawH, srcX, srcY, srcW, srcH float32, ok bool) {
	if width <= 0 || height <= 0 {
		return 0, 0, 0, 0, 0, 0, 0, 0, false
	}

	drawX, drawY, drawW, drawH = x, y, width, height
	srcX, srcY, srcW, srcH = 0, 0, 1, 1
	if !clip.enabled {
		return drawX, drawY, drawW, drawH, srcX, srcY, srcW, srcH, true
	}

	left := max(x, clip.x)
	top := max(y, clip.y)
	right := min(x+width, clip.x+clip.width)
	bottom := min(y+height, clip.y+clip.height)
	if right <= left || bottom <= top {
		return 0, 0, 0, 0, 0, 0, 0, 0, false
	}

	drawX = left
	drawY = top
	drawW = right - left
	drawH = bottom - top
	srcX = (left - x) / width
	srcY = (top - y) / height
	srcW = drawW / width
	srcH = drawH / height
	return drawX, drawY, drawW, drawH, srcX, srcY, srcW, srcH, true
}

func subPicFromNormalizedRect(pic *qimage.QPic, srcX, srcY, srcW, srcH float32) *qimage.QPic {
	if pic == nil || pic.Width == 0 || pic.Height == 0 {
		return nil
	}

	startX := clampUnitFloat32(srcX)
	startY := clampUnitFloat32(srcY)
	endX := clampUnitFloat32(srcX + srcW)
	endY := clampUnitFloat32(srcY + srcH)
	if endX <= startX || endY <= startY {
		return &qimage.QPic{}
	}

	picWidth := float64(pic.Width)
	picHeight := float64(pic.Height)
	x1 := int(math.Floor(float64(startX) * picWidth))
	y1 := int(math.Floor(float64(startY) * picHeight))
	x2 := int(math.Ceil(float64(endX) * picWidth))
	y2 := int(math.Ceil(float64(endY) * picHeight))
	return pic.SubPic(x1, y1, x2-x1, y2-y1)
}

func scaleQPic(pic *qimage.QPic, width, height int) *qimage.QPic {
	if pic == nil || width <= 0 || height <= 0 || pic.Width == 0 || pic.Height == 0 {
		return nil
	}
	if int(pic.Width) == width && int(pic.Height) == height {
		return pic
	}

	srcW := int(pic.Width)
	srcH := int(pic.Height)
	scaled := &qimage.QPic{
		Width:  uint32(width),
		Height: uint32(height),
		Pixels: make([]byte, width*height),
	}
	for y := range height {
		srcY := y * srcH / height
		for x := range width {
			srcX := x * srcW / width
			scaled.Pixels[y*width+x] = pic.Pixels[srcY*srcW+srcX]
		}
	}
	return scaled
}

func prepareCSQCPic(pic *qimage.QPic, posX, posY, sizeX, sizeY, srcX, srcY, srcW, srcH float32, clip csqcClipRect) (int, int, *qimage.QPic, bool) {
	drawX, drawY, drawW, drawH, clipSrcX, clipSrcY, clipSrcW, clipSrcH, ok := clipCSQCDrawRect(clip, posX, posY, sizeX, sizeY)
	if !ok {
		return 0, 0, nil, false
	}

	srcX += srcW * clipSrcX
	srcY += srcH * clipSrcY
	srcW *= clipSrcW
	srcH *= clipSrcH

	subPic := subPicFromNormalizedRect(pic, srcX, srcY, srcW, srcH)
	if subPic == nil || subPic.Width == 0 || subPic.Height == 0 {
		return 0, 0, nil, false
	}

	drawPic := scaleQPic(subPic, int(drawW), int(drawH))
	if drawPic == nil || drawPic.Width == 0 || drawPic.Height == 0 {
		return 0, 0, nil, false
	}

	return int(drawX), int(drawY), drawPic, true
}

func buildCSQCDrawHooks(rc renderer.RenderContext) qc.CSQCDrawHooks {
	var clip csqcClipRect

	return qc.CSQCDrawHooks{
		IsCachedPic: func(name string) bool {
			return lookupCSQCPic(name) != nil
		},
		PrecachePic: func(name string, flags int) string {
			if g.CSQC != nil {
				g.CSQC.PrecachePic(name)
			}
			return name
		},
		GetImageSize: func(name string) (float32, float32) {
			pic := lookupCSQCPic(name)
			if pic == nil {
				return 0, 0
			}
			return float32(pic.Width), float32(pic.Height)
		},
		DrawCharacter: func(posX, posY float32, char int, sizeX, sizeY float32, r, g, b, alpha float32, drawflag int) {
			rc.DrawCharacter(int(posX), int(posY), char)
		},
		DrawString: func(posX, posY float32, text string, sizeX, sizeY float32, r, g, b, alpha float32, drawflag int, useColors bool) {
			step := int(sizeX)
			if step <= 0 {
				step = 8
			}
			x := int(posX)
			for _, ch := range text {
				rc.DrawCharacter(x, int(posY), int(ch))
				x += step
			}
		},
		DrawPic: func(posX, posY float32, name string, sizeX, sizeY float32, r, g, b, alpha float32, drawflag int) {
			if alpha <= 0 {
				return
			}
			pic := lookupCSQCPic(name)
			if pic == nil {
				return
			}
			x, y, drawPic, ok := prepareCSQCPic(pic, posX, posY, sizeX, sizeY, 0, 0, 1, 1, clip)
			if !ok {
				return
			}
			rc.DrawPic(x, y, drawPic)
		},
		DrawFill: func(posX, posY float32, sizeX, sizeY float32, red, green, blue, alpha float32, drawflag int) {
			if alpha <= 0 {
				return
			}
			x, y, width, height, _, _, _, _, ok := clipCSQCDrawRect(clip, posX, posY, sizeX, sizeY)
			if !ok {
				return
			}
			var palette []byte
			if g.Draw != nil {
				palette = g.Draw.Palette()
			}
			color := nearestPaletteIndex(red, green, blue, palette)
			rc.DrawFill(int(x), int(y), int(width), int(height), color)
		},
		DrawSubPic: func(posX, posY float32, sizeX, sizeY float32, name string, srcX, srcY, srcW, srcH float32, r, g, b, alpha float32, drawflag int) {
			if alpha <= 0 {
				return
			}
			pic := lookupCSQCPic(name)
			if pic == nil {
				return
			}
			x, y, drawPic, ok := prepareCSQCPic(pic, posX, posY, sizeX, sizeY, srcX, srcY, srcW, srcH, clip)
			if !ok {
				return
			}
			rc.DrawPic(x, y, drawPic)
		},
		SetClipArea: func(x, y, width, height float32) {
			clip = csqcClipRect{enabled: true, x: x, y: y, width: width, height: height}
		},
		ResetClipArea: func() {
			clip = csqcClipRect{}
		},
		StringWidth: func(text string, useColors bool, fontSizeX, fontSizeY float32) float32 {
			var count float32
			for range text {
				count++
			}
			return count * fontSizeX
		},
	}
}

// wireCSQCDrawHooks connects CSQC drawing builtins to a RenderContext.
func wireCSQCDrawHooks(rc renderer.RenderContext) {
	qc.SetCSQCDrawHooks(buildCSQCDrawHooks(rc))
}

func buildCSQCFrameState() qc.CSQCFrameState {
	var state qc.CSQCFrameState
	if g.Host != nil {
		state.FrameTime = float32(g.Host.FrameTime())
	}
	if g.Client != nil {
		state.Time = float32(g.Client.Time)
		state.MaxClients = float32(g.Client.MaxClients)
		state.Intermission = float32(g.Client.Intermission)
		state.ViewAngles = g.Client.ViewAngles
	}
	return state
}

func main() {
	// Logger initialization is handled in logger_*.go files based on build tags
	fmt.Printf("Ironwail-Go v%d.%d.%d\n", VersionMajor, VersionMinor, VersionPatch)
	fmt.Println("A Go port of Ironwail Quake engine")
	fmt.Println()

	startupOpts, err := parseStartupOptions(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	inet.SetHostPort(startupOpts.Port)

	// Check if a map argument was provided
	headlessFlag := flag.Bool("headless", false, "Run without rendering")
	screenshotFlag := flag.String("screenshot", "", "Save screenshot to PNG file and exit")
	widthFlag := flag.Int("width", startupVidWidth, "Initial window width")
	heightFlag := flag.Int("height", startupVidHeight, "Initial window height")
	logLevel := flag.String("loglvl", "INFO", "logging level (INFO, WARN, ERROR, DEBUG)")
	if err := flag.CommandLine.Parse(startupOpts.Args); err != nil {
		log.Fatal(err)
	}
	if *widthFlag > 0 {
		startupVidWidth = *widthFlag
	}
	if *heightFlag > 0 {
		startupVidHeight = *heightFlag
	}

	switch strings.ToUpper(*logLevel) {
	case "WARN":
		slog.SetLogLoggerLevel(slog.LevelWarn)
	case "ERROR":
		slog.SetLogLoggerLevel(slog.LevelError)
	case "DEBUG":
		slog.SetLogLoggerLevel(slog.LevelDebug)
	default:
		slog.SetLogLoggerLevel(slog.LevelInfo)
	}

	// Parse Quake-style +command arguments (e.g. +map start)
	args := flag.Args()
	mapArg := startupMapArg(args)
	if startupOpts.Dedicated && mapArg == "" {
		mapArg = "start"
	}

	// Try to initialize with renderer, fall back to headless if it fails
	dedicated := startupOpts.Dedicated
	headless := *headlessFlag || dedicated
	initErr := initSubsystems(headless, dedicated, startupOpts.MaxClients, startupOpts.BaseDir, startupOpts.GameDir, args)
	if initErr != nil && !headless {
		// Check if error is related to renderer initialization
		if isRendererError(initErr) {
			fmt.Println("WARNING: Renderer initialization failed. Running in headless mode.")
			fmt.Printf("Error: %v\n", initErr)
			fmt.Println("Continuing with game loop (no rendering)...")
			headless = true
			// Re-initialize without renderer
			if err := initSubsystems(true, false, startupOpts.MaxClients, startupOpts.BaseDir, startupOpts.GameDir, args); err != nil {
				log.Fatal("Initialization failed:", err)
			}
		} else {
			log.Fatal("Initialization failed:", initErr)
		}
	}
	defer shutdownEngine()

	// Deterministic startup logs after successful initialization
	slog.Info("FS mounted")
	slog.Info("QC loaded")
	if !dedicated {
		slog.Info("menu active")
	}

	// Execute map command if map argument was provided
	if mapArg != "" {
		slog.Info("map spawn started", "map", mapArg)
		if err := g.Host.CmdMap(mapArg, g.Subs); err != nil {
			log.Printf("Failed to spawn map %s: %v", mapArg, err)
		} else {
			slog.Info("map spawn finished", "map", mapArg)
			if g.Client != nil && g.Client.State == cl.StateActive && g.Host.SignOns() == 4 {
				applyStartupGameplayInputMode()
				slog.Info("client active", "map", mapArg)
			}
		}
	}

	screenshotPath := strings.TrimSpace(*screenshotFlag)
	screenshotMode := screenshotPath != ""

	if !headless {
		cb := gameCallbacks{}
		var screenshotErr error
		screenshotCaptured := false
		// Set up renderer callbacks
		g.Renderer.OnUpdate(func(dt float64) {
			pollRuntimeInputEvents()
			if g.Input != nil {
				syncGameplayInputMode()
				applyMenuMouseMove()
				applyGameplayMouseLook()
			}
			consoleVisible := g.Input != nil && g.Input.GetKeyDest() == input.KeyConsole
			conForcedup := g.Client == nil || g.Client.Signon < cl.Signons
			updateRuntimeConsoleSlide(dt, consoleVisible, conForcedup)

			transientEvents := runRuntimeFrame(dt, cb)
			if g.Host != nil && g.Host.IsAborted() {
				if g.Renderer != nil {
					g.Renderer.Stop()
				}
				return
			}

			// Update camera from client state each frame
			// This is the critical rendering path for M4: view setup
			if g.Renderer != nil {
				origin, angles := runtimeViewState()
				camera := runtimeCameraState(origin, angles)

				// Update renderer matrices (near=0.1, far=4096 for Quake world)
				g.Renderer.UpdateCamera(camera, 0.1, 4096.0)
			}

			syncRuntimeVisualEffects(dt, transientEvents)
		})
		g.Renderer.OnDraw(func(dc renderer.RenderContext) {
			if screenshotMode && !screenshotCaptured {
				defer func() {
					screenshotCaptured = true
					screenshotErr = captureScreenshot(screenshotPath, startupOpts.BaseDir, startupOpts.GameDir)
					if g.Renderer != nil {
						g.Renderer.Stop()
					}
				}()
			}

			if g.Renderer != nil && g.Server != nil && g.Server.WorldTree != nil &&
				shouldUploadRuntimeWorld(g.WorldUploadKey, g.Server.ModelName, g.Renderer.HasWorldData()) {
				if err := g.Renderer.UploadWorld(g.Server.WorldTree); err != nil {
					slog.Warn("deferred world upload failed", "error", err)
				} else {
					g.WorldUploadKey = g.Server.ModelName
				}
			}

			brushEntities := collectBrushEntities()
			aliasEntities := collectAliasEntities()
			spriteEntities := collectSpriteEntities()
			viewModel := collectViewModelEntity()

			if drawCtx, ok := dc.(*renderer.DrawContext); ok {
				state := buildRuntimeRenderFrameState(brushEntities, aliasEntities, spriteEntities, viewModel)
				drawCtx.RenderFrame(state, func(overlay renderer.RenderContext) {
					w, h := g.Renderer.Size()
					consoleVisible := g.Input != nil && g.Input.GetKeyDest() == input.KeyConsole

					// con_forcedup: when disconnected or not fully signed on,
					// force full console behind everything (mirrors C Ironwail
					// gl_screen.c:1511: con_forcedup = !cl.worldmodel || cls.signon != SIGNONS)
					conForcedup := g.Client == nil || g.Client.Signon < cl.Signons

					// Begin 2D overlay with default canvas.
					overlay.SetCanvas(renderer.CanvasDefault)

					if g.Host != nil && g.Host.LoadingPlaqueActive(0) {
						// Loading plaque uses menu-space coordinates.
						overlay.SetCanvas(renderer.CanvasMenu)
						drawLoadingPlaque(overlay, g.Draw)
						if consoleVisible {
							drawRuntimeConsole(overlay, w, h, true, false)
						}
						return
					}

					// When disconnected, draw full console as background (full screen)
					if conForcedup {
						drawRuntimeConsole(overlay, w, h, true, true)
					}

					// Menu draws on top of console
					if g.Menu != nil && g.Menu.IsActive() {
						drawRuntimeMenu(overlay, w, h, g.Menu.M_Draw)
						telemetryState := buildRuntimeTelemetryState(conForcedup)
						telemetryState.ViewRect = runtimeOverlayViewRect(w, h, false)
						drawRuntimeFPS(overlay, telemetryState, &g.FPSOverlay)
						drawRuntimeSavingIndicator(overlay, g.Draw, telemetryState)
						return
					}

					if !conForcedup {
						telemetryState := buildRuntimeTelemetryState(conForcedup)
						csqcActive := false
						if g.CSQC != nil && g.CSQC.IsLoaded() {
							csqcActive = true
							overlay.SetCanvas(renderer.CanvasCSQC)
							wireCSQCDrawHooks(overlay)

							frameState := buildCSQCFrameState()
							showScores := g.ShowScores && g.Client != nil && g.Client.MaxClients > 1
							canvas := overlay.Canvas()
							virtW := canvas.Right - canvas.Left
							virtH := canvas.Bottom - canvas.Top
							if err := g.CSQC.CallDrawHud(frameState, virtW, virtH, showScores); err != nil {
								slog.Error("CSQC_DrawHud failed", "error", err)
							}
							if showScores && g.CSQC.HasDrawScores() {
								if err := g.CSQC.CallDrawScores(frameState, virtW, virtH, showScores); err != nil {
									slog.Error("CSQC_DrawScores failed", "error", err)
								}
							}
						}
						telemetryState.ViewRect = runtimeOverlayViewRect(w, h, csqcActive)

						if !csqcActive && g.HUD != nil {
							overlay.SetCanvas(renderer.CanvasDefault) // TODO: CanvasSbar
							g.HUD.SetScreenSize(w, h)
							updateHUDFromServer()
							g.HUD.Draw(overlay)
						}
						drawRuntimeClock(overlay, telemetryState)
						drawRuntimeDemoControls(overlay, g.Draw, telemetryState, &g.DemoOverlay)
						drawRuntimeSpeed(overlay, telemetryState, &g.SpeedOverlay)
						drawRuntimeNet(overlay, g.Draw, telemetryState)
						drawRuntimeTurtle(overlay, g.Draw, telemetryState, &g.TurtleOverlayCount)
						if runtimePauseActive() {
							drawPauseOverlay(overlay, g.Draw)
						}

						if consoleVisible || runtimeConsoleAnimating() {
							drawRuntimeConsole(overlay, w, h, true, false)
							if consoleVisible {
								return
							}
						}

						if !runtimeConsoleAnimating() {
							drawRuntimeConsole(overlay, w, h, false, false)
						}

						if g.Input != nil && g.Input.GetKeyDest() == input.KeyMessage && !runtimeConsoleAnimating() {
							drawChatInput(overlay, w, h)
						}
					}
					telemetryState := buildRuntimeTelemetryState(conForcedup)
					telemetryState.ViewRect = runtimeOverlayViewRect(w, h, false)
					drawRuntimeFPS(overlay, telemetryState, &g.FPSOverlay)
					drawRuntimeSavingIndicator(overlay, g.Draw, telemetryState)
				})
				return
			}

			dc.Clear(0, 0, 0, 1)
			dc.SetCanvas(renderer.CanvasDefault)
			w, h := g.Renderer.Size()
			if g.Host != nil && g.Host.LoadingPlaqueActive(0) {
				drawLoadingPlaque(dc, g.Draw)
				return
			}
			// con_forcedup for gogpu path
			conForcedup := g.Client == nil || g.Client.Signon < cl.Signons
			if conForcedup {
				// In gogpu path we just show menu over black
			}
			if g.Menu != nil && g.Menu.IsActive() {
				drawRuntimeMenu(dc, w, h, g.Menu.M_Draw)
			} else if !conForcedup && runtimePauseActive() {
				drawPauseOverlay(dc, g.Draw)
			}
		})

		if screenshotMode {
			// Flush pending console commands (e.g. +setpos, +noclip from command line)
			// so camera positioning takes effect before the first rendered frame.
			cmdsys.Execute()
			if g.Host != nil && g.Server != nil {
				_ = g.Server.Frame(0.05) // run one physics frame to apply position changes
			}
		}

		// Start the main loop (blocking)
		slog.Info("frame loop started")
		runErr := g.Renderer.Run()
		if runErr != nil {
			releaseRuntimeRenderer()
			if isRendererError(runErr) {
				fmt.Println("WARNING: Render loop failed. Falling back to headless mode.")
				fmt.Printf("Error: %v\n", runErr)
				fmt.Println("Continuing with game loop (no rendering)...")
				headlessGameLoop()
			} else {
				log.Fatal("Render loop failed:", runErr)
			}
		}

		if screenshotMode {
			if screenshotErr != nil {
				log.Fatal("Screenshot failed:", screenshotErr)
			}
			return
		}
	}

	if screenshotMode {
		if err := captureScreenshot(screenshotPath, startupOpts.BaseDir, startupOpts.GameDir); err != nil {
			log.Fatal("Screenshot failed:", err)
		}
		return
	}

	if headless {
		// Run in headless mode (no rendering, just game loop)
		if dedicated {
			dedicatedGameLoop()
		} else {
			headlessGameLoop()
		}
	}

}

func runtimeViewModelVisible() bool {
	if g.Client == nil {
		return false
	}
	if g.Menu != nil && g.Menu.IsActive() {
		return false
	}
	if g.Client.Intermission != 0 {
		return false
	}
	if !cvar.BoolValue("r_drawentities") {
		return false
	}
	if !cvar.BoolValue("r_drawviewmodel") {
		return false
	}
	if cvar.BoolValue("chase_active") {
		return false
	}
	if cv := cvar.Get("scr_viewsize"); cv != nil && cv.Float >= 130 {
		return false
	}
	if g.Client.Health() <= 0 {
		return false
	}
	return g.Client.Items&cl.ItemInvisibility == 0
}

func drawMenuBackdrop(rc renderer.RenderContext, w, h int) {
	if rc == nil || w <= 0 || h <= 0 {
		return
	}
	rc.SetCanvas(renderer.CanvasDefault)
	alpha := float32(cvar.FloatValue("scr_menubgalpha"))
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	rc.DrawFillAlpha(0, 0, w, h, 0, alpha)
}

func runtimePauseActive() bool {
	if g.Host != nil {
		if demo := g.Host.DemoState(); demo != nil && demo.Playback && demo.Paused {
			return true
		}
		if g.Host.ServerPaused() {
			return true
		}
	}
	return g.Client != nil && g.Client.Paused
}

func buildRuntimeTelemetryState(conForcedup bool) runtimeTelemetryState {
	state := runtimeTelemetryState{
		ViewSize:        float32(cvar.FloatValue("scr_viewsize")),
		HUDStyle:        cvar.IntValue("hud_style"),
		ShowFPS:         float32(cvar.FloatValue("scr_showfps")),
		ShowClock:       cvar.IntValue("scr_clock"),
		ShowSpeed:       cvar.BoolValue("scr_showspeed"),
		ShowTurtle:      cvar.BoolValue("scr_showturtle"),
		ShowSpeedOfs:    float32(cvar.FloatValue("scr_showspeed_ofs")),
		DemoBarTimeout:  float32(cvar.FloatValue("scr_demobar_timeout")),
		ConsoleForced:   conForcedup,
		LastServerMsgAt: g.LastServerMessageAt,
	}
	if g.Host != nil {
		state.RealTime = g.Host.RealTime()
		state.FrameCount = g.Host.FrameCount()
		state.FrameTime = g.Host.FrameTime()
		state.SavingActive = g.Host.SavingIndicatorActive(state.RealTime)
		if demo := g.Host.DemoState(); demo != nil {
			state.DemoPlayback = demo.Playback
			state.DemoSpeed = demo.Speed
			state.DemoBaseSpeed = demo.BaseSpeed
			state.DemoProgress = demo.Progress()
			state.DemoName = runtimeDemoName(demo.Filename)
		}
	}
	if g.Client != nil {
		state.ClientTime = g.Client.Time
		state.Intermission = g.Client.Intermission
		state.InCutscene = g.Client.InCutscene()
		state.Velocity = g.Client.Velocity
		state.ClientActive = g.Client.State == cl.StateActive
	}
	return state
}

func runtimeOverlayViewRect(framebufferW, framebufferH int, csqcDrawHUD bool) renderer.ViewRect {
	vidW := cvar.IntValue("vid_width")
	if vidW <= 0 {
		vidW = framebufferW
	}
	vidH := cvar.IntValue("vid_height")
	if vidH <= 0 {
		vidH = framebufferH
	}
	guiW, guiH := runtimeGUIDimensions(framebufferW, framebufferH)
	conW, conH := runtimeConsoleDimensions(guiW, guiH)
	fov := float32(90)
	if cv := cvar.Get("fov"); cv != nil && cv.Float32() > 0 {
		fov = cv.Float32()
	}
	ref, err := renderer.CalcRefdef(renderer.ScreenMetrics{
		GLWidth:        framebufferW,
		GLHeight:       framebufferH,
		VidWidth:       vidW,
		VidHeight:      vidH,
		GUIWidth:       guiW,
		GUIHeight:      guiH,
		ConWidth:       conW,
		ConHeight:      conH,
		ViewSize:       float32(cvar.FloatValue("scr_viewsize")),
		FOV:            fov,
		FOVAdapt:       true,
		ZoomFOV:        30,
		Zoom:           g.Zoom,
		SbarScale:      float32(cvar.FloatValue("scr_sbarscale")),
		SbarAlpha:      currentSbarAlpha(),
		MenuScale:      float32(cvar.FloatValue("scr_menuscale")),
		CrosshairScale: float32(cvar.FloatValue("scr_crosshairscale")),
		Intermission:   g.Client != nil && g.Client.Intermission != 0,
		HudStyle:       cvar.IntValue("hud_style"),
		CSQCDrawHud:    csqcDrawHUD,
	})
	if err != nil {
		return renderer.ViewRect{X: 0, Y: 0, Width: framebufferW, Height: framebufferH}
	}
	return ref.VRect
}

func currentSbarAlpha() float32 {
	alpha := float32(cvar.FloatValue("scr_sbaralpha"))
	if alpha <= 0 {
		return 0
	}
	if alpha > 1 {
		return 1
	}
	return alpha
}

func drawRuntimeString(rc renderer.RenderContext, x, y int, text string) {
	for _, ch := range text {
		rc.DrawCharacter(x, y, int(ch))
		x += 8
	}
}

func drawRuntimeClock(rc renderer.RenderContext, state runtimeTelemetryState) {
	if rc == nil || state.ShowClock != 1 || state.ViewSize >= 130 {
		return
	}
	minutes := int(state.ClientTime) / 60
	seconds := int(state.ClientTime) % 60
	text := fmt.Sprintf("%d:%02d", minutes, seconds)
	if state.HUDStyle == renderer.HUDClassic {
		rc.SetCanvas(renderer.CanvasBottomRight)
		drawRuntimeString(rc, 320-len(text)*8, 200-8, text)
		return
	}
	rc.SetCanvas(renderer.CanvasTopRight)
	drawRuntimeString(rc, 320-16-len(text)*8, 8, text)
}

func drawRuntimeFPS(rc renderer.RenderContext, state runtimeTelemetryState, overlay *runtimeFPSOverlay) {
	if rc == nil || overlay == nil {
		return
	}
	if state.ConsoleForced {
		overlay.oldTime = state.RealTime
		overlay.oldFrameCount = state.FrameCount
		overlay.lastFPS = 0
		return
	}
	elapsed := state.RealTime - overlay.oldTime
	frames := state.FrameCount - overlay.oldFrameCount
	if elapsed < 0 || frames < 0 {
		overlay.oldTime = state.RealTime
		overlay.oldFrameCount = state.FrameCount
		return
	}
	if elapsed > 0.75 {
		overlay.lastFPS = float64(frames) / elapsed
		overlay.oldTime = state.RealTime
		overlay.oldFrameCount = state.FrameCount
	}
	if state.ShowFPS == 0 || state.ViewSize >= 130 || overlay.lastFPS == 0 {
		return
	}
	text := fmt.Sprintf("%4.0f fps", overlay.lastFPS)
	if state.ShowFPS < 0 {
		text = fmt.Sprintf("%.2f ms", 1000.0/overlay.lastFPS)
	}
	if state.HUDStyle == renderer.HUDClassic {
		y := 200 - 8
		if state.ShowClock == 1 {
			y -= 8
		}
		rc.SetCanvas(renderer.CanvasBottomRight)
		drawRuntimeString(rc, 320-len(text)*8, y, text)
		return
	}
	y := 8
	if state.ShowClock == 1 {
		y += 8
	}
	rc.SetCanvas(renderer.CanvasTopRight)
	drawRuntimeString(rc, 320-16-len(text)*8, y, text)
}

func drawRuntimeSpeed(rc renderer.RenderContext, state runtimeTelemetryState, overlay *runtimeSpeedOverlay) {
	if rc == nil || overlay == nil {
		return
	}
	if overlay.lastRealTime == 0 && overlay.displaySpeed == 0 && overlay.maxSpeed == 0 {
		overlay.displaySpeed = -1
	}
	if overlay.lastRealTime > state.RealTime {
		overlay.lastRealTime = 0
		overlay.displaySpeed = -1
		overlay.maxSpeed = 0
	}
	speed := float32(math.Sqrt(float64(state.Velocity[0]*state.Velocity[0] + state.Velocity[1]*state.Velocity[1])))
	if speed > overlay.maxSpeed {
		overlay.maxSpeed = speed
	}
	if state.ShowSpeed && overlay.displaySpeed >= 0 && state.Intermission == 0 && !state.InCutscene && state.ViewSize < 130 {
		text := fmt.Sprintf("%d", int(overlay.displaySpeed))
		rc.SetCanvas(renderer.CanvasCrosshair)
		canvas := rc.Canvas()
		top := canvas.Top
		bottom := canvas.Bottom
		if top == 0 && bottom == 0 {
			top = -100
			bottom = 100
		}
		y := min(max(top, 4+state.ShowSpeedOfs), bottom-8)
		drawRuntimeString(rc, -(len(text) * 4), int(y), text)
	}
	if state.RealTime-overlay.lastRealTime >= 0.05 {
		overlay.lastRealTime = state.RealTime
		overlay.displaySpeed = overlay.maxSpeed
		overlay.maxSpeed = 0
	}
}

type picAlphaRenderContext interface {
	DrawPicAlpha(x, y int, pic *qimage.QPic, alpha float32)
}

func drawRuntimePicAlpha(rc renderer.RenderContext, x, y int, pic *qimage.QPic, alpha float32) {
	if rc == nil || pic == nil || alpha <= 0 {
		return
	}
	if picAlpha, ok := rc.(picAlphaRenderContext); ok {
		picAlpha.DrawPicAlpha(x, y, pic, alpha)
		return
	}
	rc.DrawPic(x, y, pic)
}

func drawRuntimeTextBoxAlpha(rc renderer.RenderContext, pics picProvider, x, y, width, lines int, alpha float32) {
	if rc == nil || pics == nil || alpha <= 0 {
		return
	}
	cx := x
	cy := y

	if pic := pics.GetPic("gfx/box_tl.lmp"); pic != nil {
		drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
	}
	if pic := pics.GetPic("gfx/box_ml.lmp"); pic != nil {
		for n := 0; n < lines; n++ {
			cy += 8
			drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
		}
	}
	if pic := pics.GetPic("gfx/box_bl.lmp"); pic != nil {
		drawRuntimePicAlpha(rc, cx, cy+8, pic, alpha)
	}

	cx += 8
	for remaining := width; remaining > 0; remaining -= 2 {
		cy = y
		if pic := pics.GetPic("gfx/box_tm.lmp"); pic != nil {
			drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
		}
		for n := 0; n < lines; n++ {
			cy += 8
			name := "gfx/box_mm.lmp"
			if n == 1 {
				name = "gfx/box_mm2.lmp"
			}
			if pic := pics.GetPic(name); pic != nil {
				drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
			}
		}
		if pic := pics.GetPic("gfx/box_bm.lmp"); pic != nil {
			drawRuntimePicAlpha(rc, cx, cy+8, pic, alpha)
		}
		cx += 16
	}

	cy = y
	if pic := pics.GetPic("gfx/box_tr.lmp"); pic != nil {
		drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
	}
	if pic := pics.GetPic("gfx/box_mr.lmp"); pic != nil {
		for n := 0; n < lines; n++ {
			cy += 8
			drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
		}
	}
	if pic := pics.GetPic("gfx/box_br.lmp"); pic != nil {
		drawRuntimePicAlpha(rc, cx, cy+8, pic, alpha)
	}
}

func runtimeDemoName(name string) string {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if len(base) > 30 {
		base = base[:30]
	}
	return base
}

func formatRuntimeDemoBaseSpeed(speed float32) string {
	if speed == 0 {
		return ""
	}
	absSpeed := math.Abs(float64(speed))
	if absSpeed >= 1 {
		return fmt.Sprintf("%gx", absSpeed)
	}
	return fmt.Sprintf("1/%gx", 1/absSpeed)
}

func drawRuntimeDemoControls(rc renderer.RenderContext, pics picProvider, state runtimeTelemetryState, overlay *runtimeDemoOverlay) {
	if rc == nil || overlay == nil || !state.DemoPlayback || state.DemoBarTimeout < 0 {
		if overlay != nil {
			overlay.showTime = 0
		}
		return
	}
	if state.DemoSpeed != overlay.prevSpeed ||
		state.DemoBaseSpeed != overlay.prevBaseSpeed ||
		math.Abs(float64(state.DemoSpeed)) > math.Abs(float64(state.DemoBaseSpeed)) ||
		state.DemoBarTimeout == 0 {
		overlay.prevSpeed = state.DemoSpeed
		overlay.prevBaseSpeed = state.DemoBaseSpeed
		overlay.showTime = 1
		if state.DemoBarTimeout > 0 {
			overlay.showTime = float64(state.DemoBarTimeout)
		}
	} else {
		overlay.showTime -= state.FrameTime
		if overlay.showTime < 0 {
			overlay.showTime = 0
			return
		}
	}

	const timebarChars = 38
	x := 160 - timebarChars/2*8
	y := -20
	rc.SetCanvas(renderer.CanvasSbar)
	if state.Intermission != 0 {
		rc.SetCanvas(renderer.CanvasMenu)
		y = 25
	}

	alpha := currentSbarAlpha()
	drawRuntimeTextBoxAlpha(rc, pics, x-8, y-8, timebarChars, 1, alpha)

	status := ">"
	if state.DemoSpeed == 0 {
		status = "II"
	} else if math.Abs(float64(state.DemoSpeed)) > 1 {
		status = ">>"
	}
	if state.DemoSpeed < 0 {
		status = strings.Repeat("<", len(status))
	}
	drawRuntimeString(rc, x, y, status)

	if base := formatRuntimeDemoBaseSpeed(state.DemoBaseSpeed); base != "" {
		drawRuntimeString(rc, x+(timebarChars-len(base))*8, y, base)
	}
	if state.DemoName != "" {
		drawRuntimeString(rc, 160-len(state.DemoName)*4, y, state.DemoName)
	}

	barY := y - 8
	rc.DrawCharacter(x-8, barY, 128)
	for i := 0; i < timebarChars; i++ {
		rc.DrawCharacter(x+i*8, barY, 129)
	}
	rc.DrawCharacter(x+timebarChars*8, barY, 130)

	progress := state.DemoProgress
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	cursorX := x + int(float64((timebarChars-1)*8)*progress)
	rc.DrawCharacter(cursorX, barY, 131)

	seconds := int(state.ClientTime)
	timeText := fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
	timeX := cursorX
	if colon := strings.IndexByte(timeText, ':'); colon >= 0 {
		timeX -= colon * 8
	}
	timeY := barY - 11
	drawRuntimeTextBoxAlpha(rc, pics, timeX-8-(len(timeText)&1)*4, timeY-8, len(timeText)+(len(timeText)&1), 1, alpha)
	drawRuntimeString(rc, timeX, timeY, timeText)
}

func drawRuntimeTurtle(rc renderer.RenderContext, pics picProvider, state runtimeTelemetryState, count *int) {
	if rc == nil || pics == nil || count == nil || !state.ShowTurtle {
		return
	}
	if state.FrameTime < 0.1 {
		*count = 0
		return
	}
	*count++
	if *count < 3 {
		return
	}
	if turtle := pics.GetPic("turtle"); turtle != nil {
		rc.SetCanvas(renderer.CanvasDefault)
		rc.DrawPic(state.ViewRect.X, state.ViewRect.Y, turtle)
	}
}

func drawRuntimeNet(rc renderer.RenderContext, pics picProvider, state runtimeTelemetryState) {
	if rc == nil || pics == nil || !state.ClientActive || state.DemoPlayback {
		return
	}
	if state.RealTime-state.LastServerMsgAt < 0.3 {
		return
	}
	if netPic := pics.GetPic("net"); netPic != nil {
		rc.SetCanvas(renderer.CanvasDefault)
		rc.DrawPic(state.ViewRect.X+64, state.ViewRect.Y, netPic)
	}
}

func drawRuntimeSavingIndicator(rc renderer.RenderContext, pics picProvider, state runtimeTelemetryState) {
	if rc == nil || pics == nil || !state.SavingActive {
		return
	}
	disc := pics.GetPic("disc")
	if disc == nil {
		return
	}
	y := 8
	if state.HUDStyle != renderer.HUDClassic && state.ViewSize < 130 {
		if state.ShowClock == 1 {
			y += 8
		}
		if state.ShowFPS != 0 {
			y += 8
		}
		if y != 8 {
			y += 8
		}
	}
	rc.SetCanvas(renderer.CanvasTopRight)
	rc.DrawPic(320-16-int(disc.Width), y, disc)
}

func drawPauseOverlay(dc renderer.RenderContext, pics picProvider) {
	if dc == nil || pics == nil {
		return
	}
	if cv := cvar.Get("showpause"); cv != nil && !cv.Bool() {
		return
	}
	dc.SetCanvas(renderer.CanvasMenu)
	if pause := pics.GetPic("gfx/pause.lmp"); pause != nil {
		dc.DrawMenuPic((320-int(pause.Width))/2, (240-48-int(pause.Height))/2, pause)
	}
}

func drawRuntimeMenu(rc renderer.RenderContext, w, h int, drawMenu func(renderer.RenderContext)) {
	if rc == nil || drawMenu == nil {
		return
	}
	drawMenuBackdrop(rc, w, h)
	rc.SetCanvas(renderer.CanvasMenu)
	drawMenu(rc)
}

func drawChatInput(rc renderer.RenderContext, w, h int) {
	prompt := "say: "
	if chatTeam {
		prompt = "say_team: "
	}
	fullText := clippedChatInput(prompt, chatBuffer, max(1, w/8-2))

	y := console.NotifyLineCount() * 8
	x := 8

	// Green text
	// Manual string drawing:
	charSize := 8
	currentX := x
	for _, char := range fullText {
		rc.DrawCharacter(currentX, y, int(char))
		currentX += charSize
	}
	rc.DrawCharacter(currentX, y, runtimeCursorGlyph(runtimeNow()))
}

func clippedChatInput(prompt, message string, maxChars int) string {
	if maxChars <= 1 {
		return prompt[:min(len(prompt), 1)]
	}
	visiblePrompt := prompt
	if len(visiblePrompt) > maxChars-1 {
		visiblePrompt = visiblePrompt[:maxChars-1]
	}
	remaining := maxChars - len(visiblePrompt) - 1
	if remaining <= 0 {
		return visiblePrompt
	}
	if len(message) > remaining {
		message = message[len(message)-remaining:]
	}
	return visiblePrompt + message
}

func runtimeCursorGlyph(now time.Time) int {
	frame := (now.UnixNano() / int64(time.Second/4)) & 1
	return 10 + int(frame)
}
