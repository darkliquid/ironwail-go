//go:build js && wasm

package main

import (
	"fmt"
	"log"
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/game"
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

	slog.Info("Ironwail-Go WASM initialized successfully")

	// Keep the Go WASM event loop running
	select {}
}
