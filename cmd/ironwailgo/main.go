package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"math"
	"math/rand"
	"strings"

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
}

var g Game

type defaultBinding struct {
	key     int
	command string
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

func (globalCommandBuffer) Init()               {}
func (globalCommandBuffer) Execute()            { cmdsys.Execute() }
func (globalCommandBuffer) AddText(text string) { cmdsys.AddText(text) }
func (globalCommandBuffer) InsertText(text string) {
	cmdsys.InsertText(text)
}
func (globalCommandBuffer) Shutdown() {}

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

	// Check if a map argument was provided
	baseDir := flag.String("basedir", ".", "Base Quake directory containing id1")
	gameDir := flag.String("game", "id1", "Game directory (e.g. id1, hipnotic)")
	headlessFlag := flag.Bool("headless", false, "Run without rendering")
	dedicatedFlag := flag.Bool("dedicated", false, "Run as dedicated server")
	screenshotFlag := flag.String("screenshot", "", "Save screenshot to PNG file and exit")
	logLevel := flag.String("loglvl", "INFO", "logging level (INFO, WARN, ERROR, DEBUG)")
	flag.Parse()

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

	// Try to initialize with renderer, fall back to headless if it fails
	dedicated := *dedicatedFlag
	headless := *headlessFlag || dedicated
	initErr := initSubsystems(headless, dedicated, *baseDir, *gameDir, args)
	if initErr != nil && !headless {
		// Check if error is related to renderer initialization
		if isRendererError(initErr) {
			fmt.Println("WARNING: Renderer initialization failed. Running in headless mode.")
			fmt.Printf("Error: %v\n", initErr)
			fmt.Println("Continuing with game loop (no rendering)...")
			headless = true
			// Re-initialize without renderer
			if err := initSubsystems(true, false, *baseDir, *gameDir, args); err != nil {
				log.Fatal("Initialization failed:", err)
			}
		} else {
			log.Fatal("Initialization failed:", initErr)
		}
	}
	cvar.SetBool("dedicated", dedicated)
	defer func() {
		if g.Host == nil {
			return
		}
		if err := g.Host.WriteConfig(g.Subs); err != nil {
			log.Printf("Failed to write config: %v", err)
		}
	}()

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

	// Screenshot mode: render single frame and save to PNG
	if *screenshotFlag != "" {
		// Flush pending console commands (e.g. +setpos, +noclip from command line)
		// so camera positioning takes effect before the screenshot.
		cmdsys.Execute()
		if g.Host != nil && g.Server != nil {
			_ = g.Server.Frame(0.05) // run one physics frame to apply position changes
		}
		if err := captureScreenshot(*screenshotFlag, *baseDir, *gameDir); err != nil {
			log.Fatal("Screenshot failed:", err)
		}
		return
	}

	if !headless {
		cb := gameCallbacks{}
		// Set up renderer callbacks
		g.Renderer.OnUpdate(func(dt float64) {
			if g.Input != nil {
				_ = g.Input.PollEvents()
				syncGameplayInputMode()
				applyMenuMouseMove()
				applyGameplayMouseLook()
			}

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
						overlay.SetCanvas(renderer.CanvasDefault) // TODO: CanvasMenu when DrawMenuPic is canvas-aware
						drawLoadingPlaque(overlay, g.Draw)
						if consoleVisible {
							overlay.SetCanvas(renderer.CanvasDefault) // TODO: CanvasConsole
							console.Draw(overlay, w, h, true)
						}
						return
					}

					// When disconnected, draw full console as background (full screen)
					if conForcedup {
						overlay.SetCanvas(renderer.CanvasDefault) // TODO: CanvasConsole
						console.Draw(overlay, w, h, true, true)
					}

					// Menu draws on top of console
					if g.Menu != nil && g.Menu.IsActive() {
						overlay.SetCanvas(renderer.CanvasDefault) // TODO: CanvasMenu
						g.Menu.M_Draw(overlay)
						return
					}

					if !conForcedup {
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

						if !csqcActive && g.HUD != nil {
							overlay.SetCanvas(renderer.CanvasDefault) // TODO: CanvasSbar
							g.HUD.SetScreenSize(w, h)
							updateHUDFromServer()
							g.HUD.Draw(overlay)
						}

						if consoleVisible {
							overlay.SetCanvas(renderer.CanvasDefault) // TODO: CanvasConsole
							console.Draw(overlay, w, h, true)
							return
						}

						overlay.SetCanvas(renderer.CanvasDefault) // TODO: CanvasConsole for notify lines
						console.Draw(overlay, w, h, false)

						if g.Input != nil && g.Input.GetKeyDest() == input.KeyMessage {
							drawChatInput(overlay, w, h)
						}
					}
				})
				return
			}

			dc.Clear(0, 0, 0, 1)
			dc.SetCanvas(renderer.CanvasDefault)
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
				dc.SetCanvas(renderer.CanvasDefault) // TODO: CanvasMenu
				g.Menu.M_Draw(dc)
			}
		})

		// Start the main loop (blocking)
		slog.Info("frame loop started")
		runErr := g.Renderer.Run()
		if runErr != nil {
			g.Renderer.Shutdown()
			if isRendererError(runErr) {
				fmt.Println("WARNING: Render loop failed. Falling back to headless mode.")
				fmt.Printf("Error: %v\n", runErr)
				fmt.Println("Continuing with game loop (no rendering)...")
				headlessGameLoop()
			} else {
				log.Fatal("Render loop failed:", runErr)
			}
		} else {
			// Cleanup
			g.Renderer.Shutdown()
		}
	}

	if headless {
		// Run in headless mode (no rendering, just game loop)
		if dedicated {
			dedicatedGameLoop()
		} else {
			headlessGameLoop()
		}
	}

	if g.CSQC != nil && g.CSQC.IsLoaded() {
		if err := g.CSQC.CallShutdown(); err != nil {
			slog.Error("CSQC_Shutdown failed", "error", err)
		}
		g.CSQC.Unload()
	}

	slog.Info("Engine shutdown complete")
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

func drawChatInput(rc renderer.RenderContext, w, h int) {
	prompt := "say: "
	if chatTeam {
		prompt = "say_team: "
	}
	fullText := prompt + chatBuffer + "_"

	// Draw at roughly 1/3 down the screen or near top, standard Quake is just below notify lines?
	// Actually Quake draws it at specific Y.
	// Let's draw it at Y=60 or similar, or center?
	// Quake draws it around y=32 or so?
	y := 64
	x := 8

	// Green text
	// Manual string drawing:
	charSize := 8
	currentX := x
	for _, char := range fullText {
		rc.DrawCharacter(currentX, y, int(char))
		currentX += charSize
	}
}
