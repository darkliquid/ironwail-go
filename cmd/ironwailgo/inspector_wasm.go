//go:build js && wasm

// inspector_wasm.go — plan 22 Phase B. Exposes a read-side inspector bridge
// to the browser: window.ironwailInspector.getState(layer), getTimeline(),
// getEdict(n), getSourceAnchor(layer). It reads ONLY state the engine already
// surfaces (console, host timing, server edict table, client state, camera).
// The same data powers the walkthrough web UI (web/walkthrough/).
package main

import (
	"encoding/json"
	"strconv"
	"syscall/js"

	"github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/game"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

// inspectorLayers is the ordered set of engine layers the walkthrough tours.
// The order matches the left rail in web/walkthrough/index.html and the
// source anchors in web/walkthrough/anchors.json.
var inspectorLayers = []string{
	"boot", "console", "host", "server", "quakec", "client", "renderer",
}

// toJSValue converts any Go value into standard shapes syscall/js.ValueOf
// accepts (map[string]any, []any, float64, string, bool, nil) via JSON round-trip.
// This prevents reflection panics or timeouts when returning structs, typed maps,
// or custom slices across the JS bridge.
func toJSValue(v any) any {
	if v == nil {
		return js.Null()
	}
	data, err := json.Marshal(v)
	if err != nil {
		return js.Null()
	}
	return js.Global().Get("JSON").Call("parse", string(data))
}

// installInspector registers window.ironwailInspector. js.Func values must be
// attached to a JS object via Set (with .Release on each after assignment is
// not required here because the funcs live for the program's lifetime);
// embedding them in a Go map and passing that to Set panics with
// "ValueOf: invalid value".
func installInspector(g *game.Game) {
	obj := js.Global().Get("Object").New()
	obj.Set("getState", js.FuncOf(func(this js.Value, args []js.Value) any {
		return toJSValue(inspectorGetState(g, args))
	}))
	obj.Set("getStateJSON", js.FuncOf(func(this js.Value, args []js.Value) any {
		// Deterministic payload: encoding/json sorts map keys, so the string
		// is byte-stable for identical state — the page's change-detection
		// cache keys on it. (js object conversion would reorder Go map keys
		// randomly every call, breaking the paused-frame stability.)
		data, err := json.Marshal(inspectorGetState(g, args))
		if err != nil {
			return ""
		}
		return string(data)
	}))
	obj.Set("getTimeline", js.FuncOf(func(this js.Value, args []js.Value) any {
		return toJSValue(inspectorGetTimeline(g))
	}))
	obj.Set("getEdict", js.FuncOf(func(this js.Value, args []js.Value) any {
		return toJSValue(inspectorGetEdict(g, args))
	}))
	obj.Set("getSourceAnchor", js.FuncOf(func(this js.Value, args []js.Value) any {
		return toJSValue(inspectorGetSourceAnchor(g, args))
	}))
	obj.Set("getLayers", js.FuncOf(func(this js.Value, args []js.Value) any {
		layers := js.Global().Get("Array").New(len(inspectorLayers))
		for i, l := range inspectorLayers {
			layers.SetIndex(i, l)
		}
		return layers
	}))
	obj.Set("setPaused", js.FuncOf(func(this js.Value, args []js.Value) any {
		paused := false
		if len(args) > 0 && args[0].Type() == js.TypeBoolean {
			paused = args[0].Bool()
		}
		g.WasmSetPaused(paused)
		return nil
	}))
	obj.Set("getPaused", js.FuncOf(func(this js.Value, args []js.Value) any {
		return g.WasmPaused()
	}))
	obj.Set("stepFrames", js.FuncOf(func(this js.Value, args []js.Value) any {
		n := 1
		if len(args) > 0 && args[0].Type() == js.TypeNumber {
			n = args[0].Int()
		}
		g.WasmStepFrames(n)
		return nil
	}))
	obj.Set("getGoroutines", js.FuncOf(func(this js.Value, args []js.Value) any {
		return toJSValue(inspectorGetGoroutines())
	}))
	obj.Set("getTelemetryLog", js.FuncOf(func(this js.Value, args []js.Value) any {
		return toJSValue(inspectorGetTelemetryLog(g))
	}))
	obj.Set("getGpuStatus", js.FuncOf(func(this js.Value, args []js.Value) any {
		return toJSValue(inspectorGetGpuStatus(g))
	}))
	obj.Set("getPassToggles", js.FuncOf(func(this js.Value, args []js.Value) any {
		return toJSValue(renderer.GetPassTogglesMap())
	}))
	obj.Set("setPassToggle", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) >= 2 && args[0].Type() == js.TypeString && args[1].Type() == js.TypeBoolean {
			name := args[0].String()
			enabled := args[1].Bool()
			renderer.SetPassToggleByName(name, enabled)
		}
		return nil
	}))
	obj.Set("setPassToggles", js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 && args[0].Type() == js.TypeObject {
			val := args[0]
			for name := range renderer.PassNameToFlag {
				prop := val.Get(name)
				if !prop.IsUndefined() && !prop.IsNull() && prop.Type() == js.TypeBoolean {
					renderer.SetPassToggleByName(name, prop.Bool())
				}
			}
		}
		return nil
	}))
	js.Global().Set("ironwailInspector", obj)
}

