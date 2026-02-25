package main

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/ironwail/ironwail-go/internal/console"
	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/host"
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
		Files:  nil, // No filesystem in this demo
		Client: nil, // No client in server mode
		Audio:  nil, // No audio in this demo
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

	// Set up renderer callbacks
	gameRenderer.OnUpdate(func(dt float64) {
		gameHost.Frame(dt, nil)
	})
	gameRenderer.OnDraw(func(dc *renderer.DrawContext) {
		// empty for now
	})

	// Start the main loop (blocking)
	if err := gameRenderer.Run(); err != nil {
		log.Fatal("Render loop failed:", err)
	}
	// Cleanup
	gameRenderer.Shutdown()

	slog.Info("Engine shutdown complete")
}
