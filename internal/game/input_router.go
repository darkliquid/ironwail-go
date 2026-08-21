package game

import (
	"github.com/darkliquid/ironwail-go/internal/input"
)

// InputRouter is the single policy point for the decoupled input path
// (ADR-0012 §4.2/§5.2, R1.2). It owns the exclusive key routing decision
// between the engine and the gogpu/ui widget tree on the ui_backend=1 path:
//
//	KeyGame/HUD-only → engine gameplay (polling app.Input() later, M2.2)
//	KeyConsole/KeyMenu → UI only (EventSource → widget tree)
//	KeyMessage        → engine (chat input stays engine-side)
//	backtick/binding-capture → engine pre-route (before the router)
//
// The split is EXCLUSIVE for keys: a key routed to the UI never also reaches
// the engine's key handlers, and vice versa — guarding the v3 double-dispatch
// bug where surface keys hit both the engine and the widget tree.
//
// The router is called from the input system's per-destination callbacks
// (installed in place of the legacy OnKey/OnMenuKey wiring when the gogpu/ui
// path is active).
type InputRouter struct {
	// engine receives events destined for the engine's key routing.
	engine func(ev input.KeyEvent)
	// ui receives events destined for the widget tree.
	ui func(ev input.KeyEvent)
	// uiActive reports the frozen startup path decision (G11).
	uiActive func() bool
	// captureKey reports engine pre-route keys (backtick/binding capture).
	captureKey func(key int) bool
}

// NewInputRouter builds the router with the engine and ui event sinks.
func NewInputRouter(
	engine func(ev input.KeyEvent),
	ui func(ev input.KeyEvent),
	uiActive func() bool,
	captureKey func(key int) bool,
) *InputRouter {
	if engine == nil {
		engine = func(ev input.KeyEvent) {}
	}
	if ui == nil {
		ui = func(ev input.KeyEvent) {}
	}
	if captureKey == nil {
		captureKey = func(int) bool { return false }
	}
	return &InputRouter{
		engine:     engine,
		ui:         ui,
		uiActive:   uiActive,
		captureKey: captureKey,
	}
}

// RouteKeyEvent is the single policy point for one platform key event. It
// returns the sink that consumed the event (for tests and diagnostics).
//
// Exclusive routing (R1.2): each KeyDest maps to exactly one sink. When the
// gogpu/ui path is NOT active (legacy), everything routes to the engine so
// behavior is byte-for-byte identical to the pre-rewrite input system.
func (r *InputRouter) RouteKeyEvent(ev input.KeyEvent, dest input.KeyDest) string {
	if r == nil {
		return "none"
	}
	// Backtick/binding capture pre-routes to the engine regardless of dest
	// (the console toggle and key-binding capture must always see it).
	if r.captureKey(ev.Key) {
		r.engine(ev)
		return "engine-capture"
	}

	uiOn := r.uiActive != nil && r.uiActive()
	if !uiOn {
		// Legacy path: engine owns everything (OnKey/OnMenuKey as today).
		r.engine(ev)
		return "engine"
	}

	switch dest {
	case input.KeyConsole, input.KeyMenu:
		// Exclusive: the widget tree consumes menu/console input on path 1.
		// The engine must NOT also process these (guards double-dispatch).
		r.ui(ev)
		return "ui"
	case input.KeyGame, input.KeyMessage:
		r.engine(ev)
		return "engine"
	default:
		r.engine(ev)
		return "engine"
	}
}
