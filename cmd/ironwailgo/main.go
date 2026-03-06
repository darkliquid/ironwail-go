package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/png"
	"log"
	"log/slog"
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
)

var (
	gameHost      *host.Host
	gameServer    *server.Server
	gameQC        *qc.VM
	gameRenderer  *renderer.Renderer
	gameSubs      *host.Subsystems // Store subsystems for command execution
	gameClient    *cl.Client
	gameParticles *renderer.ParticleSystem
	particleRNG   *rand.Rand
	particleTime  float32

	// Menu subsystem
	gameMenu  *menu.Manager
	gameInput *input.System
	gameDraw  *draw.Manager
	gameHUD   *hud.HUD

	gameMouseGrabbed bool
	aliasModelCache  map[string]*model.Model
)

const (
	mouseYawScale   = 0.15
	mousePitchScale = 0.12
)

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
	cvar.Register("s_volume", "0.7", cvar.FlagArchive, "Sound volume")
	cvar.Register("r_gamma", "1.0", cvar.FlagArchive, "Gamma correction")
	cvar.Register("developer", "0", 0, "Developer mode")

	// Create host instance
	gameHost = host.NewHost()

	return nil
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

	// Set up menu input callbacks
	gameInput.OnMenuKey = func(event input.KeyEvent) {
		gameMenu.M_Key(event.Key)
	}
	gameInput.OnKey = handleGameKeyEvent

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
	gameSubs = &host.Subsystems{
		Files:    fileSys,
		Commands: globalCommandBuffer{},
		Server:   gameServer,
		Audio:    audioAdapter,
	}
	// Wire the loopback client to the server so server→client messages are parsed (M3).
	host.SetupLoopbackClientServer(gameSubs, gameServer)

	if err := gameHost.Init(&host.InitParams{
		BaseDir:    basedir,
		GameDir:    gamedir,
		UserDir:    "",
		Args:       append([]string(nil), args...),
		MaxClients: 1,
	}, gameSubs); err != nil {
		return fmt.Errorf("failed to initialize host: %w", err)
	}

	// Set menu in host
	gameHost.SetMenu(gameMenu)

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
	if gameRenderer != nil {
		gameParticles = renderer.NewParticleSystem(renderer.MaxParticles)
		particleRNG = rand.New(rand.NewSource(1))
		particleTime = 0
	}

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
	_ = gameSubs.Client.ReadFromServer()
	_ = gameSubs.Client.SendCommand()
}

func (gameCallbacks) UpdateScreen() {}

