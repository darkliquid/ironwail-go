package main

import (
	"bytes"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/draw"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/host"
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
	slog.Info("QC loaded")

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
	slog.Info("FS mounted")

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

	// Wire subsystems together through Host.Init
	gameSubs = &host.Subsystems{
		Files:  fileSys,
		Server: gameServer,
		Client: nil, // No client in server mode
		Audio:  nil, // No audio in this demo
	}
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

	// Initialize draw manager from data directory (for testing/development)
	dataDir := "data"
	if err := gameDraw.InitFromDir(dataDir); err != nil {
		slog.Warn("Failed to initialize draw manager", "error", err)
	}

	// Make sure the menu is visible at startup
	gameMenu.ShowMenu()
	slog.Info("menu active")

	slog.Info("All subsystems initialized")
	return nil
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
	flag.Parse()

	// Check if a map argument was provided
	args := flag.Args()
	mapArg := ""
	if len(args) > 0 {
		mapArg = args[0]
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

	// Execute map command if map argument was provided
	if mapArg != "" {
		slog.Info("map spawn started", "map", mapArg)
		if err := gameHost.CmdMap(mapArg, gameSubs); err != nil {
			log.Printf("Failed to spawn map %s: %v", mapArg, err)
		} else {
			slog.Info("map spawn finished", "map", mapArg)
		}
	}

	if !headless {
		// Set up renderer callbacks
		gameRenderer.OnUpdate(func(dt float64) {
			gameHost.Frame(dt, nil)
		})
		gameRenderer.OnDraw(func(dc renderer.RenderContext) {
			// Draw menu if active
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

func headlessGameLoop() {
	slog.Info("Starting headless game loop")

	// Simple game loop without rendering
	slog.Info("frame loop started")
	lastTime := time.Now()
	ticker := time.NewTicker(time.Second / 250) // 250 FPS target
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			now := time.Now()
			dt := now.Sub(lastTime).Seconds()
			lastTime = now

			// Update game state
			gameHost.Frame(dt, nil)
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