func inspectorGetGoroutines() any {
	return game.WasmGoroutineSnapshot()
}

func inspectorGetTelemetryLog(g *game.Game) any {
	if g == nil {
		return []map[string]any{}
	}
	return g.GetWasmTelemetryLog()
}

func inspectorGetGpuStatus(g *game.Game) any {
	out := map[string]any{
		"navigatorGpu":   false,
		"rendererActive": false,
	}
	if doc := js.Global().Get("document"); !doc.IsUndefined() && !doc.IsNull() {
		nav := js.Global().Get("navigator")
		gpu := nav.Get("gpu")
		out["navigatorGpu"] = gpu.Truthy()
	}
	if g != nil && g.Renderer != nil {
		out["rendererActive"] = true
	}
	return out
}

// inspectorGetState returns a JSON-friendly snapshot for a single layer.
func inspectorGetState(g *game.Game, args []js.Value) any {
	layer := "host"
	if len(args) > 0 && args[0].Type() == js.TypeString {
		layer = args[0].String()
	}
	switch layer {
	case "boot":
		return inspectorBootState(g)
	case "console":
		return inspectorConsoleState(g)
	case "host":
		return inspectorHostState(g)
	case "server":
		return inspectorServerState(g)
	case "quakec":
		return inspectorQCState(g)
	case "client":
		return inspectorClientState(g)
	case "renderer":
		return inspectorRendererState(g)
	default:
		return map[string]any{"error": "unknown layer: " + layer}
	}
}

func inspectorBootState(g *game.Game) any {
	out := map[string]any{}
	if g.Subs != nil {
		fs, _ := g.Subs.Files.(interface{ GameDir() string })
		if fs == nil {
			out["gamedir"] = "?"
		} else {
			out["gamedir"] = fs.GameDir()
		}
		out["hasFiles"] = g.Subs.Files != nil
	}
	if g.Server != nil {
		out["map"] = g.Server.Name
		out["serverActive"] = g.Server.Active
	}
	return out
}

func inspectorConsoleState(g *game.Game) any {
	out := map[string]any{"lines": []string{}}
	con := console.Global()
	if con == nil {
		return out
	}
	lines := make([]string, 0, 40)
	total := con.TotalLines()
	start := total - 40
	if start < 0 {
		start = 0
	}
	for i := start; i < total; i++ {
		lines = append(lines, con.Line(i))
	}
	out["lines"] = lines
	return out
}

func inspectorHostState(g *game.Game) any {
	out := map[string]any{}
	if g.Host != nil {
		out["frameCount"] = g.Host.FrameCount()
		out["frameTime"] = g.Host.FrameTime()
		out["simFrameTime"] = g.Host.SimFrameTime()
		out["signons"] = g.Host.SignOns()
		out["aborted"] = g.Host.IsAborted()
	}
	if g.Client != nil {
		out["clientState"] = clientStateName(g.Client.State)
	}
	return out
}

