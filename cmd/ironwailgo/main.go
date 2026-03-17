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

var (
	gameHost       *host.Host
	gameServer     *server.Server
	gameQC         *qc.VM
	gameRenderer   *renderer.Renderer
	gameSubs       *host.Subsystems // Store subsystems for command execution
	gameClient     *cl.Client
	gameParticles  *renderer.ParticleSystem
	gameDecalMarks *renderer.DecalMarkSystem
	particleRNG    *rand.Rand
	particleTime   float32

	// Menu subsystem
	gameMenu  *menu.Manager
	gameInput *input.System
	gameDraw  *draw.Manager
	gameHUD   *hud.HUD
	gameAudio *audio.AudioAdapter

	gameMouseGrabbed bool
	aliasModelCache  map[string]*model.Model
	spriteModelCache map[string]*runtimeSpriteModel
	soundSFXByIndex  map[int]*audio.SFX
	menuSFXByName    map[string]*audio.SFX
	ambientSFX       [audio.NumAmbients]*audio.SFX
	soundPrecacheKey string
	staticSoundKey   string
	musicTrackKey    string
	skyboxNameKey    string
	gameShowScores   bool
	gameModDir       string

	// runtimeCameraInLiquid tracks whether the current camera/view leaf is a
	// liquid leaf (water, slime, or lava). Updated each frame in the OnUpdate
	// callback alongside ambient audio; used to drive the visual waterwarp effect.
	runtimeCameraInLiquid bool

	// runtimeCameraLeafContents is the BSP leaf contents type at the current
	// camera position (e.g. bsp.ContentsWater, ContentsLava, ContentsEmpty).
	// Updated alongside runtimeCameraInLiquid in syncRuntimeAmbientAudio.
	// Used to drive the contents color-shift (v_blend underwater tint).
	runtimeCameraLeafContents int32
)

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
		if gameHost == nil {
			return
		}
		if err := gameHost.WriteConfig(gameSubs); err != nil {
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
		if err := gameHost.CmdMap(mapArg, gameSubs); err != nil {
			log.Printf("Failed to spawn map %s: %v", mapArg, err)
		} else {
			slog.Info("map spawn finished", "map", mapArg)
			if gameClient != nil && gameClient.State == cl.StateActive && gameHost.SignOns() == 4 {
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
		gameRenderer.OnUpdate(func(dt float64) {
			if gameInput != nil {
				_ = gameInput.PollEvents()
				syncGameplayInputMode()
				applyMenuMouseMove()
				applyGameplayMouseLook()
			}

			transientEvents := runRuntimeFrame(dt, cb)
			if gameHost != nil && gameHost.IsAborted() {
				if gameRenderer != nil {
					gameRenderer.Stop()
				}
				return
			}

			// Update camera from client state each frame
			// This is the critical rendering path for M4: view setup
			if gameRenderer != nil {
				origin, angles := runtimeViewState()
				camera := runtimeCameraState(origin, angles)

				// Update renderer matrices (near=0.1, far=4096 for Quake world)
				gameRenderer.UpdateCamera(camera, 0.1, 4096.0)
			}

			syncRuntimeVisualEffects(dt, transientEvents)
		})
		gameRenderer.OnDraw(func(dc renderer.RenderContext) {
			if gameRenderer != nil && gameServer != nil && gameServer.WorldTree != nil && !gameRenderer.HasWorldData() {
				if err := gameRenderer.UploadWorld(gameServer.WorldTree); err != nil {
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
					w, h := gameRenderer.Size()
					consoleVisible := gameInput != nil && gameInput.GetKeyDest() == input.KeyConsole

					// con_forcedup: when disconnected or not fully signed on,
					// force full console behind everything (mirrors C Ironwail
					// gl_screen.c:1511: con_forcedup = !cl.worldmodel || cls.signon != SIGNONS)
					conForcedup := gameClient == nil || gameClient.Signon < cl.Signons

					if gameHost != nil && gameHost.LoadingPlaqueActive(0) {
						drawLoadingPlaque(overlay, gameDraw)
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
					if gameMenu != nil && gameMenu.IsActive() {
						gameMenu.M_Draw(overlay)
						return
					}

					if !conForcedup {
						if gameHUD != nil {
							gameHUD.SetScreenSize(w, h)
							updateHUDFromServer()
							gameHUD.Draw(overlay)
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
			if gameHost != nil && gameHost.LoadingPlaqueActive(0) {
				drawLoadingPlaque(dc, gameDraw)
				return
			}
			// con_forcedup for gogpu path
			conForcedup := gameClient == nil || gameClient.Signon < cl.Signons
			if conForcedup {
				// In gogpu path we just show menu over black
			}
			if gameMenu != nil && gameMenu.IsActive() {
				gameMenu.M_Draw(dc)
			}
		})

		// Start the main loop (blocking)
		slog.Info("frame loop started")
		runErr := gameRenderer.Run()
		if runErr != nil {
			gameRenderer.Shutdown()
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
			gameRenderer.Shutdown()
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
	if gameClient == nil {
		return false
	}
	if gameMenu != nil && gameMenu.IsActive() {
		return false
	}
	if gameClient.Intermission != 0 {
		return false
	}
	if !cvar.BoolValue("r_drawviewmodel") {
		return false
	}
	if gameClient.Health() <= 0 {
		return false
	}
	return gameClient.Items&cl.ItemInvisibility == 0
}
