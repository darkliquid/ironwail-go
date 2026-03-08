package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/png"
	"log"
	"log/slog"
	"math"
	"math/rand"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/ironwail/ironwail-go/internal/audio"
	cl "github.com/ironwail/ironwail-go/internal/client"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/fs"
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
	qtypes "github.com/ironwail/ironwail-go/pkg/types"
)

const (
	VersionMajor = 0
	VersionMinor = 2
	VersionPatch = 0
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
	soundPrecacheKey string
	staticSoundKey   string
	musicTrackKey    string
	skyboxNameKey    string
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

func initGameHost() error {
	// Initialize console and command system
	console.InitGlobal(0)

	// Initialize cvars for video, sound, gameplay
	cvar.Register("vid_width", "1280", cvar.FlagArchive, "Video width")
	cvar.Register("vid_height", "720", cvar.FlagArchive, "Video height")
	cvar.Register("vid_fullscreen", "0", cvar.FlagArchive, "Fullscreen mode (0=windowed, 1=fullscreen)")
	cvar.Register("vid_vsync", "1", cvar.FlagArchive, "Vertical sync")
	cvar.Register("host_maxfps", "250", cvar.FlagArchive, "Maximum frames per second")
	sVolume := cvar.Register("s_volume", "0.7", cvar.FlagArchive, "Sound volume")
	sVolume.Callback = func(*cvar.CVar) {
		applySVolume()
	}
	cvar.Register("r_gamma", "1.0", cvar.FlagArchive, "Gamma correction")
	cvar.Register("r_drawviewmodel", "1", cvar.FlagArchive, "Draw first-person viewmodel")
	cvar.Register(renderer.CvarRSkyFog, "0.5", cvar.FlagArchive, "Sky fog mix factor (0..1)")
	cvar.Register("developer", "0", 0, "Developer mode")
	registerControlCvars()

	// Create host instance
	gameHost = host.NewHost()

	return nil
}

func registerControlCvars() {
	alwaysRun := cvar.Register("cl_alwaysrun", "1", cvar.FlagArchive, "Always run movement by default")
	freelook := cvar.Register("freelook", "1", cvar.FlagArchive, "Enable mouse freelook")
	lookspring := cvar.Register("lookspring", "0", cvar.FlagArchive, "Center view when look key released")
	cvar.Register("lookstrafe", "0", cvar.FlagArchive, "Use mouse X for strafing when +strafe held")
	cvar.Register("sensitivity", "6.8", cvar.FlagArchive, "Mouse sensitivity scale")
	cvar.Register("m_pitch", "0.0176", cvar.FlagArchive, "Mouse pitch scale")
	cvar.Register("m_yaw", "0.022", cvar.FlagArchive, "Mouse yaw scale")
	cvar.Register("m_forward", "1", cvar.FlagArchive, "Mouse forward scale")
	cvar.Register("m_side", "0.8", cvar.FlagArchive, "Mouse side scale")
	for _, cv := range []*cvar.CVar{alwaysRun, freelook, lookspring} {
		cv.Callback = func(*cvar.CVar) {
			syncControlCvarsToClient()
		}
	}
}

func syncControlCvarsToClient() {
	if gameClient == nil {
		return
	}
	gameClient.AlwaysRun = cvar.BoolValue("cl_alwaysrun")
	gameClient.FreeLook = cvar.BoolValue("freelook")
	gameClient.LookSpring = cvar.BoolValue("lookspring")
}

func initGameServer() error {
	// Create server instance
	gameServer = server.NewServer()

	return nil
}

func initGameQC() error {
	// Create QC VM instance
	gameQC = qc.NewVM()
	// slog.Info("QC loaded") - moved to main for deterministic logs

	return nil
}

func initGameRenderer() error {
	preferWaylandForGoGPU()

	// Create renderer instance from cvars
	cfg := renderer.ConfigFromCvars()

	tr, err := renderer.NewWithConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}
	gameRenderer = tr

	return nil
}

func preferWaylandForGoGPU() {
	if runtime.GOOS != "linux" {
		return
	}

	if strings.EqualFold(os.Getenv("IW_INPUT_BACKEND"), "sdl3") {
		return
	}

	if os.Getenv("WAYLAND_DISPLAY") != "" {
		return
	}

	if os.Getenv("DISPLAY") != "" {
		slog.Warn("Using X11 backend; gogpu X11 keyboard events are currently not implemented")
	}
}

func initSubsystems(headless bool, basedir, gamedir string, args []string) error {
	// Initialize input system
	gameInput = input.NewSystem(nil) // No backend yet - will be set by renderer
	if err := gameInput.Init(); err != nil {
		return fmt.Errorf("failed to init input system: %w", err)
	}

	// Initialize draw manager
	gameDraw = draw.NewManager()

	// Initialize menu system
	gameMenu = menu.NewManager(gameDraw, gameInput)
	gameMenu.SetSoundPlayer(playMenuSound)

	// Set up menu input callbacks
	gameInput.OnMenuKey = handleMenuKeyEvent
	gameInput.OnMenuChar = handleMenuCharEvent
	gameInput.OnKey = handleGameKeyEvent
	gameInput.OnChar = handleGameCharEvent

	if err := initGameHost(); err != nil {
		return err
	}
	// Initialize filesystem
	fileSys := fs.NewFileSystem()
	if err := fileSys.Init(basedir, gamedir); err != nil {
		return fmt.Errorf("failed to init filesystem: %w", err)
	}
	// slog.Info("FS mounted") - moved to main for deterministic logs

	// Initialize QuakeC VM
	if err := initGameQC(); err != nil {
		return err
	}

	// Load progs.dat into QC VM
	progsData, err := fileSys.LoadFile("progs.dat")
	if err != nil {
		return fmt.Errorf("failed to load progs.dat: %w", err)
	}
	if err := gameQC.LoadProgs(bytes.NewReader(progsData)); err != nil {
		return fmt.Errorf("failed to parse progs.dat: %w", err)
	}
	// Link the builtins and set up entity sizes
	qc.RegisterBuiltins(gameQC)

	if err := initGameServer(); err != nil {
		return err
	}
	// Link QC VM into the server
	gameServer.QCVM = gameQC

	if !headless {
		if err := initGameRenderer(); err != nil {
			return err
		}
	}

	// If renderer was created, wire its input backend into the input system
	if gameRenderer != nil && gameInput != nil {
		// Some renderers provide a backend factory to adapt window events
		// to the engine input system.
		if bb := gameRenderer.InputBackendForSystem(gameInput); bb != nil {
			if err := gameInput.SetBackend(bb); err != nil {
				return fmt.Errorf("failed to set renderer input backend: %w", err)
			}
		}
	}

	// Optional override to force SDL3 input even when renderer backend exists.
	// Useful when platform-specific window backends do not emit keyboard events.
	if gameInput != nil && strings.EqualFold(os.Getenv("IW_INPUT_BACKEND"), "sdl3") {
		previousBackend := gameInput.Backend()
		if b := input.NewSDL3Backend(gameInput); b != nil {
			if err := gameInput.SetBackend(b); err != nil {
				slog.Warn("failed to force SDL3 input backend; keeping previous backend", "error", err)
				if previousBackend != nil {
					if restoreErr := gameInput.SetBackend(previousBackend); restoreErr != nil {
						return fmt.Errorf("failed to restore previous input backend after SDL3 override failure: %w", restoreErr)
					}
				}
			} else {
				slog.Warn("input backend override active", "backend", "sdl3")
			}
		} else {
			slog.Warn("IW_INPUT_BACKEND=sdl3 requested but SDL3 backend is not available in this build")
		}
	}

	// If no backend was provided by the renderer, allow other build-tagged
	// backends (e.g. SDL3) to provide system input. input.NewSDL3Backend
	// is a no-op stub when the sdl3 build tag is not present.
	if gameInput != nil {
		if err := func() error {
			// Only set SDL3 backend if renderer didn't provide one
			if gameInput.Backend() != nil {
				return nil
			}
			if b := input.NewSDL3Backend(gameInput); b != nil {
				return gameInput.SetBackend(b)
			}
			return nil
		}(); err != nil {
			return fmt.Errorf("failed to set input backend: %w", err)
		}
	}

	// Wire subsystems together through Host.Init
	audioAdapter := audio.NewAudioAdapter(audio.NewSystem())
	gameAudio = audioAdapter
	resetRuntimeSoundState()
	gameSubs = &host.Subsystems{
		Files:    fileSys,
		Commands: globalCommandBuffer{},
		Server:   gameServer,
		Input:    gameInput,
		Audio:    audioAdapter,
	}
	// Wire the loopback client to the server so server→client messages are parsed (M3).
	host.SetupLoopbackClientServer(gameSubs, gameServer)
	registerGameplayBindCommands()
	registerConsoleCompletionProviders()
	applyDefaultGameplayBindings()

	if err := gameHost.Init(&host.InitParams{
		BaseDir:    basedir,
		GameDir:    gamedir,
		UserDir:    "",
		Args:       append([]string(nil), args...),
		MaxClients: 1,
	}, gameSubs); err != nil {
		return fmt.Errorf("failed to initialize host: %w", err)
	}
	applySVolume()

	// Set menu in host
	gameHost.SetMenu(gameMenu)
	gameMenu.SetSaveSlotProvider(func(slotCount int) []menu.SaveSlotInfo {
		hostSlots := gameHost.ListSaveSlots(slotCount)
		menuSlots := make([]menu.SaveSlotInfo, 0, len(hostSlots))
		for _, slot := range hostSlots {
			menuSlots = append(menuSlots, menu.SaveSlotInfo{
				Name:        slot.Name,
				DisplayName: slot.DisplayName,
			})
		}
		return menuSlots
	})

	// Initialize draw manager from the game filesystem (loads gfx.wad from pak files)
	drawErr := gameDraw.Init(fileSys)
	if drawErr != nil {
		// Fall back to local "data" directory for development/testing
		slog.Warn("Failed to initialize draw manager from filesystem, trying data/", "error", drawErr)
		drawErr = gameDraw.InitFromDir("data")
	}
	if drawErr != nil {
		slog.Warn("Failed to initialize draw manager", "error", drawErr)
	} else if gameRenderer != nil {
		if pal := gameDraw.Palette(); len(pal) >= 768 {
			gameRenderer.SetPalette(pal)
		}
		if conchars := gameDraw.GetConcharsData(); len(conchars) >= 128*128 {
			gameRenderer.SetConchars(conchars)
		}
	}

	// Initialize HUD
	gameHUD = hud.NewHUD(gameDraw)
	gameClient = host.LoopbackClientState(gameSubs)
	syncControlCvarsToClient()
	resetRuntimeVisualState()

	// Make sure the menu is visible at startup
	gameMenu.ShowMenu()
	// slog.Info("menu active") - moved to main for deterministic logs

	slog.Info("All subsystems initialized")
	return nil
}