func inspectorServerState(g *game.Game) any {
	out := map[string]any{"edicts": []map[string]any{}}
	if g.Server == nil || !g.Server.Active {
		out["active"] = false
		return out
	}
	out["active"] = true
	out["map"] = g.Server.Name
	out["time"] = g.Server.Time
	out["numEdicts"] = g.Server.NumEdicts

	// Edict table summary (classname + origin for non-free edicts).
	edicts := make([]map[string]any, 0, 16)
	for i := 0; i < g.Server.NumEdicts && i < 64; i++ {
		ent := g.Server.EdictNum(i)
		if ent == nil || ent.Free {
			continue
		}
		entry := map[string]any{
			"num": i,
		}
		if cn := ent.ClassNameString(g.Server); cn != "" {
			entry["classname"] = cn
		}
		org := ent.Origin(g.Server)
		entry["origin"] = []float32{org.X, org.Y, org.Z}
		edicts = append(edicts, entry)
	}
	out["edicts"] = edicts
	return out
}

func inspectorClientState(g *game.Game) any {
	out := map[string]any{}
	if g.Client == nil {
		out["state"] = "no client"
		return out
	}
	out["state"] = clientStateName(g.Client.State)
	out["time"] = g.Client.Time
	out["entities"] = len(g.Client.Entities)
	out["stats"] = g.Client.Stats[:]
	if g.Host != nil {
		out["signons"] = g.Host.SignOns()
	}
	return out
}

// inspectorRendererState reports the renderer layer surface exposed by the
// engine: camera origin/angles from the game frame, plus the narrow renderer
// capability flags. Deep GPU pass counters are not exposed through the game
// Renderer interface (plan 27 O6); the walkthrough covers the renderer layer
// top-down via source anchors instead.
func inspectorRendererState(g *game.Game) any {
	out := map[string]any{}
	origin, angles := g.WasmViewState()
	out["cameraOrigin"] = []float32{origin[0], origin[1], origin[2]}
	out["cameraAngles"] = []float32{angles[0], angles[1], angles[2]}
	out["backend"] = "gogpu/WebGPU"
	if doc := js.Global().Get("document"); !doc.IsUndefined() && !doc.IsNull() {
		canvas := doc.Call("querySelector", "canvas")
		out["canvasPresent"] = !canvas.IsNull() && !canvas.IsUndefined()
		nav := js.Global().Get("navigator")
		gpu := nav.Get("gpu")
		out["webgpu"] = gpu.Truthy()
	}
	if g.Server != nil && g.Server.WorldTree != nil {
		out["worldTree"] = "loaded"
		out["worldFaces"] = len(g.Server.WorldTree.Faces)
	}
	if g.Renderer != nil {
		out["rendererActive"] = true
	}
	// Live per-frame pass activity (monotonic counters; the UI diffs across
	// frames to show what the renderer actually executed).
	stats := renderer.WasmFramePassStatsSnapshot()
	out["passStats"] = map[string]any{
		"worldDraws":      stats.WorldDraws,
		"overlayDraws":    stats.OverlayDraws,
		"worldUploads":    stats.WorldUploads,
		"sceneDraws":      stats.SceneDraws,
		"particlesDrawn":  stats.ParticlesDrawn,
		"aliasDraws":      stats.AliasDraws,
		"spriteDraws":     stats.SpriteDraws,
		"lightmapUploads": stats.LightmapUploads,
	}
	out["passToggles"] = renderer.GetPassTogglesMap()
	return out
}

// inspectorQCState surfaces the QuakeC layer: core QC globals, the recent
// function-call trace ring, and per-function call counts. Reads the server's
// retained QC observer (internal/server/qc_trace_record.go), which is filled
// for every VM call regardless of the sv_debug_qc_trace telemetry cvar.
func inspectorQCState(g *game.Game) any {
	out := map[string]any{}
	if g.Server == nil || g.Server.QCVM == nil {
		out["active"] = false
		return out
	}
	vm := g.Server.QCVM
	out["active"] = true

	globals := map[string]any{}
	if o := vm.FindGlobal("time"); o >= 0 {
		globals["time"] = vm.GFloat(o)
	}
	if o := vm.FindGlobal("self"); o >= 0 {
		globals["self"] = vm.GInt(o)
	}
	if o := vm.FindGlobal("world"); o >= 0 {
		globals["world"] = vm.GInt(o)
	}
	if o := vm.FindGlobal("mapname"); o >= 0 {
		globals["mapname"] = vm.GString(o)
	}
	out["globals"] = globals

	events, counts := g.Server.QCTraceSnapshot()
	trace := make([]map[string]any, 0, len(events))
	for _, e := range events {
		trace = append(trace, map[string]any{
			"phase": e.Phase,
			"fn":    e.Function,
			"depth": e.Depth,
			"self":  e.Self,
		})
	}
	out["trace"] = trace
	out["callCounts"] = counts
	return out
}

