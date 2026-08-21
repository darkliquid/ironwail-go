package game

import (
	"log/slog"

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

	// Software/headless fallback: without a live gogpu device provider the
	// ggcanvas cannot composite over the world; force the legacy path so the
	// session never renders a broken overlay (AC3c/AC4 fail-open).
	if selected && g.gpuContextProvider() == nil {
		slog.Warn("quakeui: software/headless UI path unavailable; forcing legacy ui path")
		g.uiBackendForceLegacy = true
		selected = false
	}

	g.uiBackendPath = selected

	if selected {
		g.ensureQuakeUIOverlay()
		g.wireUITeardown()
	}
}

// uiPathActive reports whether the frozen startup decision selected the
// gogpu/ui path. It never re-reads ui_backend; a mid-session cvar toggling
// after startup has no effect (G11 frozen path).
func (g *Game) uiPathActive() bool {
	return g != nil && g.uiBackendFrozen && g.uiBackendPath && !g.uiBackendForceLegacy
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