// gameCallbacks implements host.FrameCallbacks to drive server+client each frame.
type gameCallbacks struct{}

func (gameCallbacks) GetEvents() {
	if gameInput != nil {
		gameInput.PollEvents()
	}
	if gameSubs != nil && gameSubs.Client != nil && gameHost != nil {
		_ = gameSubs.Client.Frame(gameHost.FrameTime())
	}
}

func (gameCallbacks) ProcessConsoleCommands() {
	host.DispatchLoopbackStuffText(gameSubs)
}

func (gameCallbacks) ProcessServer() {
	if gameSubs == nil || gameSubs.Server == nil {
		return
	}
	dt := gameHost.FrameTime()
	if err := gameSubs.Server.Frame(dt); err != nil {
		slog.Warn("server frame error", "error", err)
	}
}

func (gameCallbacks) ProcessClient() {
	if gameSubs == nil || gameSubs.Client == nil {
		return
	}

	// Handle demo playback
	if gameHost != nil && gameHost.DemoState() != nil && gameHost.DemoState().Playback {
		demo := gameHost.DemoState()
		if !demo.ShouldReadFrame(gameHost.FrameCount()) {
			return
		}
		clientState := host.LoopbackClientState(gameSubs)
		if clientState != nil {
			clientState.AdvanceTime(demo, gameHost.FrameTime())
			if !shouldReadNextDemoMessage(clientState, demo) {
				return
			}
		}

		// Try to read next demo frame
		msgData, viewAngles, err := demo.ReadDemoFrame()
		if err != nil {
			if err.Error() == "EOF" || err.Error() == "unexpected EOF" {
				// Demo ended, check if we should loop to next demo
				_ = demo.StopPlayback()
				gameHost.SetClientState(0) // caDisconnected

				// Demo loop: play next demo if demo loop is active
				if gameHost.DemoNum() >= 0 && len(gameHost.DemoList()) > 0 {
					demoNum := gameHost.DemoNum()
					demos := gameHost.DemoList()

					// Wrap around to start
					if demoNum >= len(demos) {
						demoNum = 0
						gameHost.SetDemoNum(demoNum)
					}

					if demoNum < len(demos) && demos[demoNum] != "" {
						// Play the next demo
						gameHost.CmdPlaydemo(demos[demoNum], gameSubs)
						// Advance for next time
						gameHost.SetDemoNum(demoNum + 1)
					} else {
						// No more demos
						gameHost.SetDemoNum(-1)
					}
				}
				return
			}
			// Other errors - stop playback
			slog.Warn("demo playback error", "error", err)
			_ = demo.StopPlayback()
			gameHost.SetClientState(0) // caDisconnected
			return
		}

		// Successfully read demo frame - parse the message and apply view angles
		// Get the actual client state to access parser
		if clientState != nil {
			applyDemoPlaybackViewAngles(clientState, viewAngles)

			// Parse the server message from demo
			parser := cl.NewParser(clientState)
			if err := parser.ParseServerMessage(msgData); err != nil {
				slog.Warn("failed to parse demo message", "error", err)
			}
			host.DispatchLoopbackStuffText(gameSubs)

		}

		// Don't run normal networked gameplay during demo playback
		return
	}

	// Normal networked gameplay
	_ = gameSubs.Client.ReadFromServer()
	recordRuntimeDemoFrame()
	host.DispatchLoopbackStuffText(gameSubs)
	_ = gameSubs.Client.SendCommand()
}

func (gameCallbacks) UpdateScreen() {}

func syncAudioViewEntity() {
	if gameAudio == nil {
		return
	}

	viewEntity := 0
	if gameClient != nil {
		viewEntity = gameClient.ViewEntity
	}
	gameAudio.SetViewEntity(viewEntity)
}

func (gameCallbacks) UpdateAudio(origin, forward, right, up [3]float32) {
	if gameAudio == nil {
		return
	}
	syncAudioViewEntity()
	gameAudio.SetListener(origin, forward, right, up)
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

func main() {
	// Logger initialization is handled in logger_*.go files based on build tags
	fmt.Printf("Ironwail-Go v%d.%d.%d\n", VersionMajor, VersionMinor, VersionPatch)
	fmt.Println("A Go port of Ironwail Quake engine")
	fmt.Println()

	// Check if a map argument was provided
	baseDir := flag.String("basedir", ".", "Base Quake directory containing id1")
	gameDir := flag.String("game", "id1", "Game directory (e.g. id1, hipnotic)")
	headlessFlag := flag.Bool("headless", false, "Run without rendering")
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
	headless := *headlessFlag
	initErr := initSubsystems(headless, *baseDir, *gameDir, args)
	if initErr != nil && !headless {
		// Check if error is related to renderer initialization
		if isRendererError(initErr) {
			fmt.Println("WARNING: Renderer initialization failed. Running in headless mode.")
			fmt.Printf("Error: %v\n", initErr)
			fmt.Println("Continuing with game loop (no rendering)...")
			headless = true
			// Re-initialize without renderer
			if err := initSubsystems(true, *baseDir, *gameDir, args); err != nil {
				log.Fatal("Initialization failed:", err)
			}
		} else {
			log.Fatal("Initialization failed:", initErr)
		}
	}
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
	slog.Info("menu active")

	// Execute map command if map argument was provided
	if mapArg != "" {
		slog.Info("map spawn started", "map", mapArg)
		if err := gameHost.CmdMap(mapArg, gameSubs); err != nil {
			log.Printf("Failed to spawn map %s: %v", mapArg, err)
		} else {
			slog.Info("map spawn finished", "map", mapArg)
			if gameClient != nil && gameClient.State == cl.StateActive && gameHost.SignOns() == 4 {
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
				applyGameplayMouseLook()
			}

			runRuntimeFrame(dt, cb)
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

			syncRuntimeVisualEffects(dt)
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

					if gameHost != nil && gameHost.LoadingPlaqueActive(0) {
						drawLoadingPlaque(overlay, gameDraw)
						if consoleVisible {
							console.Draw(overlay, w, h, true)
						}
						return
					}

					if gameMenu != nil && gameMenu.IsActive() {
						gameMenu.M_Draw(overlay)
						return
					}

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
				})
				return
			}

			dc.Clear(0, 0, 0, 1)
			if gameHost != nil && gameHost.LoadingPlaqueActive(0) {
				drawLoadingPlaque(dc, gameDraw)
				return
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
		headlessGameLoop()
	}

	slog.Info("Engine shutdown complete")
}

