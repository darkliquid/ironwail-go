// Command ironwailgo is the Ironwail-Go Quake engine executable. It wires
// together the client, server, renderer, and host subsystems and serves as
// the main entry point for launching the game.
package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strings"

	cl "github.com/darkliquid/ironwail-go/internal/client"

	"github.com/darkliquid/ironwail-go/internal/game"
)

const (
	VersionMajor = 0
	VersionMinor = 2
	VersionPatch = 0
)

var (
	startupVidWidth  = 1280
	startupVidHeight = 720
)

// g is the game instance used throughout the application
var g *game.Game

func main() {
	// Logger initialization is handled in logger_*.go files based on build tags
	fmt.Printf("Ironwail-Go v%d.%d.%d\n", VersionMajor, VersionMinor, VersionPatch)
	fmt.Println("A Go port of Ironwail Quake engine")
	fmt.Println()

	// Initialize the game instance
	g = game.New()

	startupOpts, err := parseStartupOptions(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}
	g.Host.Net.SetHostPort(startupOpts.Port)

	headlessFlag := flag.Bool("headless", false, "Run without rendering")
	screenshotFlag := flag.String("screenshot", "", "Save screenshot to PNG file and exit")
	widthFlag := flag.Int("width", startupVidWidth, "Initial window width")
	heightFlag := flag.Int("height", startupVidHeight, "Initial window height")
	windowFlag := flag.Bool("window", false, "Run in windowed mode")
	logLevel := flag.String("loglvl", "INFO", "logging level spec (DEBUG or INFO,renderer=WARN,input=DEBUG)")
	pprofAddr := flag.String("pprof", "", "pprof listener address (e.g., localhost:6060)")
	if err := flag.CommandLine.Parse(startupOpts.Args); err != nil {
		log.Fatal(err)
	}
	hasWidthFlag, hasHeightFlag := false, false
	flag.CommandLine.Visit(func(f *flag.Flag) {
		switch f.Name {
		case "width":
			hasWidthFlag = true
		case "height":
			hasHeightFlag = true
		}
	})
	if *widthFlag > 0 {
		startupVidWidth = *widthFlag
	}
	if *heightFlag > 0 {
		startupVidHeight = *heightFlag
	}
	game.SetStartupVideoOverrides(startupVidWidth, startupVidHeight, hasWidthFlag, hasHeightFlag, *windowFlag)

	if err := installLogging(*logLevel); err != nil {
		log.Fatal(err)
	}

	if addr := strings.TrimSpace(*pprofAddr); addr == "" {
		addr = strings.TrimSpace(os.Getenv("IRONWAILGO_PPROF"))
		*pprofAddr = addr
	}
	if addr := strings.TrimSpace(*pprofAddr); addr != "" {
		go func(addr string) {
			slog.Info("pprof listener starting", "addr", addr, "hint", "curl http://"+addr+"/debug/pprof/goroutine?debug=2 while hung")
			srv := &http.Server{Addr: addr}
			if err := srv.ListenAndServe(); err != nil {
				slog.Warn("pprof listener exited", "err", err)
			}
		}(addr)
	}

	args := flag.Args()
	mapArg := startupMapArg(args)
	explicitPlusMap := hasPlusMapArg(args)
	if startupOpts.Dedicated && mapArg == "" {
		mapArg = "start"
	}

	dedicated := startupOpts.Dedicated
	headless := *headlessFlag || dedicated
	initErr := g.InitSubsystems(headless, dedicated, startupOpts.MaxClients, startupOpts.BaseDir, startupOpts.GameDir, args)
	if initErr != nil && !headless {
		if isRendererError(initErr) {
			fmt.Println("WARNING: Renderer initialization failed. Running in headless mode.")
			fmt.Printf("Error: %v\n", initErr)
			fmt.Println("Continuing with game loop (no rendering)...")
			headless = true
			if err := g.InitSubsystems(true, false, startupOpts.MaxClients, startupOpts.BaseDir, startupOpts.GameDir, args); err != nil {
				log.Fatal("Initialization failed:", err)
			}
		} else {
			log.Fatal("Initialization failed:", initErr)
		}
	}
	defer shutdownEngine()

	slog.Info("FS mounted")
	slog.Info("QC loaded")
	if !dedicated {
		slog.Info("menu active")
	}

	// C Ironwail executes +map from command-line via stuffcmds inside quake.rc.
	// Avoid manually spawning in that case or startup map logic will run twice.
	if !explicitPlusMap {
		runStartupMap(mapArg)
	}

	screenshotPath := strings.TrimSpace(*screenshotFlag)
	screenshotMode := screenshotPath != ""
	gameStartupOpts := game.StartupOptions{
		BaseDir:    startupOpts.BaseDir,
		GameDir:    startupOpts.GameDir,
		Dedicated:  startupOpts.Dedicated,
		Listen:     startupOpts.Listen,
		MaxClients: startupOpts.MaxClients,
		Port:       startupOpts.Port,
		Args:       startupOpts.Args,
	}

	if !headless {
		result, err := g.RunRuntimeRendererLoop(gameStartupOpts, screenshotPath)
		if err != nil {
			log.Fatal(err)
		}
		if result.ScreenshotCaptured {
			if result.ScreenshotErr != nil {
				log.Fatal("Screenshot failed:", result.ScreenshotErr)
			}
			return
		}
		if result.HandledFallback {
			return
		}
	}

	if screenshotMode {
		if err := g.CaptureScreenshot(screenshotPath, startupOpts.BaseDir, startupOpts.GameDir); err != nil {
			log.Fatal("Screenshot failed:", err)
		}
		return
	}

	if headless {
		if dedicated {
			g.DedicatedGameLoop()
		} else {
			g.HeadlessGameLoop()
		}
	}
}

func isRendererError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "renderer") ||
		strings.Contains(errStr, "wayland") ||
		strings.Contains(errStr, "configure") ||
		strings.Contains(errStr, "display") ||
		strings.Contains(errStr, "window") ||
		strings.Contains(errStr, "surface") ||
		strings.Contains(errStr, "segv")
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

func hasPlusMapArg(args []string) bool {
	for i := 0; i < len(args); i++ {
		if args[i] == "+map" && i+1 < len(args) {
			return true
		}
	}
	return false
}

func runStartupMap(mapArg string) {
	if mapArg == "" {
		return
	}

	slog.Info("map spawn started", "map", mapArg)
	if err := g.Host.CmdMap(mapArg, g.Subs); err != nil {
		log.Printf("Failed to spawn map %s: %v", mapArg, err)
		return
	}

	slog.Info("map spawn finished", "map", mapArg)
	if g.Client != nil && g.Client.State == cl.StateActive && g.Host.SignOns() == 4 {
		g.ApplyStartupGameplayInputMode()
		slog.Info("client active", "map", mapArg)

		// Dismiss startup menu so game can unpause and run queued frames
		g.Host.Cmd.AddText("togglemenu\n")
	}
}
