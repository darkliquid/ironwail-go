package main

import (
	"bytes"
	"flag"
	"fmt"
	"image/png"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/ironwail/ironwail-go/internal/audio"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/host"
	"github.com/ironwail/ironwail-go/internal/hud"
	"github.com/ironwail/ironwail-go/internal/input"
	"github.com/ironwail/ironwail-go/internal/menu"
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
	gameHost     *host.Host
	gameServer   *server.Server
	gameQC       *qc.VM
	gameRenderer *renderer.Renderer
	gameSubs     *host.Subsystems // Store subsystems for command execution

	// Menu subsystem
	gameMenu  *menu.Manager
	gameInput *input.System
	gameDraw  *draw.Manager
	gameHUD   *hud.HUD
)

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
	// Create renderer instance from cvars
	cfg := renderer.ConfigFromCvars()

	tr, err := renderer.NewWithConfig(cfg)
	if err != nil {
		return fmt.Errorf("failed to create renderer: %w", err)
	}
	gameRenderer = tr

	return nil
}

func initSubsystems(headless bool, basedir, gamedir string) error {
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
		if bb := gameRenderer.InputBackendForSystem; bb != nil {
			gameInput.SetBackend(gameRenderer.InputBackendForSystem(gameInput))
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
		Files:  fileSys,
		Server: gameServer,
		Audio:  audioAdapter,
	}
	// Wire the loopback client to the server so server→client messages are parsed (M3).
	host.SetupLoopbackClientServer(gameSubs, gameServer)

	if err := gameHost.Init(&host.InitParams{
		BaseDir:    basedir,
		UserDir:    "",
		Args:       []string{},
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
}

func (gameCallbacks) ProcessConsoleCommands() {}

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
	mapArg := ""
	for i := 0; i < len(args); i++ {
		if args[i] == "+map" && i+1 < len(args) {
			mapArg = args[i+1]
			i++
		}
	}

	// Try to initialize with renderer, fall back to headless if it fails
	headless := *headlessFlag
	initErr := initSubsystems(headless, *baseDir, *gameDir)
	if initErr != nil && !headless {
		// Check if error is related to renderer initialization
		if isRendererError(initErr) {
			fmt.Println("WARNING: Renderer initialization failed. Running in headless mode.")
			fmt.Printf("Error: %v\n", initErr)
			fmt.Println("Continuing with game loop (no rendering)...")
			headless = true
			// Re-initialize without renderer
			if err := initSubsystems(true, *baseDir, *gameDir); err != nil {
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
			gameHost.Frame(dt, cb)
		})
		gameRenderer.OnDraw(func(dc renderer.RenderContext) {
			// Frame pipeline: Clear → World → Entities → Particles → 2D Overlay

			// Phase 1: Clear screen to black
			dc.Clear(0, 0, 0, 1)

			// Phase 2-4: World/Entities/Particles (stubs until M4.3/M4.4)
			// These will be implemented in later milestones

			// Phase 5: 2D Overlay (menu, console, HUD)
			if gameMenu != nil && gameMenu.IsActive() {
				gameMenu.M_Draw(dc)
			} else if gameHUD != nil {
				w, h := gameRenderer.Size()
				gameHUD.SetScreenSize(w, h)
				updateHUDFromServer()
				gameHUD.Draw(dc)
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