func collectBrushEntities() []renderer.BrushEntity {
	if gameClient == nil || gameServer == nil || gameServer.WorldTree == nil || len(gameServer.WorldTree.Models) <= 1 {
		return nil
	}

	resolve := func(state inet.EntityState) (renderer.BrushEntity, bool) {
		if state.ModelIndex <= 1 {
			return renderer.BrushEntity{}, false
		}
		precacheIndex := int(state.ModelIndex) - 1
		if precacheIndex < 0 || precacheIndex >= len(gameClient.ModelPrecache) {
			return renderer.BrushEntity{}, false
		}
		modelName := gameClient.ModelPrecache[precacheIndex]
		if len(modelName) < 2 || modelName[0] != '*' {
			return renderer.BrushEntity{}, false
		}
		submodelIndex, err := strconv.Atoi(modelName[1:])
		if err != nil || submodelIndex <= 0 || submodelIndex >= len(gameServer.WorldTree.Models) {
			return renderer.BrushEntity{}, false
		}
		return renderer.BrushEntity{
			SubmodelIndex: submodelIndex,
			Frame:         int(state.Frame),
			Origin:        state.Origin,
			Angles:        state.Angles,
			Alpha:         entityStateAlpha(state),
			Scale:         entityStateScale(state),
		}, true
	}

	brushEntities := make([]renderer.BrushEntity, 0, len(gameClient.Entities)+len(gameClient.StaticEntities))
	for entityNum, state := range gameClient.Entities {
		if entityNum == gameClient.ViewEntity {
			continue
		}
		if brushEntity, ok := resolve(state); ok {
			brushEntities = append(brushEntities, brushEntity)
		}
	}
	for _, state := range gameClient.StaticEntities {
		if brushEntity, ok := resolve(state); ok {
			brushEntities = append(brushEntities, brushEntity)
		}
	}

	return brushEntities
}

func loadAliasModel(modelName string) (*model.Model, bool) {
	if modelName == "" || gameSubs == nil || gameSubs.Files == nil {
		return nil, false
	}
	if aliasModelCache == nil {
		aliasModelCache = make(map[string]*model.Model)
	}
	if mdl, ok := aliasModelCache[modelName]; ok {
		return mdl, mdl != nil
	}

	data, err := gameSubs.Files.LoadFile(modelName)
	if err != nil {
		slog.Debug("alias model load skipped", "model", modelName, "error", err)
		aliasModelCache[modelName] = nil
		return nil, false
	}
	loaded, err := model.LoadAliasModel(bytes.NewReader(data))
	if err != nil {
		slog.Debug("alias model parse skipped", "model", modelName, "error", err)
		aliasModelCache[modelName] = nil
		return nil, false
	}
	loaded.Name = modelName
	aliasModelCache[modelName] = loaded
	return loaded, true
}

func loadSpriteModel(modelName string) (*runtimeSpriteModel, bool) {
	if gameSubs == nil || gameSubs.Files == nil || modelName == "" {
		return nil, false
	}
	if spriteModelCache == nil {
		spriteModelCache = make(map[string]*runtimeSpriteModel)
	}
	if entry, ok := spriteModelCache[modelName]; ok {
		return entry, entry != nil
	}

	data, err := gameSubs.Files.LoadFile(modelName)
	if err != nil {
		slog.Debug("sprite model load skipped", "model", modelName, "error", err)
		spriteModelCache[modelName] = nil
		return nil, false
	}
	loaded, err := model.LoadSprite(bytes.NewReader(data))
	if err != nil {
		slog.Debug("sprite model parse skipped", "model", modelName, "error", err)
		spriteModelCache[modelName] = nil
		return nil, false
	}

	halfWidth := float32(loaded.MaxWidth) * 0.5
	halfHeight := float32(loaded.MaxHeight) * 0.5
	entry := &runtimeSpriteModel{
		model: &model.Model{
			Name:      modelName,
			Type:      model.ModSprite,
			NumFrames: loaded.NumFrames,
			Mins:      [3]float32{-halfWidth, -halfWidth, -halfHeight},
			Maxs:      [3]float32{halfWidth, halfWidth, halfHeight},
		},
		sprite: loaded,
	}
	spriteModelCache[modelName] = entry
	return entry, true
}

func collectAliasEntities() []renderer.AliasModelEntity {
	if gameClient == nil || gameSubs == nil || gameSubs.Files == nil {
		return nil
	}

	resolve := func(state inet.EntityState) (renderer.AliasModelEntity, bool) {
		if state.ModelIndex == 0 {
			return renderer.AliasModelEntity{}, false
		}
		precacheIndex := int(state.ModelIndex) - 1
		if precacheIndex < 0 || precacheIndex >= len(gameClient.ModelPrecache) {
			return renderer.AliasModelEntity{}, false
		}
		modelName := gameClient.ModelPrecache[precacheIndex]
		if modelName == "" || strings.HasPrefix(modelName, "*") || !strings.HasSuffix(strings.ToLower(modelName), ".mdl") {
			return renderer.AliasModelEntity{}, false
		}

		mdl, _ := loadAliasModel(modelName)
		if mdl == nil || mdl.Type != model.ModAlias || mdl.AliasHeader == nil || len(mdl.AliasHeader.Poses) == 0 {
			return renderer.AliasModelEntity{}, false
		}

		frame := int(state.Frame)
		if frame < 0 || frame >= mdl.AliasHeader.NumFrames {
			frame = 0
		}

		return renderer.AliasModelEntity{
			ModelID: modelName,
			Model:   mdl,
			Frame:   frame,
			SkinNum: int(state.Skin),
			Origin:  state.Origin,
			Angles:  state.Angles,
			Alpha:   entityStateAlpha(state),
			Scale:   entityStateScale(state),
		}, true
	}

	aliasEntities := make([]renderer.AliasModelEntity, 0, len(gameClient.Entities)+len(gameClient.StaticEntities))
	for entityNum, state := range gameClient.Entities {
		if entityNum == gameClient.ViewEntity {
			continue
		}
		if aliasEntity, ok := resolve(state); ok {
			aliasEntities = append(aliasEntities, aliasEntity)
		}
	}
	for _, state := range gameClient.StaticEntities {
		if aliasEntity, ok := resolve(state); ok {
			aliasEntities = append(aliasEntities, aliasEntity)
		}
	}

	return aliasEntities
}

func collectEntityEffectSources() []renderer.EntityEffectSource {
	if gameClient == nil {
		return nil
	}

	resolve := func(state inet.EntityState) (renderer.EntityEffectSource, bool) {
		if state.Effects == 0 || state.ModelIndex == 0 {
			return renderer.EntityEffectSource{}, false
		}
		precacheIndex := int(state.ModelIndex) - 1
		if precacheIndex < 0 || precacheIndex >= len(gameClient.ModelPrecache) {
			return renderer.EntityEffectSource{}, false
		}
		modelName := gameClient.ModelPrecache[precacheIndex]
		if modelName == "" || strings.HasPrefix(modelName, "*") || !strings.HasSuffix(strings.ToLower(modelName), ".mdl") {
			return renderer.EntityEffectSource{}, false
		}
		return renderer.EntityEffectSource{
			Origin:  state.Origin,
			Angles:  state.Angles,
			Effects: state.Effects,
		}, true
	}

	sources := make([]renderer.EntityEffectSource, 0, len(gameClient.Entities)+len(gameClient.StaticEntities))
	for _, state := range gameClient.Entities {
		if source, ok := resolve(state); ok {
			sources = append(sources, source)
		}
	}
	for _, state := range gameClient.StaticEntities {
		if source, ok := resolve(state); ok {
			sources = append(sources, source)
		}
	}

	return sources
}

