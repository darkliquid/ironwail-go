package game

import (
	"github.com/darkliquid/ironwail-go/internal/hud"
	"github.com/darkliquid/ironwail-go/internal/renderer"
	quakeuihud "github.com/darkliquid/ironwail-go/internal/quakeui/hud"
)

// csqcAwareHUDProvider is the quakeui HUD provider used on the gogpu/ui path.
// It wraps the engine's *hud.HUD (the legacy native HUD renderer) and adds
// the CSQC fallback (SPEC-004 §5.3): when a mod loads CSQC, the native HUD
// widget is skipped so the legacy CSQC_DrawHud canvas path owns the HUD —
// the widget tree never double-draws the HUD over the mod's.
type csqcAwareHUDProvider struct {
	hud *hud.HUD
	g   *Game
}

// State delegates to the wrapped HUD.
func (p *csqcAwareHUDProvider) State() hud.State {
	if p == nil || p.hud == nil {
		return hud.State{}
	}
	return p.hud.State()
}

// Style delegates to the wrapped HUD.
func (p *csqcAwareHUDProvider) Style() hud.HUDStyle {
	if p == nil || p.hud == nil {
		return hud.HUDStyleClassic
	}
	return p.hud.Style()
}

// Draw delegates to the wrapped HUD (only invoked when SkipHUD is false).
func (p *csqcAwareHUDProvider) Draw(rc renderer.RenderContext) {
	if p == nil || p.hud == nil {
		return
	}
	p.hud.Draw(rc)
}

// SkipHUD reports whether the native HUD widget must be skipped this frame:
// true when a CSQC VM with DrawHud support is loaded (the mod draws its own
// HUD via the legacy CSQC canvas path).
func (p *csqcAwareHUDProvider) SkipHUD() bool {
	if p == nil || p.g == nil {
		return false
	}
	return p.g.csqcOwnsHUD()
}

// csqcOwnsHUD reports whether a loaded CSQC VM draws its own HUD this frame.
// The optional csqcOwnsHUDTest hooks let tests force the decision without a
// real mod payload; nil uses the live CSQC state.
func (g *Game) csqcOwnsHUD() bool {
	if g == nil {
		return false
	}
	if g.csqcOwnsHUDTest != nil {
		return *g.csqcOwnsHUDTest
	}
	return g.CSQC != nil && g.CSQC.IsLoaded()
}

// compile-time checks: csqcAwareHUDProvider satisfies the quakeui provider
// contract including the optional skip interface.
var (
	_ quakeuihud.HUDStateProvider = (*csqcAwareHUDProvider)(nil)
	_ quakeuihud.SkipProvider     = (*csqcAwareHUDProvider)(nil)
)
