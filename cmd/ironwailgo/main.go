package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"os"
	"strings"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	inet "github.com/darkliquid/ironwail-go/internal/net"

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
	inet.SetHostPort(startupOpts.Port)

	headlessFlag := flag.Bool("headless", false, "Run without rendering")
	screenshotFlag := flag.String("screenshot", "", "Save screenshot to PNG file and exit")
	widthFlag := flag.Int("width", startupVidWidth, "Initial window width")
	heightFlag := flag.Int("height", startupVidHeight, "Initial window height")
	logLevel := flag.String("loglvl", "INFO", "logging level spec (DEBUG or INFO,renderer=WARN,input=DEBUG)")
	if err := flag.CommandLine.Parse(startupOpts.Args); err != nil {
		log.Fatal(err)
	}
	if *widthFlag > 0 {
		startupVidWidth = *widthFlag
	}
	if *heightFlag > 0 {
		startupVidHeight = *heightFlag
	}

	if err := installLogging(*logLevel); err != nil {
		log.Fatal(err)
	}

	args := flag.Args()
	mapArg := startupMapArg(args)
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

	runStartupMap(mapArg)

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
	}
}