func collectSpriteEntities() []renderer.SpriteEntity {
	if gameClient == nil || gameSubs == nil || gameSubs.Files == nil {
		return nil
	}

	viewForward, viewRight, _ := runtimeAngleVectors(gameClient.ViewAngles)
	resolve := func(state inet.EntityState) (renderer.SpriteEntity, bool) {
		if state.ModelIndex == 0 {
			return renderer.SpriteEntity{}, false
		}
		precacheIndex := int(state.ModelIndex) - 1
		if precacheIndex < 0 || precacheIndex >= len(gameClient.ModelPrecache) {
			return renderer.SpriteEntity{}, false
		}
		modelName := gameClient.ModelPrecache[precacheIndex]
		if modelName == "" || strings.HasPrefix(modelName, "*") || !strings.HasSuffix(strings.ToLower(modelName), ".spr") {
			return renderer.SpriteEntity{}, false
		}

		entry, _ := loadSpriteModel(modelName)
		if entry == nil || entry.model == nil || entry.model.Type != model.ModSprite || entry.sprite == nil || entry.sprite.NumFrames == 0 {
			return renderer.SpriteEntity{}, false
		}

		frame := resolveRuntimeSpriteFrame(entry.sprite, int(state.Frame), state.Angles, viewForward, viewRight, gameClient.Time)

		return renderer.SpriteEntity{
			ModelID:    modelName,
			Model:      entry.model,
			Frame:      frame,
			Origin:     state.Origin,
			Angles:     state.Angles,
			Alpha:      entityStateAlpha(state),
			Scale:      entityStateScale(state),
			SpriteData: entry.sprite,
		}, true
	}

	spriteEntities := make([]renderer.SpriteEntity, 0, len(gameClient.Entities)+len(gameClient.StaticEntities))
	for entityNum, state := range gameClient.Entities {
		if entityNum == gameClient.ViewEntity {
			continue
		}
		if spriteEntity, ok := resolve(state); ok {
			spriteEntities = append(spriteEntities, spriteEntity)
		}
	}
	for _, state := range gameClient.StaticEntities {
		if spriteEntity, ok := resolve(state); ok {
			spriteEntities = append(spriteEntities, spriteEntity)
		}
	}

	return spriteEntities
}

func resolveRuntimeSpriteFrame(sprite *model.MSprite, frame int, entityAngles [3]float32, viewForward, viewRight [3]float32, clientTime float64) int {
	if sprite == nil || sprite.NumFrames == 0 || len(sprite.Frames) == 0 {
		return 0
	}
	if frame < 0 || frame >= sprite.NumFrames || frame >= len(sprite.Frames) {
		frame = 0
	}

	flatOffset := spriteFlatFrameOffset(sprite, frame)
	frameDesc := sprite.Frames[frame]
	switch frameDesc.Type {
	case model.SpriteFrameGroup:
		return flatOffset + resolveRuntimeSpriteGroupSubframe(frameDesc.FramePtr, clientTime)
	case model.SpriteFrameAngled:
		return flatOffset + resolveRuntimeSpriteAngledSubframe(frameDesc.FramePtr, entityAngles, viewForward, viewRight)
	default:
		return flatOffset
	}
}

func resolveRuntimeSpriteGroupSubframe(framePtr interface{}, clientTime float64) int {
	group, ok := framePtr.(*model.MSpriteGroup)
	if !ok || group == nil || group.NumFrames <= 0 || len(group.Intervals) == 0 {
		return 0
	}
	lastInterval := group.Intervals[len(group.Intervals)-1]
	if lastInterval <= 0 {
		return 0
	}

	targetTime := float32(math.Mod(clientTime, float64(lastInterval)))
	if targetTime < 0 {
		targetTime += lastInterval
	}
	for subframe := 0; subframe < group.NumFrames && subframe < len(group.Intervals); subframe++ {
		if targetTime < group.Intervals[subframe] {
			return subframe
		}
	}
	return 0
}

func resolveRuntimeSpriteAngledSubframe(framePtr interface{}, entityAngles [3]float32, viewForward, viewRight [3]float32) int {
	group, ok := framePtr.(*model.MSpriteGroup)
	if !ok || group == nil || group.NumFrames <= 0 || len(group.Frames) == 0 {
		return 0
	}

	frameCount := group.NumFrames
	if len(group.Frames) < frameCount {
		frameCount = len(group.Frames)
	}
	if frameCount <= 0 {
		return 0
	}

	entityForward, _, _ := runtimeAngleVectors(entityAngles)
	forwardDot := qtypes.Vec3Dot(
		qtypes.Vec3{X: viewForward[0], Y: viewForward[1], Z: viewForward[2]},
		qtypes.Vec3{X: entityForward[0], Y: entityForward[1], Z: entityForward[2]},
	)
	rightDot := qtypes.Vec3Dot(
		qtypes.Vec3{X: viewRight[0], Y: viewRight[1], Z: viewRight[2]},
		qtypes.Vec3{X: entityForward[0], Y: entityForward[1], Z: entityForward[2]},
	)

	dir := int((math.Atan2(float64(rightDot), float64(forwardDot)) + 1.125*math.Pi) * (4.0 / math.Pi))
	dir %= frameCount
	if dir < 0 {
		dir += frameCount
	}
	return dir
}

func spriteFlatFrameOffset(sprite *model.MSprite, frame int) int {
	if sprite == nil || frame <= 0 {
		return 0
	}
	maxFrame := frame
	if maxFrame > len(sprite.Frames) {
		maxFrame = len(sprite.Frames)
	}
	offset := 0
	for i := 0; i < maxFrame; i++ {
		offset += spriteFrameSpan(sprite.Frames[i])
	}
	return offset
}

func spriteFrameSpan(frameDesc model.MSpriteFrameDesc) int {
	switch frameDesc.Type {
	case model.SpriteFrameGroup, model.SpriteFrameAngled:
		group, ok := frameDesc.FramePtr.(*model.MSpriteGroup)
		if !ok || group == nil || group.NumFrames <= 0 {
			return 1
		}
		return group.NumFrames
	default:
		return 1
	}
}

func buildRuntimeRenderFrameState(brushEntities []renderer.BrushEntity, aliasEntities []renderer.AliasModelEntity, spriteEntities []renderer.SpriteEntity, viewModel *renderer.AliasModelEntity) *renderer.RenderFrameState {
	state := renderer.DefaultRenderFrameState()
	state.ClearColor = [4]float32{0, 0, 0, 1}
	state.DrawWorld = gameRenderer != nil && gameRenderer.HasWorldData()
	state.DrawEntities = len(brushEntities) > 0 || len(aliasEntities) > 0 || len(spriteEntities) > 0 || viewModel != nil
	state.BrushEntities = brushEntities
	state.AliasEntities = aliasEntities
	state.SpriteEntities = spriteEntities
	state.ViewModel = viewModel
	state.DrawParticles = gameParticles != nil && gameParticles.ActiveCount() > 0
	state.Draw2DOverlay = true
	state.MenuActive = gameMenu != nil && gameMenu.IsActive()
	state.Particles = gameParticles
	if gameDecalMarks != nil {
		state.DecalMarks = gameDecalMarks.ActiveMarks()
	}
	if gameClient != nil {
		state.LightStyles = gameClient.LightStyleValues()
		state.FogDensity, state.FogColor = gameClient.CurrentFog()
	}
	if gameDraw != nil {
		state.Palette = gameDraw.Palette()
	}
	return state
}

func entityStateAlpha(state inet.EntityState) float32 {
	return inet.ENTALPHA_DECODE(state.Alpha)
}

func entityStateScale(state inet.EntityState) float32 {
	scale := inet.ENTSCALE_DECODE(state.Scale)
	if scale <= 0 {
		return 1
	}
	return scale
}

