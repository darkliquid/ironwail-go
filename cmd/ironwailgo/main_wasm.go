package main

import (
	"fmt"
	"io"
	"log"
	"log/slog"
	"net/http"
	"runtime/debug"
	"syscall/js"

	"github.com/darkliquid/ironwail-go/internal/fs"
	"github.com/darkliquid/ironwail-go/internal/game"
	"github.com/darkliquid/ironwail-go/internal/gameconfig"
)

const (
	VersionMajor = 0
	VersionMinor = 2
	VersionPatch = 0
)

var g *game.Game

func fetchWasmPak0() ([]byte, error) {
	urls := []string{
		"/testdata/id1/pak0.pak",
		"/id1/pak0.pak",
		"id1/pak0.pak",
	}
	for _, u := range urls {
		resp, err := http.Get(u)
		if err != nil {
			continue
		}
		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			continue
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err == nil && len(data) > 0 {
			slog.Info("successfully fetched pak0.pak over HTTP", "url", u, "bytes", len(data))
			return data, nil
		}
	}
	return nil, fmt.Errorf("could not fetch pak0.pak from any URL")
}

func main() {
	fmt.Printf("Ironwail-Go WASM v%d.%d.%d\n", VersionMajor, VersionMinor, VersionPatch)
	fmt.Println("A Go WebAssembly port of Ironwail Quake engine")

	g = game.New()

	// Fetch testing pak0.pak over HTTP before initializing subsystems. This
	// lets the browser boot with real map data (the walkthrough's viewport
	// renders Quake's start map instead of a plain synthetic room).
	var mountedPak *fs.Pack
	if pakData, err := fetchWasmPak0(); err == nil {
		if pack, err := fs.LoadPackFromBytes("pak0.pak", pakData); err == nil {
			mountedPak = pack
		} else {
			slog.Warn("failed to parse fetched pak0.pak", "err", err)
		}
	} else {
		slog.Warn("failed to fetch pak0.pak over HTTP", "err", err)
	}

	// Non-headless so the WebGPU renderer constructs and binds the <canvas>
	// (gogpu browser platform). basedir "/" + gamedir "id1" keeps the
	// registration check on the shareware path; testing pak0 supplies map data.
	if err := g.InitSubsystems(false, false, 4, "/", gameconfig.Default().BaseGameDir, nil, mountedPak); err != nil {
		log.Fatalf("WASM initialization failed: %v", err)
	}

	// Plan 22 Phase B: expose the read-side inspector bridge to the browser
	// (window.ironwailInspector). The walkthrough UI consumes it.
	installInspector(g)

	// Browser input: ensure a DOM input backend is present so keyboard/mouse
	// move the player in the walkthrough (the renderer's gogpu adapter may be
	// absent when WebGPU is unavailable).
	g.InstallWasmDOMInput()

	// Auto-start the testing pak0 start map if present.
	if g.Subs != nil && g.Subs.Files != nil && g.Subs.Files.FileExists("maps/start.bsp") {
		slog.Info("auto-starting start map from testing pak0 (wasm)")
		if err := g.Host.CmdMap("start", g.Subs); err != nil {
			log.Printf("Failed to spawn start map: %v", err)
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
		GameDir:    gameconfig.Default().BaseGameDir,
		Listen:     true,
		MaxClients: 4,
	}
	runRendererSafe(g, startup)

	// Keep the Go WebAssembly runtime alive so browser requestAnimationFrame
	// callbacks (OnUpdate / OnDraw) continue executing. Returning from main()
	// terminates the Go WASM instance and freezes frame execution at frame 0.
	select {}
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
			debug.PrintStack()
			g.RunWasmHeadlessLoop()
		}
	}()
	if _, err := g.RunRuntimeRendererLoop(startup, ""); err != nil {
		slog.Warn("renderer loop failed — falling back to headless walkthrough", "err", err)
		g.RunWasmHeadlessLoop()
	}
}