// inspectorGetTimeline returns host/server timing bars recycled from
// host_speeds measurement (srvTime + frame counters). The physics phase bars
// live in stepframe.go behind the host_speeds cvar; here we expose the frame
// totals the walkthrough's timeline panel renders.
func inspectorGetTimeline(g *game.Game) any {
	out := map[string]any{}
	if g.Host != nil {
		out["frameCount"] = g.Host.FrameCount()
		out["frameTimeMs"] = g.Host.FrameTime() * 1000
	}
	if g.Server != nil {
		out["srvTime"] = g.Server.Time
	}
	return out
}

// inspectorGetEdict returns a typed field dump for one edict.
func inspectorGetEdict(g *game.Game, args []js.Value) any {
	n := 0
	if len(args) > 0 && args[0].Type() == js.TypeNumber {
		n = args[0].Int()
	}
	if g.Server == nil || g.Server.EdictNum(n) == nil {
		return map[string]any{"error": "no edict " + strconv.Itoa(n)}
	}
	ent := g.Server.EdictNum(n)
	out := map[string]any{"num": n, "free": ent.Free}
	if cn := ent.ClassNameString(g.Server); cn != "" {
		out["classname"] = cn
	}
	org := ent.Origin(g.Server)
	out["origin"] = []float32{org.X, org.Y, org.Z}
	out["health"] = ent.Health(g.Server)
	mb := ent.MoveType(g.Server)
	out["movetype"] = mb
	sol := ent.Solid(g.Server)
	out["solid"] = sol
	return out
}

// inspectorGetSourceAnchor returns {file, line, docRef} from the static
// anchor table shipped in web/walkthrough/anchors.json (embedded here so the
// wasm inspector is self-contained; the web app reads the same file).
func inspectorGetSourceAnchor(g *game.Game, args []js.Value) any {
	layer := "host"
	if len(args) > 0 && args[0].Type() == js.TypeString {
		layer = args[0].String()
	}
	if a, ok := inspectorAnchors[layer]; ok {
		return a
	}
	return map[string]any{"error": "no anchor for layer: " + layer}
}

// inspectorAnchors mirrors web/walkthrough/anchors.json. Keep in sync.
var inspectorAnchors = map[string]any{
	"boot":     map[string]any{"file": "cmd/ironwailgo/main.go", "doc": "docs/LEARNING_GUIDE.md"},
	"console":  map[string]any{"file": "internal/host/commands.go", "doc": "docs/WALKTHROUGH_BOOT_TO_MENU.md"},
	"host":     map[string]any{"file": "internal/game/game_loop.go", "line": 492, "doc": "docs/WALKTHROUGH_SINGLEPLAYER_FORWARD.md"},
	"server":   map[string]any{"file": "internal/server/physics/stepframe.go", "doc": "docs/WALKTHROUGH_SINGLEPLAYER_FORWARD.md"},
	"quakec":   map[string]any{"file": "internal/qc/exec.go", "doc": "docs/QGO_QUAKEGO_GUIDE.md"},
	"client":   map[string]any{"file": "internal/client/parse_clientdata.go", "doc": "docs/LEARNING_GUIDE.md"},
	"renderer": map[string]any{"file": "internal/renderer/renderer_gogpu_frame.go", "doc": "docs/RENDERER_LEARNING_PLAN.md"},
}

func clientStateName(s client.ClientState) string {
	switch s {
	case client.StateDisconnected:
		return "disconnected"
	case client.StateConnected:
		return "connected"
	case client.StateActive:
		return "active"
	default:
		return strconv.Itoa(int(s))
	}
}