func collectViewModelEntity() *renderer.AliasModelEntity {
	if !runtimeViewModelVisible() {
		return nil
	}

	modelIndex := gameClient.WeaponModelIndex()
	if modelIndex <= 0 {
		return nil
	}
	precacheIndex := modelIndex - 1
	if precacheIndex < 0 || precacheIndex >= len(gameClient.ModelPrecache) {
		return nil
	}

	modelName := gameClient.ModelPrecache[precacheIndex]
	if modelName == "" || strings.HasPrefix(modelName, "*") || !strings.HasSuffix(strings.ToLower(modelName), ".mdl") {
		return nil
	}
	mdl, ok := loadAliasModel(modelName)
	if !ok || mdl == nil || mdl.AliasHeader == nil || mdl.AliasHeader.NumFrames == 0 {
		return nil
	}

	frame := gameClient.WeaponFrame()
	if frame < 0 || frame >= mdl.AliasHeader.NumFrames {
		frame = 0
	}
	origin, _ := runtimeViewState()
	angles := gameClient.ViewAngles
	angles[0] = -angles[0]

	return &renderer.AliasModelEntity{
		ModelID: modelName,
		Model:   mdl,
		Frame:   frame,
		SkinNum: 0,
		Origin:  origin,
		Angles:  angles,
		Alpha:   1,
		Scale:   1,
	}
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

func registerGameplayBindCommands() {
	cmdsys.AddCommand("bind", cmdBind, "Bind a key to a command")
	cmdsys.AddCommand("unbind", cmdUnbind, "Remove a key binding")
	cmdsys.AddCommand("unbindall", cmdUnbindAll, "Remove all key bindings")
	cmdsys.AddCommand("bindlist", cmdBindList, "List all key bindings")
	cmdsys.AddCommand("impulse", cmdImpulse, "Trigger an impulse command")
	cmdsys.AddCommand("toggleconsole", cmdToggleConsole, "Toggle the console")

	registerGameplayButtonCommand("forward", func(c *cl.Client) *cl.KButton { return &c.InputForward })
	registerGameplayButtonCommand("back", func(c *cl.Client) *cl.KButton { return &c.InputBack })
	registerGameplayButtonCommand("moveleft", func(c *cl.Client) *cl.KButton { return &c.InputMoveLeft })
	registerGameplayButtonCommand("moveright", func(c *cl.Client) *cl.KButton { return &c.InputMoveRight })
	registerGameplayButtonCommand("left", func(c *cl.Client) *cl.KButton { return &c.InputLeft })
	registerGameplayButtonCommand("right", func(c *cl.Client) *cl.KButton { return &c.InputRight })
	registerGameplayButtonCommand("speed", func(c *cl.Client) *cl.KButton { return &c.InputSpeed })
	registerGameplayButtonCommand("strafe", func(c *cl.Client) *cl.KButton { return &c.InputStrafe })
	registerGameplayButtonCommand("attack", func(c *cl.Client) *cl.KButton { return &c.InputAttack })
	registerGameplayButtonCommand("jump", func(c *cl.Client) *cl.KButton { return &c.InputJump })
	registerGameplayButtonCommand("use", func(c *cl.Client) *cl.KButton { return &c.InputUse })
	registerGameplayButtonCommand("mlook", func(c *cl.Client) *cl.KButton { return &c.InputMLook })
	registerGameplayButtonCommand("klook", func(c *cl.Client) *cl.KButton { return &c.InputKLook })
	registerGameplayButtonCommand("lookup", func(c *cl.Client) *cl.KButton { return &c.InputLookUp })
	registerGameplayButtonCommand("lookdown", func(c *cl.Client) *cl.KButton { return &c.InputLookDown })
	registerGameplayButtonCommand("up", func(c *cl.Client) *cl.KButton { return &c.InputUp })
	registerGameplayButtonCommand("down", func(c *cl.Client) *cl.KButton { return &c.InputDown })
}

func registerConsoleCompletionProviders() {
	console.SetGlobalCommandProvider(cmdsys.Complete)
	console.SetGlobalCVarProvider(cvar.Complete)
	console.SetGlobalAliasProvider(cmdsys.CompleteAliases)
}

func registerGameplayButtonCommand(name string, selectButton func(*cl.Client) *cl.KButton) {
	cmdsys.AddCommand("+"+name, func(args []string) {
		runGameplayButtonCommand(selectButton, true, args)
	}, "Gameplay button press")
	cmdsys.AddCommand("-"+name, func(args []string) {
		runGameplayButtonCommand(selectButton, false, args)
	}, "Gameplay button release")
}

func runGameplayButtonCommand(selectButton func(*cl.Client) *cl.KButton, down bool, args []string) {
	if gameClient == nil {
		return
	}
	key := -1
	if len(args) > 0 {
		if parsed, err := strconv.Atoi(args[0]); err == nil {
			key = parsed
		}
	}
	button := selectButton(gameClient)
	if down {
		gameClient.KeyDown(button, key)
		return
	}
	gameClient.KeyUp(button, key)
}

func applyDefaultGameplayBindings() {
	if gameInput == nil {
		return
	}
	for _, binding := range gameplayDefaultBindings {
		gameInput.SetBinding(binding.key, binding.command)
	}
}

func parseBindingKey(name string) (int, bool) {
	key := input.StringToKey(strings.ToUpper(name))
	if key <= 0 || key >= input.NumKeycode {
		return 0, false
	}
	return key, true
}

func cmdBind(args []string) {
	if gameInput == nil {
		return
	}
	if len(args) < 1 {
		console.Printf("usage: bind <key> [command]\n")
		return
	}
	key, ok := parseBindingKey(args[0])
	if !ok {
		console.Printf("bind: \"%s\" is not a valid key\n", args[0])
		return
	}
	if len(args) == 1 {
		binding := gameInput.GetBinding(key)
		if binding == "" {
			console.Printf("\"%s\" is not bound\n", args[0])
		} else {
			console.Printf("\"%s\" = \"%s\"\n", args[0], binding)
		}
		return
	}
	gameInput.SetBinding(key, strings.Join(args[1:], " "))
}

func cmdUnbind(args []string) {
	if gameInput == nil {
		return
	}
	if len(args) != 1 {
		console.Printf("usage: unbind <key>\n")
		return
	}
	key, ok := parseBindingKey(args[0])
	if !ok {
		console.Printf("unbind: \"%s\" is not a valid key\n", args[0])
		return
	}
	gameInput.SetBinding(key, "")
}

func cmdUnbindAll(_ []string) {
	if gameInput == nil {
		return
	}
	for key := 0; key < input.NumKeycode; key++ {
		gameInput.SetBinding(key, "")
	}
}

func cmdBindList(_ []string) {
	if gameInput == nil {
		return
	}
	count := 0
	for key := 0; key < input.NumKeycode; key++ {
		binding := gameInput.GetBinding(key)
		if binding == "" {
			continue
		}
		keyName := input.KeyToString(key)
		if keyName == "" {
			keyName = strconv.Itoa(key)
		}
		console.Printf("\"%s\" = \"%s\"\n", keyName, binding)
		count++
	}
	console.Printf("%d bindings\n", count)
}

func cmdImpulse(args []string) {
	if gameClient == nil {
		return
	}
	if len(args) < 1 {
		console.Printf("usage: impulse <value>\n")
		return
	}
	impulse, err := strconv.Atoi(args[0])
	if err != nil {
		console.Printf("impulse: \"%s\" is not a number\n", args[0])
		return
	}
	gameClient.InImpulse = impulse
}

func cmdToggleConsole(_ []string) {
	if gameInput == nil {
		return
	}

	if gameInput.GetKeyDest() == input.KeyConsole {
		console.ResetCompletion()
		gameInput.SetKeyDest(input.KeyGame)
		syncGameplayInputMode()
		return
	}

	if gameMenu != nil && gameMenu.IsActive() {
		gameMenu.HideMenu()
	}
	console.ResetCompletion()
	gameInput.SetKeyDest(input.KeyConsole)
	syncGameplayInputMode()
}

func handleGameKeyEvent(event input.KeyEvent) {
	if gameInput == nil {
		return
	}

	switch gameInput.GetKeyDest() {
	case input.KeyConsole:
		handleConsoleKeyEvent(event)
		return
	case input.KeyGame:
	default:
		return
	}

	if event.Key == input.KEscape && event.Down {
		if gameMenu != nil {
			gameMenu.ToggleMenu()
		}
		syncGameplayInputMode()
		return
	}

	binding := strings.TrimSpace(gameInput.GetBinding(event.Key))
	if binding == "" {
		return
	}
	if strings.HasPrefix(binding, "+") {
		if gameClient == nil {
			return
		}
		command := binding
		if !event.Down {
			command = "-" + binding[1:]
		}
		cmdsys.ExecuteText(fmt.Sprintf("%s %d", command, event.Key))
		return
	}
	if event.Down {
		cmdsys.ExecuteText(binding)
	}
}

func handleMenuKeyEvent(event input.KeyEvent) {
	if !event.Down || gameMenu == nil {
		return
	}
	gameMenu.M_Key(event.Key)
}

func handleMenuCharEvent(ch rune) {
	if gameInput == nil || gameInput.GetKeyDest() != input.KeyMenu || gameMenu == nil {
		return
	}
	gameMenu.M_Char(ch)
}

func handleGameCharEvent(ch rune) {
	if gameInput == nil || gameInput.GetKeyDest() != input.KeyConsole {
		return
	}
	if ch == '`' {
		return
	}
	console.AppendInputRune(ch)
}

func handleConsoleKeyEvent(event input.KeyEvent) {
	if !event.Down {
		return
	}

	switch event.Key {
	case input.KEscape, int('`'):
		console.ResetCompletion()
		gameInput.SetKeyDest(input.KeyGame)
		syncGameplayInputMode()
	case input.KEnter:
		line := strings.TrimSpace(console.CommitInput())
		console.ResetCompletion()
		if line == "" {
			return
		}
		console.Printf("]%s\n", line)
		cmdsys.ExecuteText(line)
	case input.KTab:
		line := console.InputLine()
		completed, matches := console.CompleteInput(line, true)
		if len(matches) == 0 {
			return
		}
		console.SetInputLine(completed)
	case input.KBackspace:
		console.BackspaceInput()
	case input.KUpArrow:
		console.PreviousHistory()
	case input.KDownArrow:
		console.NextHistory()
	case input.KPgUp:
		console.Scroll(2)
	case input.KPgDn:
		console.Scroll(-2)
	case input.KHome:
		console.Scroll(console.TotalLines())
	case input.KEnd:
		console.Scroll(-console.TotalLines())
	}
}

func syncGameplayInputMode() {
	if gameInput == nil {
		return
	}

	menuActive := gameMenu != nil && gameMenu.IsActive()
	wantDest := gameInput.GetKeyDest()
	switch {
	case menuActive:
		wantDest = input.KeyMenu
	case wantDest == input.KeyMenu:
		wantDest = input.KeyGame
	case wantDest != input.KeyConsole:
		wantDest = input.KeyGame
	}
	if gameInput.GetKeyDest() != wantDest {
		gameInput.SetKeyDest(wantDest)
	}

	shouldGrab := !menuActive && wantDest == input.KeyGame
	if shouldGrab == gameMouseGrabbed {
		return
	}

	gameInput.SetMouseGrab(shouldGrab)
	gameInput.ClearState()
	if !shouldGrab {
		releaseGameplayButtons()
	}
	gameMouseGrabbed = shouldGrab
}

func applyGameplayMouseLook() {
	if gameInput == nil || gameClient == nil {
		return
	}
	if gameInput.GetKeyDest() != input.KeyGame {
		gameInput.ClearState()
		return
	}

	state := gameInput.GetState()
	sensitivity := float32(cvar.FloatValue("sensitivity"))
	if sensitivity <= 0 {
		sensitivity = 1
	}
	yawScale := sensitivity * float32(cvar.FloatValue("m_yaw"))
	if yawScale == 0 {
		yawScale = 0.15
	}
	pitchScale := sensitivity * float32(cvar.FloatValue("m_pitch"))
	if pitchScale == 0 {
		pitchScale = 0.12
	}
	mouseLook := gameClient.FreeLook || gameClient.InputMLook.State&1 != 0
	if state.MouseDX != 0 {
		gameClient.ViewAngles[1] -= float32(state.MouseDX) * yawScale
	}
	if state.MouseDY != 0 && mouseLook {
		gameClient.ViewAngles[0] += float32(state.MouseDY) * pitchScale
		if gameClient.ViewAngles[0] > gameClient.MaxPitch {
			gameClient.ViewAngles[0] = gameClient.MaxPitch
		}
		if gameClient.ViewAngles[0] < gameClient.MinPitch {
			gameClient.ViewAngles[0] = gameClient.MinPitch
		}
	}
	gameInput.ClearState()
}

func releaseGameplayButtons() {
	if gameClient == nil {
		return
	}
	buttons := []*cl.KButton{
		&gameClient.InputForward,
		&gameClient.InputBack,
		&gameClient.InputLeft,
		&gameClient.InputRight,
		&gameClient.InputUp,
		&gameClient.InputDown,
		&gameClient.InputLookUp,
		&gameClient.InputLookDown,
		&gameClient.InputMoveLeft,
		&gameClient.InputMoveRight,
		&gameClient.InputStrafe,
		&gameClient.InputSpeed,
		&gameClient.InputUse,
		&gameClient.InputJump,
		&gameClient.InputAttack,
		&gameClient.InputKLook,
		&gameClient.InputMLook,
	}
	for _, button := range buttons {
		gameClient.KeyUp(button, -1)
	}
}

func headlessGameLoop() {
	slog.Info("Starting headless game loop")

	// Simple game loop without rendering
	slog.Info("frame loop started")
	lastTime := time.Now()
	ticker := time.NewTicker(time.Second / 250) // 250 FPS target
	defer ticker.Stop()

	for range ticker.C {
		if gameHost != nil && gameHost.IsAborted() {
			return
		}
		now := time.Now()
		dt := now.Sub(lastTime).Seconds()
		lastTime = now

		// Update game state
		if err := gameHost.Frame(dt, gameCallbacks{}); err != nil {
			log.Fatal("host frame error", err)
		}
		if gameHost != nil && gameHost.IsAborted() {
			return
		}
	}
}

func runtimeViewState() (origin, angles [3]float32) {
	origin = [3]float32{0, 0, 128}
	angles = [3]float32{45, 0, 0}
	foundPlayerStart := false

	if gameServer != nil {
		for _, ent := range gameServer.Edicts {
			if ent == nil || ent.Free || ent.Vars == nil || ent.Vars.ClassName == 0 {
				continue
			}
			className := gameServer.GetString(ent.Vars.ClassName)
			if className != "info_player_start" && className != "info_player_deathmatch" {
				continue
			}
			origin = ent.Vars.Origin
			origin[2] += 22
			angles = ent.Vars.Angles
			foundPlayerStart = true
			break
		}
	}

	if !foundPlayerStart && gameRenderer != nil {
		if minBounds, maxBounds, ok := gameRenderer.GetWorldBounds(); ok {
			centerX := (minBounds[0] + maxBounds[0]) * 0.5
			centerY := (minBounds[1] + maxBounds[1]) * 0.5
			centerZ := (minBounds[2] + maxBounds[2]) * 0.5

			extentX := maxBounds[0] - minBounds[0]
			extentY := maxBounds[1] - minBounds[1]
			extentZ := maxBounds[2] - minBounds[2]

			radius := extentX
			if extentY > radius {
				radius = extentY
			}
			if extentZ > radius {
				radius = extentZ
			}
			if radius < 256 {
				radius = 256
			}

			origin = [3]float32{centerX, centerY + radius, centerZ + radius*0.5}
			angles = [3]float32{26.565052, 0, 0}
		}
	}

	if gameClient != nil {
		if clientOrigin, ok := runtimePlayerOrigin(); ok {
			clientOrigin[2] += gameClient.ViewHeight
			return clientOrigin, gameClient.ViewAngles
		}
	}

	return origin, angles
}

func runtimePlayerOrigin() ([3]float32, bool) {
	if gameClient == nil {
		return [3]float32{}, false
	}

	if gameClient.ViewEntity != 0 {
		if state, ok := gameClient.Entities[gameClient.ViewEntity]; ok {
			return state.Origin, true
		}
	}

	if gameClient.ViewEntity == 0 {
		if state, ok := gameClient.Entities[0]; ok {
			return state.Origin, true
		}
	}

	clientOrigin := gameClient.PredictedOrigin
	if clientOrigin[0] != 0 || clientOrigin[1] != 0 || clientOrigin[2] != 0 {
		return clientOrigin, true
	}

	return [3]float32{}, false
}

func runtimeCameraState(origin, angles [3]float32) renderer.CameraState {
	camera := renderer.ConvertClientStateToCamera(origin, angles, 96.0)
	if gameClient != nil {
		if gameClient.Intermission == 0 {
			camera.Angles.X += gameClient.PunchAngle[0]
			camera.Angles.Y += gameClient.PunchAngle[1]
			camera.Angles.Z += gameClient.PunchAngle[2]
		}
		camera.Time = float32(gameClient.Time)
	}
	return camera
}

type picProvider interface {
	GetPic(name string) *qimage.QPic
}

func drawLoadingPlaque(dc renderer.RenderContext, pics picProvider) {
	if pics == nil {
		return
	}

	if plaque := pics.GetPic("gfx/qplaque.lmp"); plaque != nil {
		dc.DrawMenuPic(16, 4, plaque)
	}
	if loading := pics.GetPic("gfx/loading.lmp"); loading != nil {
		dc.DrawMenuPic((320-int(loading.Width))/2, (240-48-int(loading.Height))/2, loading)
	}
}

func applyDemoPlaybackViewAngles(clientState *cl.Client, viewAngles [3]float32) {
	if clientState == nil {
		return
	}
	clientState.MViewAngles[1] = clientState.MViewAngles[0]
	clientState.MViewAngles[0] = viewAngles
	clientState.ViewAngles = viewAngles
}

func shouldReadNextDemoMessage(clientState *cl.Client, demo *cl.DemoState) bool {
	if clientState == nil || demo == nil {
		return true
	}
	if clientState.Signon < cl.Signons {
		return true
	}
	if demo.Speed > 0 {
		return clientState.Time > clientState.MTime[0]
	}
	if demo.Speed < 0 {
		return clientState.Time < clientState.MTime[0]
	}
	return false
}

func recordRuntimeDemoFrame() {
	if gameHost == nil || gameSubs == nil || gameSubs.Client == nil || gameClient == nil {
		return
	}

	demo := gameHost.DemoState()
	if demo == nil || !demo.Recording {
		return
	}

	source, ok := gameSubs.Client.(interface{ LastServerMessage() []byte })
	if !ok {
		return
	}
	message := source.LastServerMessage()
	if len(message) == 0 {
		return
	}

	if err := demo.WriteDemoFrame(message, gameClient.ViewAngles); err != nil {
		slog.Warn("failed to record demo frame", "error", err)
	}
}

func runtimeAngleVectors(angles [3]float32) (forward, right, up [3]float32) {
	forwardVec, rightVec, upVec := qtypes.AngleVectors(qtypes.Vec3{
		X: angles[0],
		Y: angles[1],
		Z: angles[2],
	})
	return [3]float32{forwardVec.X, forwardVec.Y, forwardVec.Z},
		[3]float32{rightVec.X, rightVec.Y, rightVec.Z},
		[3]float32{upVec.X, upVec.Y, upVec.Z}
}

func resetRuntimeSoundState() {
	soundSFXByIndex = nil
	menuSFXByName = nil
	soundPrecacheKey = ""
	staticSoundKey = ""
	musicTrackKey = ""
}

func resetRuntimeVisualState() {
	if gameRenderer == nil {
		gameParticles = nil
		gameDecalMarks = nil
		particleRNG = nil
		particleTime = 0
		skyboxNameKey = ""
		return
	}

	gameParticles = renderer.NewParticleSystem(renderer.MaxParticles)
	gameDecalMarks = renderer.NewDecalMarkSystem()
	particleRNG = rand.New(rand.NewSource(1))
	particleTime = 0
	skyboxNameKey = ""
}

func syncRuntimeVisualEffects(dt float64) {
	if gameParticles == nil && gameDecalMarks == nil && gameRenderer == nil {
		return
	}

	if gameClient == nil || gameClient.State != cl.StateActive {
		if gameClient != nil {
			gameClient.ConsumeParticleEvents()
			gameClient.ConsumeTempEntities()
		}
		if gameRenderer != nil {
			gameRenderer.ClearDynamicLights()
		}
		if (gameParticles != nil && gameParticles.ActiveCount() > 0) || (gameDecalMarks != nil && gameDecalMarks.ActiveCount() > 0) {
			resetRuntimeVisualState()
		}
		return
	}

	oldTime := particleTime
	particleTime += float32(dt)

	particleEvents := gameClient.ConsumeParticleEvents()
	tempEntities := gameClient.ConsumeTempEntities()
	effectSources := collectEntityEffectSources()

	if gameRenderer != nil {
		gameRenderer.UpdateLights(float32(dt))
		renderer.EmitDynamicLights(gameRenderer.SpawnDynamicLight, tempEntities)
		renderer.EmitEntityEffectLights(gameRenderer.SpawnDynamicLight, effectSources)
	}
	if gameParticles != nil {
		renderer.EmitClientEffects(gameParticles, particleEvents, tempEntities, particleRNG, particleTime)
		renderer.EmitEntityEffectParticles(gameParticles, effectSources, particleTime)
		gameParticles.RunParticles(particleTime, oldTime, 800)
	}
	if gameDecalMarks != nil {
		gameDecalMarks.Run(particleTime)
		renderer.EmitDecalMarks(gameDecalMarks, tempEntities, particleRNG, particleTime)
	}
}

func syncRuntimeSkybox() {
	if gameRenderer == nil {
		skyboxNameKey = ""
		return
	}
	skyboxName := ""
	if gameClient != nil && gameClient.State == cl.StateActive {
		skyboxName = gameClient.SkyboxName
	}
	if skyboxName == skyboxNameKey {
		return
	}
	skyboxNameKey = skyboxName
	if skyboxName == "" || gameSubs == nil || gameSubs.Files == nil {
		gameRenderer.SetExternalSkybox("", nil)
		return
	}
	gameRenderer.SetExternalSkybox(skyboxName, gameSubs.Files.LoadFile)
}

func refreshRuntimeSoundCache() {
	if gameClient == nil {
		resetRuntimeSoundState()
		return
	}
	key := strings.Join(gameClient.SoundPrecache, "\x00")
	if key == soundPrecacheKey {
		return
	}
	soundPrecacheKey = key
	soundSFXByIndex = make(map[int]*audio.SFX)
}

func resolveRuntimeSFX(soundIndex int) *audio.SFX {
	if gameAudio == nil || gameClient == nil || gameSubs == nil || gameSubs.Files == nil || soundIndex <= 0 {
		return nil
	}
	refreshRuntimeSoundCache()
	if sfx, ok := soundSFXByIndex[soundIndex]; ok {
		return sfx
	}
	precacheIndex := soundIndex - 1
	if precacheIndex < 0 || precacheIndex >= len(gameClient.SoundPrecache) {
		soundSFXByIndex[soundIndex] = nil
		return nil
	}
	soundName := gameClient.SoundPrecache[precacheIndex]
	if soundName == "" {
		soundSFXByIndex[soundIndex] = nil
		return nil
	}
	sfx := gameAudio.PrecacheSound(soundName, func() ([]byte, error) {
		return gameSubs.Files.LoadFile("sound/" + soundName)
	})
	soundSFXByIndex[soundIndex] = sfx
	return sfx
}

func resolveMenuSFX(name string) *audio.SFX {
	if gameAudio == nil || gameSubs == nil || gameSubs.Files == nil || name == "" {
		return nil
	}
	if menuSFXByName == nil {
		menuSFXByName = make(map[string]*audio.SFX)
	}
	if sfx, ok := menuSFXByName[name]; ok {
		return sfx
	}
	sfx := gameAudio.PrecacheSound(name, func() ([]byte, error) {
		return gameSubs.Files.LoadFile("sound/" + name)
	})
	menuSFXByName[name] = sfx
	return sfx
}

func playMenuSound(name string) {
	sfx := resolveMenuSFX(name)
	if sfx == nil {
		return
	}
	gameAudio.StartSound(0, 0, sfx, [3]float32{}, 1, 0)
}

func applySVolume() {
	if gameAudio == nil {
		return
	}
	vol := 0.7
	if cv := cvar.Get("s_volume"); cv != nil {
		vol = cv.Float
	}
	gameAudio.SetVolume(vol)
}

func buildRuntimeStaticSoundKey(c *cl.Client) string {
	if c == nil {
		return ""
	}
	var b strings.Builder
	b.Grow(64 + len(c.SoundPrecache)*16 + len(c.StaticSounds)*48)
	fmt.Fprintf(&b, "%p", c)
	b.WriteByte('\x1f')
	b.WriteString(strconv.Itoa(int(c.State)))
	b.WriteByte('\x1f')
	b.WriteString(soundPrecacheKey)
	for _, snd := range c.StaticSounds {
		b.WriteByte('\x1f')
		b.WriteString(strconv.Itoa(snd.SoundIndex))
		b.WriteByte('\x1e')
		b.WriteString(strconv.Itoa(snd.Volume))
		b.WriteByte('\x1e')
		b.WriteString(strconv.FormatUint(uint64(math.Float32bits(snd.Attenuation)), 16))
		for i := 0; i < 3; i++ {
			b.WriteByte('\x1e')
			b.WriteString(strconv.FormatUint(uint64(math.Float32bits(snd.Origin[i])), 16))
		}
	}
	return b.String()
}

func syncRuntimeStaticSounds() {
	if gameAudio == nil {
		staticSoundKey = ""
		return
	}
	if gameClient == nil || gameClient.State != cl.StateActive {
		if staticSoundKey != "" {
			gameAudio.ClearStaticSounds()
			staticSoundKey = ""
		}
		return
	}

	refreshRuntimeSoundCache()
	key := buildRuntimeStaticSoundKey(gameClient)
	if key == staticSoundKey {
		return
	}

	gameAudio.ClearStaticSounds()
	for _, staticSound := range gameClient.StaticSounds {
		sfx := resolveRuntimeSFX(staticSound.SoundIndex)
		if sfx == nil {
			continue
		}
		gameAudio.StartStaticSound(
			sfx,
			staticSound.Origin,
			float32(staticSound.Volume)/255.0,
			staticSound.Attenuation,
		)
	}
	staticSoundKey = key
}

func runtimeMusicSelection() (track, loopTrack int) {
	if gameHost != nil {
		if demo := gameHost.DemoState(); demo != nil && demo.Playback {
			if gameClient != nil && gameClient.CDTrack != 0 {
				track = gameClient.CDTrack
				loopTrack = gameClient.LoopTrack
			} else if demo.CDTrack != 0 {
				track = demo.CDTrack
				loopTrack = demo.CDTrack
			}
			if track != 0 && loopTrack == 0 {
				loopTrack = track
			}
			return track, loopTrack
		}
	}
	if gameClient == nil {
		return 0, 0
	}
	track = gameClient.CDTrack
	loopTrack = gameClient.LoopTrack
	if track != 0 && loopTrack == 0 {
		loopTrack = track
	}
	return track, loopTrack
}

func syncRuntimeMusic() {
	track, loopTrack := runtimeMusicSelection()
	key := fmt.Sprintf("%d/%d", track, loopTrack)

	if gameAudio == nil {
		musicTrackKey = ""
		return
	}
	if key == musicTrackKey {
		return
	}
	musicTrackKey = key
	if track == 0 {
		gameAudio.StopMusic()
		return
	}
	if gameSubs == nil || gameSubs.Files == nil {
		gameAudio.StopMusic()
		slog.Warn("cannot play cd track without filesystem", "track", track)
		return
	}
	if err := gameAudio.PlayCDTrack(track, loopTrack, func(name string) ([]byte, error) {
		return gameSubs.Files.LoadFile(name)
	}); err != nil {
		slog.Warn("failed to play cd track", "track", track, "loop", loopTrack, "error", err)
	}
}

func processRuntimeAudioEvents(viewOrigin [3]float32) {
	if gameClient == nil {
		return
	}
	soundEvents := gameClient.ConsumeSoundEvents()
	stopEvents := gameClient.ConsumeStopSoundEvents()
	if gameAudio == nil {
		return
	}
	for _, stopEvent := range stopEvents {
		gameAudio.StopSound(stopEvent.Entity, stopEvent.Channel)
	}
	for _, soundEvent := range soundEvents {
		sfx := resolveRuntimeSFX(soundEvent.SoundIndex)
		if sfx == nil {
			continue
		}
		origin := soundEvent.Origin
		entNum := soundEvent.Entity
		entChannel := soundEvent.Channel
		attenuation := soundEvent.Attenuation
		if soundEvent.Local {
			origin = viewOrigin
			attenuation = 0
			if gameClient.ViewEntity != 0 {
				entNum = gameClient.ViewEntity
			}
		}
		gameAudio.StartSound(
			entNum,
			entChannel,
			sfx,
			origin,
			float32(soundEvent.Volume)/255.0,
			attenuation,
		)
	}
}

func runRuntimeFrame(dt float64, cb gameCallbacks) {
	if gameHost != nil {
		gameHost.Frame(dt, cb)
	}
	syncControlCvarsToClient()
	if gameClient != nil {
		gameClient.PredictPlayers(float32(dt))
	}
	viewOrigin, viewAngles := runtimeViewState()
	syncRuntimeSkybox()
	if gameAudio != nil {
		forward, right, up := runtimeAngleVectors(viewAngles)
		syncAudioViewEntity()
		gameAudio.SetListener(viewOrigin, forward, right, up)
		syncRuntimeStaticSounds()
		syncRuntimeMusic()
		processRuntimeAudioEvents(viewOrigin)
		gameAudio.Update(viewOrigin, forward, right, up)
	}
}

func isRendererError(err error) bool {
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "renderer") ||
		strings.Contains(errStr, "wayland") ||
		strings.Contains(errStr, "configure") ||
		strings.Contains(errStr, "display") ||
		strings.Contains(errStr, "window") ||
		strings.Contains(errStr, "surface") ||
		strings.Contains(errStr, "segv")
}

