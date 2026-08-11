//go:build js && wasm

package main

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/game"
	"github.com/darkliquid/ironwail-go/internal/server"
)

const (
	VersionMajor = 0
	VersionMinor = 2
	VersionPatch = 0
)

var g *game.Game

func main() {
	fmt.Printf("Ironwail-Go WASM v%d.%d.%d\n", VersionMajor, VersionMinor, VersionPatch)
	fmt.Println("A Go WebAssembly port of Ironwail Quake engine")

	g = game.New()

	if err := g.InitSubsystems(true, false, 4, "/", "/", nil); err != nil {
		log.Fatalf("WASM initialization failed: %v", err)
	}

	// Plan 22 Phase B: expose the read-side inspector bridge to the browser
	// (window.ironwailInspector). The walkthrough UI consumes it.
	installInspector(g)

	// Auto-start the synthetic no-assets map so the walkthrough has a live
	// world to inspect even with no Quake data (mirrors runStartupMap).
	if g.Subs != nil && g.Subs.Files != nil && !g.Subs.Files.FileExists("maps/start.bsp") {
		slog.Info("no map assets found — auto-starting synthetic demo room (wasm)")
		if err := g.Host.CmdMap(server.SyntheticMapName, g.Subs); err != nil {
			log.Printf("Failed to spawn synthetic map: %v", err)
		}
	}

	slog.Info("Ironwail-Go WASM initialized successfully")

	// Drive host frames so the inspector's timing/edict/camera data is live.
	g.RunWasmInspectorLoop()
}
