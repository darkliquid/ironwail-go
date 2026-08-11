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

	// The no-assets wasm boot has no `go` binary or disk: feed the QC VM the
	// build-time embedded progs.dat (see gen_wasm_progs + progs_data.go).
	game.WasmEmbeddedProgsData = embeddedProgsData
	server.EmbeddedProgsData = embeddedProgsData

	// InitSubsystems(true /*headless*/, ...). The wasm deploy has no Quake
	// data in the browser; basedir "/" + gamedir "id1" keeps the registration
	// check on the shareware path (gamedir != "id1" would demand a registered
	// version), and the synthetic no-assets map provides the world.
	if err := g.InitSubsystems(true, false, 4, "/", "id1", nil); err != nil {
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