func captureScreenshot(sspath, _, _ string) error {
	const (
		ssWidth  = 1280
		ssHeight = 720
	)

	var palette []byte
	if gameDraw != nil {
		palette = gameDraw.Palette()
	}
	soft := renderer.NewSoftwareRenderer(ssWidth, ssHeight, 1.0, palette)

	// Sky-blue background
	soft.Clear(0.08, 0.08, 0.18, 1.0)

	// Render BSP world geometry if a map is loaded
	if gameServer != nil && gameServer.WorldTree != nil {
		soft.DrawBSPWorld(gameServer.WorldTree)
	}

	// Render 2D overlay (menu if active)
	if gameMenu != nil && gameMenu.IsActive() {
		gameMenu.M_Draw(soft)
	}

	f, err := os.Create(sspath)
	if err != nil {
		return fmt.Errorf("create screenshot file: %w", err)
	}
	defer f.Close()

	if err := png.Encode(f, soft.Image()); err != nil {
		return fmt.Errorf("encode PNG: %w", err)
	}

	slog.Info("Screenshot saved", "path", sspath)
	return nil
}

// updateHUDFromServer pushes current player/client state into the HUD.
func updateHUDFromServer() {
	if gameHUD == nil {
		return
	}

	if gameClient != nil {
		shells, nails, rockets, cells := gameClient.AmmoCounts()
		gameHUD.SetState(hud.State{
			Health:        gameClient.Health(),
			Armor:         gameClient.Armor(),
			Ammo:          gameClient.Ammo(),
			WeaponModel:   gameClient.WeaponModelIndex(),
			ActiveWeapon:  gameClient.ActiveWeapon(),
			Shells:        shells,
			Nails:         nails,
			Rockets:       rockets,
			Cells:         cells,
			Items:         gameClient.Items,
			Intermission:  gameClient.Intermission,
			CompletedTime: gameClient.CompletedTime,
			Time:          gameClient.Time,
			CenterPrint:   gameClient.CenterPrint,
			CenterPrintAt: gameClient.CenterPrintAt,
			LevelName:     gameClient.LevelName,
			Secrets:       gameClient.Stats[cl.StatSecrets],
			TotalSecrets:  gameClient.Stats[cl.StatTotalSecrets],
			Monsters:      gameClient.Stats[cl.StatMonsters],
			TotalMonsters: gameClient.Stats[cl.StatTotalMonsters],
		})
		return
	}

	if gameServer == nil {
		return
	}
	ent := gameServer.EdictNum(1)
	if ent == nil {
		return
	}
	gameHUD.SetState(hud.State{
		Health:      int(ent.Vars.Health),
		Armor:       int(ent.Vars.ArmorValue),
		Ammo:        int(ent.Vars.CurrentAmmo),
		WeaponModel: int(ent.Vars.Weapon),
	})
}
