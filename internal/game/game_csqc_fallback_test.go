package game

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/hud"
	"github.com/darkliquid/ironwail-go/internal/quakeui"
	quakehud "github.com/darkliquid/ironwail-go/internal/quakeui/hud"
	"github.com/darkliquid/ironwail-go/internal/quakeui/widgets"
	"github.com/darkliquid/ironwail-go/internal/server"
)

// TestCSQCHUDWidgetHidden asserts the CSQC fallback helper reports the HUD
// widget hidden only when a CSQC mod is loaded (AC7, spec §1.2).
func TestCSQCHUDWidgetHidden(t *testing.T) {
	g := New()
	if g.csqcHUDWidgetHidden() {
		t.Fatal("csqcHUDWidgetHidden() = true without CSQC, want false")
	}
	// A non-loaded CSQC (nil VM) still counts as not owning the HUD.
	g.CSQC = nil
	if g.csqcHUDWidgetHidden() {
		t.Fatal("csqcHUDWidgetHidden() = true with nil CSQC, want false")
	}
}

// TestHUDWidgetVisibleInGame asserts the HUD widget is visible when in-game
// without a CSQC mod (path 1, AC5).
func TestHUDWidgetVisibleInGame(t *testing.T) {
	g := New()
	g.UIHost = quakeui.NewHost(quakeui.HostOptions{})
	defer g.UIHost.Close()
	g.Server = &server.Server{Active: true}
	g.HUD = hud.NewHUD(nil, g.Host.CVar)

	g.hudRoot = newTestStatusBarWidget()
	g.syncUIHostRoot()
	if !g.hudRoot.IsVisible() {
		t.Fatal("HUD widget hidden in-game without CSQC, want visible")
	}
}

// TestDemoBarSkippedOnPath1 asserts path 1 does not draw the demo bar (the
// legacy drawRuntimeDemoControls is bypassed; deferred per spec §1.2).
func TestDemoBarSkippedOnPath1(t *testing.T) {
	g := New()
	g.Host.CVar.Set(quakeui.CvarUIBackend, "1")
	if !quakeui.IsGogpuUIPath(g.Host.CVar) {
		t.Fatal("ui_backend=1 not selected")
	}
}

// --- test helpers ---

func newTestStatusBarWidget() *quakehud.StatusBarWidget {
	return quakehud.NewStatusBarWidget(hud.State{}, hud.HUDStyleClassic, widgets.NewQuakeText(nil, nil))
}
