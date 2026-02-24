package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/cmdsys"
	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/host"
	"github.com/ironwail/ironwail-go/pkg/types"
	"github.com/ironwail/ironwail-go/internal/renderer"
	"github.com/ironwail/ironwail-go/pkg/types"
)

const (
	VersionMajor = 0
	VersionMinor = 2
	VersionPatch = 0
)

var (
	gameHost   *host.Host
	gameServer *server.Server
	gameQC    *qc.VM
	gameRenderer *renderer.Renderer
)

func initGameHost() error {
	// Initialize console and command system
	console.InitGlobal(0)
	cmdsys.Init()

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

func initSubsystems() error {
	if err := initGameHost(); err != nil {
		return err
	}
	if err := initGameServer(); err != nil {
		return err
	}
	if err := initGameQC(); err != nil {
		return err
	}
	if err := initGameRenderer(); err != nil {
		return err
	}

	// Wire subsystems together through Host.Init
	subs := &host.Subsystems{
		Files:    nil, // No filesystem in this demo
		Commands: cmdsys.GlobalBuffer(),
		Console:  console.Global(),
		Server:   gameServer,
		Client:   nil, // No client in server mode
		Audio:    nil, // No audio in this demo
		Renderer: gameRenderer,
	}

	if err := gameHost.Init(&host.InitParams{
		BaseDir:    "",
		UserDir:    "",
		Args:       []string{},
		MaxClients: 1,
	}, subs); err != nil {
		return fmt.Errorf("failed to initialize host: %w", err)
	}

	slog.Info("All subsystems initialized")
	return nil
}

func main() {
	fmt.Printf("Ironwail-Go v%d.%d.%d\n", VersionMajor, VersionMinor, VersionPatch)
	fmt.Println("A Go port of the Ironwail Quake engine")
	fmt.Println()

	// Initialize all subsystems
	if err := initSubsystems(); err != nil {
		log.Fatal("Initialization failed:", err)
	}

	// Set up frame callbacks
	gameHost.OnDraw(func(dc *renderer.DrawContext) {
		// Dark blue-gray background (Quake-style)
		dc.Clear(gmath.Color{R: 0.1, G: 0.1, B: 0.2, A: 1.0})
	})

	gameHost.OnUpdate(func(dt float64) {
		// Game logic update will be called here
	})

	gameHost.OnClose(func() {
		slog.Info("Shutting down...")
	})

	// Start the game loop
	if err := gameHost.FrameLoop(); err != nil {
		log.Fatal("Game loop error:", err)
	}

	slog.Info("Engine shutdown complete")
}
