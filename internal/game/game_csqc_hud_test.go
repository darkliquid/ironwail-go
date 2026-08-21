package game

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/host"
	"github.com/darkliquid/ironwail-go/internal/input"
)

// TestCSQCHUDFallbackSkipsNativeHUDWidget pins SPEC-004 §5.3 (M4.3): with a
// CSQC VM active on the gogpu/ui path, the native HUD widget must be skipped
// (SkipHUD true + HUDRoot not visible) so the mod's CSQC_DrawHud canvas path
// owns the HUD — the widget tree never double-draws over it.
func TestCSQCHUDFallbackSkipsNativeHUDWidget(t *testing.T) {
	g := New()
	g.Host = host.NewHost()
	g.Host.CVar.Set("ui_backend", "1")
	g.Input = input.NewSystem(nil)
	_ = g.Input.Init()
	g.Renderer = &providerTestRenderer{}

	// Override the fallback decision: CSQC active (mod draws its HUD).
	owns := true
	g.csqcOwnsHUDTest = &owns

	overlay := g.ensureQuakeUIOverlay()
	if overlay == nil || overlay.HUDRoot() == nil {
		t.Fatal("quakeui overlay/HUDRoot not built")
	}
	provider, ok := overlay.HUDRoot().Provider().(interface{ SkipHUD() bool })
	if !ok {
		t.Fatal("HUDRoot provider does not implement the skip contract")
	}
	if !provider.SkipHUD() {
		t.Fatal("SkipHUD() = false with CSQC active — native HUD widget would double-draw over CSQC")
	}
	if overlay.HUDRoot().IsVisible() {
		t.Fatal("HUDRoot visible with CSQC active — must be skipped (legacy CSQC canvas path owns the HUD)")
	}
}

// TestCSQCHUDFallbackOffWithoutCSQC pins the inverse: without CSQC the native
// HUD widget renders normally on the gogpu/ui path.
func TestCSQCHUDFallbackOffWithoutCSQC(t *testing.T) {
	g := New()
	g.Host = host.NewHost()
	g.Host.CVar.Set("ui_backend", "1")
	g.Input = input.NewSystem(nil)
	_ = g.Input.Init()
	g.Renderer = &providerTestRenderer{}

	owns := false
	g.csqcOwnsHUDTest = &owns

	overlay := g.ensureQuakeUIOverlay()
	if overlay == nil || overlay.HUDRoot() == nil {
		t.Fatal("quakeui overlay/HUDRoot not built")
	}
	if !overlay.HUDRoot().IsVisible() {
		t.Fatal("HUDRoot not visible without CSQC — native HUD must render on the gogpu/ui path")
	}
}
