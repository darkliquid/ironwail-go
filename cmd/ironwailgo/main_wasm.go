//go:build js && wasm

package main

import (
	"fmt"
	"log"
	"log/slog"
	"syscall/js"

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

	// Non-headless so the WebGPU renderer constructs and binds the <canvas>
	// (gogpu browser platform). basedir "/" + gamedir "id1" keeps the
	// registration check on the shareware path; the synthetic map provides
	// the world without Quake data.
	if err := g.InitSubsystems(false, false, 4, "/", "id1", nil); err != nil {
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

	// WebGPU available → run the real renderer loop on the canvas. Absent
	// (older/unsupported browser) → degrade to the headless inspector loop so
	// the data panels still work; the walkthrough must not die.
	navGpu := js.Global().Get("navigator").Get("gpu")
	if navGpu.IsUndefined() || navGpu.IsNull() {
		slog.Warn("navigator.gpu unavailable — running headless walkthrough (no WebGPU viewport)")
		g.RunWasmHeadlessLoop()
		return
	}

	startup := game.StartupOptions{
		BaseDir:    "/",
		GameDir:    "id1",
		Listen:     true,
		MaxClients: 4,
	}
	runRendererSafe(g, startup)
}

// runRendererSafe runs the WebGPU renderer loop, degrading to the headless
// inspector loop on any failure — including the renderer's internal panics
// (WebGPU strict-validation issues surface as panics, not errors, e.g. missing
// depth32float-stencil8 feature, WGSL uniformity rules, or the 4-bind-group
// layout limit). The walkthrough must stay usable as a data tour even when the
// renderer is not browser-ready.
func runRendererSafe(g *game.Game, startup game.StartupOptions) {
	defer func() {
		if r := recover(); r != nil {
			slog.Warn("renderer panic — falling back to headless walkthrough", "panic", fmt.Sprint(r))
			g.RunWasmHeadlessLoop()
		}
	}()
	if _, err := g.RunRuntimeRendererLoop(startup, ""); err != nil {
		slog.Warn("renderer loop failed — falling back to headless walkthrough", "err", err)
		g.RunWasmHeadlessLoop()
	}
}
