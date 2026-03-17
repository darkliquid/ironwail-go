package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
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
	ShowScores       bool
	ModDir           string

	CameraInLiquid     bool
	CameraLeafContents int32
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
			if g.Renderer != nil && g.Server != nil && g.Server.WorldTree != nil && !g.Renderer.HasWorldData() {
				if err := g.Renderer.UploadWorld(g.Server.WorldTree); err != nil {
					slog.Warn("deferred world upload failed", "error", err)
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

					if g.Host != nil && g.Host.LoadingPlaqueActive(0) {
						drawLoadingPlaque(overlay, g.Draw)
						if consoleVisible {
							console.Draw(overlay, w, h, true)
						}
						return
					}

					// When disconnected, draw full console as background
					if conForcedup {
						console.Draw(overlay, w, h, true)
					}

					// Menu draws on top of console
					if g.Menu != nil && g.Menu.IsActive() {
						g.Menu.M_Draw(overlay)
						return
					}

					if !conForcedup {
						if g.HUD != nil {
							g.HUD.SetScreenSize(w, h)
							updateHUDFromServer()
							g.HUD.Draw(overlay)
						}

						if consoleVisible {
							console.Draw(overlay, w, h, true)
							return
						}

						console.Draw(overlay, w, h, false)
					}
				})
				return
			}

			dc.Clear(0, 0, 0, 1)
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
	if !cvar.BoolValue("r_drawviewmodel") {
		return false
	}
	if g.Client.Health() <= 0 {
		return false
	}
	return g.Client.Items&cl.ItemInvisibility == 0
}
