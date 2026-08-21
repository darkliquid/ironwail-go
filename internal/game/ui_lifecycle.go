package game

import (
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/darkliquid/ironwail-go/internal/quakeui"
)

// initializeUIPath selects the UI render path ONCE at startup (G11,
// ADR-0012 A1). The ui_backend cvar is read a single time and the decision is
// frozen for the whole session — no mid-session flip. The path is forced to
// legacy when a gogpu provider is unavailable (headless never reaches the
// runtime renderer loop; software/init failure fails open to legacy, AC4).
//
// When the gogpu/ui path is selected, the quakeui overlay (and its uiApp,
// created inside NewOverlayRenderer) is built immediately so the widget tree
// exists before the first frame; teardown is wired to gogpuApp.OnClose.
func (g *Game) initializeUIPath() {
	if g == nil || g.uiBackendFrozen {
		return
	}
	g.uiBackendFrozen = true

	selected := false
	if g.Host != nil && quakeui.IsGogpuUIPath(g.Host.CVar) {
		selected = true
	}

	// Headless fail-open (AC3c): with no renderer there is no window/surface
	// at all, so the gogpu/ui path is impossible — force legacy. A native
	// boot with a renderer selects the ui path even though the gogpu
	// provider may not be live yet (it appears during App.Run(); the
	// ggcanvas is resolved lazily on first draw, WASM guard P2). The
	// software-adapter case is handled at draw time by ensureCanvas.
	if selected && g.Renderer == nil {
		slog.Warn("quakeui: headless UI path unavailable; forcing legacy ui path")
		g.uiBackendForceLegacy = true
		selected = false
	}

	g.uiBackendPath = selected

	if selected {
		g.ensureQuakeUIOverlay()
		g.installInputRouter()
		g.wireUITeardown()
	}
}

// installInputRouter builds the decoupled input router (ADR-0012 §4.2) and
// installs it as the input system's policy point on the gogpu/ui path:
// OnMenuKey and the general OnKey sink delegate to the router, which makes
// the exclusive engine-vs-ui split per KeyDest. The legacy handlers remain
// reachable as the router's engine sink; when the path is later forced legacy
// the router routes everything to the engine anyway (fail-open). The
// poll-only backend was already selected at init (InitSubsystems, M2.2), so
// the EventSource callback path is never active here — the UI owns it.
func (g *Game) installInputRouter() {
	if g == nil || g.Input == nil {
		return
	}
	router := NewInputRouter(
		g.handleGameKeyEvent,
		g.forwardUIKey,
		g.uiPathActive,
		func(key int) bool {
			// Engine pre-route: backtick toggles the console; binding capture
			// must always reach the engine regardless of KeyDest (R1.2).
			return key == int('`') || g.WaitingForKeyBinding()
		},
	)
	g.inputRouter = router

	uiSink := router.RouteKeyEvent
	// Menu-mode exclusivity: the input system fires BOTH OnMenuKey and OnKey
	// while in KeyMenu mode (types_binding.go menu branch). On the gogpu/ui
	// path OnMenuKey already routed the event through the router (menu→ui),
	// so the OnKey wrapper must not re-route the same event (double-dispatch
	// guard, R1.2).
	g.Input.OnMenuKey = func(ev input.KeyEvent) {
		uiSink(ev, input.KeyMenu)
	}
	g.Input.OnKey = func(ev input.KeyEvent) {
		if g.Input.KeyDest() == input.KeyMenu {
			return // OnMenuKey already routed this event
		}
		uiSink(ev, g.Input.KeyDest())
	}
}

// WaitingForKeyBinding reports whether the menu is in key-capture mode (any
// key should reach the engine for binding assignment). Compact helper so the
// router's capture predicate stays readable.
func (g *Game) WaitingForKeyBinding() bool {
	if g == nil || g.Menu == nil {
		return false
	}
	return g.Menu.WaitingForKeyBinding()
}

// uiPathActive reports whether the frozen startup decision selected the
// gogpu/ui path. It never re-reads ui_backend; a mid-session cvar toggling
// after startup has no effect (G11 frozen path).
func (g *Game) uiPathActive() bool {
	return g != nil && g.uiBackendFrozen && g.uiBackendPath && !g.uiBackendForceLegacy
}

// startupUIPathSelected is the pre-freeze heuristic mirroring the selection
// initializeUIPath performs: ui_backend != 0 AND the renderer exists. Called
// during init (before the freeze) to pick the input backend; initializeUIPath
// performs the authoritative frozen decision later. A native boot selects the
// gogpu/ui path even though the provider livens only during App.Run.
func (g *Game) startupUIPathSelected() bool {
	if g == nil || g.Host == nil {
		return false
	}
	if !quakeui.IsGogpuUIPath(g.Host.CVar) {
		return false
	}
	return g.Renderer != nil
}

// wireUITeardown registers the gogpuApp.OnClose teardown for the quakeui
// overlay once. The engine-owned loop never calls App.Frame after close, so
// teardown here releases the widget tree owned by the uiApp (G11 lifecycle).
func (g *Game) wireUITeardown() {
	if g == nil || g.uiTeardownRegistered {
		return
	}
	g.uiTeardownRegistered = true

	closeWired := false
	if g.Renderer != nil {
		if closer, ok := g.Renderer.(interface{ OnClose(func()) }); ok {
			closer.OnClose(func() {
				g.teardownUIPath()
			})
			closeWired = true
		}
	}
	if !closeWired {
		slog.Debug("quakeui: no renderer close hook, ui path torn down with process exit")
	}
}

// teardownUIPath releases the quakeui overlay and its uiApp widget tree. It
// is idempotent and safe to call from the renderer close callback.
func (g *Game) teardownUIPath() {
	if g == nil || g.quakeuiOverlay == nil {
		return
	}
	g.quakeuiOverlay.Close()
	g.quakeuiOverlay = nil
	g.uiInput = nil
	g.uiHost = nil
	slog.Debug("quakeui: ui path torn down")
}