func (gameCallbacks) UpdateAudio(_, _, _, _ [3]float32) {}

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

			gameHost.Frame(dt, cb)

			// Update camera from client state each frame
			// This is the critical rendering path for M4: view setup
			if gameRenderer != nil {
				// Fallback: position camera above world center, looking down
				// In Quake coords: origin at (X, Y, Z) with angles (pitch, yaw, roll)
				// Positive pitch = look down
				origin := [3]float32{0, 0, 128} // High above origin
				angles := [3]float32{45, 0, 0}  // Look down 45 degrees
				fallbackFromPlayerStart := false
				fallbackFromWorldBounds := false
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
						fallbackFromPlayerStart = true
						break
					}
				}
				if !fallbackFromPlayerStart {
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
						fallbackFromWorldBounds = true
					}
				}
				if gameClient != nil {
					clientOrigin := gameClient.PredictedOrigin
					clientAngles := gameClient.ViewAngles
					// Only use client values if they're non-zero (player has spawned)
					if clientOrigin[0] != 0 || clientOrigin[1] != 0 || clientOrigin[2] != 0 {
						origin = clientOrigin
						angles = clientAngles
					} else {
						slog.Debug("Client at origin, using fallback camera", "origin", origin, "angles", angles, "from_world_bounds", fallbackFromWorldBounds, "from_player_start", fallbackFromPlayerStart)
					}
				} else {
					slog.Debug("No client, using fallback camera", "origin", origin, "angles", angles, "from_world_bounds", fallbackFromWorldBounds, "from_player_start", fallbackFromPlayerStart)
				}

				// Create camera state from client prediction (or fallback values)
				camera := renderer.ConvertClientStateToCamera(origin, angles, 96.0)

				// Update renderer matrices (near=0.1, far=4096 for Quake world)
				gameRenderer.UpdateCamera(camera, 0.1, 4096.0)
			}

			if gameParticles != nil && gameClient != nil {
				oldTime := particleTime
				particleTime += float32(dt)
				renderer.EmitClientEffects(gameParticles, gameClient.ConsumeParticleEvents(), gameClient.ConsumeTempEntities(), particleRNG, particleTime)
				gameParticles.RunParticles(particleTime, oldTime, 800)
			}
		})
		gameRenderer.OnDraw(func(dc renderer.RenderContext) {
			if gameRenderer != nil && gameServer != nil && gameServer.WorldTree != nil && !gameRenderer.HasWorldData() {
				if err := gameRenderer.UploadWorld(gameServer.WorldTree); err != nil {
					slog.Warn("deferred world upload failed", "error", err)
				}
			}

			brushEntities := collectBrushEntities()
			aliasEntities := collectAliasEntities()
			viewModel := collectViewModelEntity()

			if drawCtx, ok := dc.(*renderer.DrawContext); ok {
				state := renderer.DefaultRenderFrameState()
				state.ClearColor = [4]float32{0, 0, 0, 1}
				state.DrawWorld = gameRenderer != nil && gameRenderer.HasWorldData()
				state.DrawEntities = len(brushEntities) > 0 || len(aliasEntities) > 0 || viewModel != nil
				state.BrushEntities = brushEntities
				state.AliasEntities = aliasEntities
				state.ViewModel = viewModel
				state.DrawParticles = false
				state.Draw2DOverlay = true
				state.MenuActive = gameMenu != nil && gameMenu.IsActive()
				state.Particles = gameParticles
				if gameClient != nil {
					state.LightStyles = gameClient.LightStyleValues()
				}
				if gameDraw != nil {
					state.Palette = gameDraw.Palette()
				}
				drawCtx.RenderFrame(state, func(overlay renderer.RenderContext) {
					if gameMenu != nil && gameMenu.IsActive() {
						gameMenu.M_Draw(overlay)
					} else if gameHUD != nil {
						w, h := gameRenderer.Size()
						gameHUD.SetScreenSize(w, h)
						updateHUDFromServer()
						gameHUD.Draw(overlay)
					}
				})
				return
			}

			dc.Clear(0, 0, 0, 1)
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
			Origin:        state.Origin,
			Angles:        state.Angles,
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
		alpha := float32(1)
		if state.Alpha > 0 && state.Alpha < 255 {
			alpha = float32(state.Alpha) / 255.0
		}

		return renderer.AliasModelEntity{
			ModelID: modelName,
			Model:   mdl,
			Frame:   frame,
			SkinNum: int(state.Skin),
			Origin:  state.Origin,
			Angles:  state.Angles,
			Alpha:   alpha,
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

func collectViewModelEntity() *renderer.AliasModelEntity {
	if gameClient == nil || gameMenu == nil || gameMenu.IsActive() {
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
	angles := gameClient.ViewAngles
	angles[0] = -angles[0]

	return &renderer.AliasModelEntity{
		ModelID: modelName,
		Model:   mdl,
		Frame:   frame,
		SkinNum: 0,
		Origin:  gameClient.PredictedOrigin,
		Angles:  angles,
		Alpha:   1,
	}
}

func handleGameKeyEvent(event input.KeyEvent) {
	if gameInput == nil {
		return
	}

	if gameInput.GetKeyDest() != input.KeyGame {
		return
	}

	if event.Key == input.KEscape && event.Down {
		if gameMenu != nil {
			gameMenu.ToggleMenu()
		}
		syncGameplayInputMode()
		return
	}

	if gameClient == nil {
		return
	}

	applyButton := func(button *cl.KButton) {
		if event.Down {
			gameClient.KeyDown(button, event.Key)
		} else {
			gameClient.KeyUp(button, event.Key)
		}
	}

	switch event.Key {
	case int('w'), input.KUpArrow:
		applyButton(&gameClient.InputForward)
	case int('s'), input.KDownArrow:
		applyButton(&gameClient.InputBack)
	case int('a'):
		applyButton(&gameClient.InputMoveLeft)
	case int('d'):
		applyButton(&gameClient.InputMoveRight)
	case input.KLeftArrow:
		applyButton(&gameClient.InputLeft)
	case input.KRightArrow:
		applyButton(&gameClient.InputRight)
	case input.KShift:
		applyButton(&gameClient.InputSpeed)
	case input.KAlt:
		applyButton(&gameClient.InputStrafe)
	case input.KCtrl, input.KMouse1:
		applyButton(&gameClient.InputAttack)
	case input.KSpace, input.KMouse2:
		applyButton(&gameClient.InputJump)
	case int('e'):
		applyButton(&gameClient.InputUse)
	case input.KMouse3:
		applyButton(&gameClient.InputMLook)
	case input.KMWheelUp:
		if event.Down {
			gameClient.InImpulse = 10
		}
	case input.KMWheelDown:
		if event.Down {
			gameClient.InImpulse = 12
		}
	}
}

func syncGameplayInputMode() {
	if gameInput == nil {
		return
	}

	menuActive := gameMenu != nil && gameMenu.IsActive()
	wantDest := input.KeyGame
	if menuActive {
		wantDest = input.KeyMenu
	}
	if gameInput.GetKeyDest() != wantDest {
		gameInput.SetKeyDest(wantDest)
	}

	shouldGrab := !menuActive
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
	if state.MouseDX != 0 {
		gameClient.ViewAngles[1] -= float32(state.MouseDX) * mouseYawScale
	}
	if state.MouseDY != 0 {
		gameClient.ViewAngles[0] += float32(state.MouseDY) * mousePitchScale
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
		now := time.Now()
		dt := now.Sub(lastTime).Seconds()
		lastTime = now

		// Update game state
		if err := gameHost.Frame(dt, gameCallbacks{}); err != nil {
			log.Fatal("host frame error", err)
		}
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

// updateHUDFromServer reads player state from the server's player edict and
// pushes it into the HUD so it displays current health/armor/ammo.
func updateHUDFromServer() {
	if gameHUD == nil || gameServer == nil {
		return
	}
	ent := gameServer.EdictNum(1)
	if ent == nil {
		return
	}
	gameHUD.SetState(
		int(ent.Vars.Health),
		int(ent.Vars.ArmorValue),
		int(ent.Vars.CurrentAmmo),
		int(ent.Vars.Weapon),
	)
}
