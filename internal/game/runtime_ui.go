package game

import (
	"time"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/console"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

func (g *Game) runtimeGUIDimensions(framebufferW, framebufferH int) (int, int) {
	guiW := framebufferW
	guiH := framebufferH
	if guiW <= 0 {
		guiW = g.Host.CVar.IntValue("vid_width")
	}
	if guiH <= 0 {
		guiH = g.Host.CVar.IntValue("vid_height")
	}
	pixelAspect := g.currentRuntimePixelAspect()
	if pixelAspect > 1 {
		guiW = int(float64(guiW)/pixelAspect + 0.5)
	} else if pixelAspect > 0 && pixelAspect < 1 {
		guiH = int(float64(guiH)*pixelAspect + 0.5)
	}
	return guiW, guiH
}

func (g *Game) runtimeConsoleDimensions(guiW, guiH int) (int, int) {
	if guiW <= 0 || guiH <= 0 {
		return 0, 0
	}
	conWidth := guiW
	if override := g.Host.CVar.FloatValue("scr_conwidth"); override > 0 {
		conWidth = int(override)
	} else if scale := g.Host.CVar.FloatValue("scr_conscale"); scale > 0 {
		conWidth = int(float64(guiW) / scale)
	}
	if conWidth < 320 {
		conWidth = 320
	}
	if conWidth > guiW {
		conWidth = guiW
	}
	conWidth &^= 7
	if conWidth <= 0 {
		conWidth = guiW
	}
	conHeight := conWidth * guiH / guiW
	if conHeight <= 0 {
		conHeight = guiH
	}
	return conWidth, conHeight
}

func (g *Game) runtimeCanvasParams(framebufferW, framebufferH int, slideFraction float32) renderer.CanvasTransformParams {
	guiW, guiH := g.runtimeGUIDimensions(framebufferW, framebufferH)
	conW, conH := g.runtimeConsoleDimensions(guiW, guiH)
	return renderer.CanvasTransformParams{
		GUIWidth:         float32(guiW),
		GUIHeight:        float32(guiH),
		GLWidth:          float32(framebufferW),
		GLHeight:         float32(framebufferH),
		ConWidth:         float32(conW),
		ConHeight:        float32(conH),
		MenuScale:        float32(g.Host.CVar.FloatValue("scr_menuscale")),
		SbarScale:        float32(g.Host.CVar.FloatValue("scr_sbarscale")),
		CrosshairScale:   float32(g.Host.CVar.FloatValue("scr_crosshairscale")),
		ConSlideFraction: slideFraction,
	}
}

func (g *Game) runtimeOverlayCanvasParams(framebufferW, framebufferH int) renderer.CanvasTransformParams {
	return g.runtimeCanvasParams(framebufferW, framebufferH, g.clampUnitFloat32(g.ConsoleSlideFraction))
}

func (g *Game) runtimeConsoleCanvasParams(framebufferW, framebufferH int, slideFraction float32) renderer.CanvasTransformParams {
	return g.runtimeCanvasParams(framebufferW, framebufferH, slideFraction)
}

func (g *Game) runtimeConsoleBackgroundPic() *qimage.QPic {
	if g.Draw == nil {
		return nil
	}
	return g.Draw.Pic("gfx/conback.lmp")
}

func (g *Game) drawRuntimeConsole(overlay renderer.RenderContext, framebufferW, framebufferH int, full, forcedup bool) {
	slideFraction := g.clampUnitFloat32(g.ConsoleSlideFraction)
	if forcedup {
		slideFraction = 1
	}
	params := g.runtimeConsoleCanvasParams(framebufferW, framebufferH, slideFraction)
	if setter, ok := overlay.(CanvasParamSetter); ok {
		setter.SetCanvasParams(params)
	}
	overlay.SetCanvas(renderer.CanvasConsole)
	var background *qimage.QPic
	if full {
		background = g.runtimeConsoleBackgroundPic()
	}
	console.Draw(overlay, int(params.ConWidth), int(params.ConHeight), full, background, forcedup)
}

func (g *Game) updateRuntimeConsoleSlide(dt float64, consoleVisible, forcedup bool) {
	if forcedup {
		g.ConsoleSlideFraction = 1
		return
	}

	target := float32(0)
	if consoleVisible {
		target = 1
	}
	if dt <= 0 {
		g.ConsoleSlideFraction = target
		return
	}

	speed := float32(g.Host.CVar.FloatValue("scr_conspeed"))
	if speed <= 0 {
		speed = 1e6
	}
	step := speed * float32(dt) / 300
	current := g.clampUnitFloat32(g.ConsoleSlideFraction)
	if current < target {
		current = min(current+step, target)
	} else if current > target {
		current = max(current-step, target)
	}
	g.ConsoleSlideFraction = g.clampUnitFloat32(current)
}

func (g *Game) runtimeConsoleAnimating() bool {
	return g.ConsoleSlideFraction > 0
}

func (g *Game) clampUnitFloat32(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func (g *Game) runtimeConsoleForcedUp() bool {
	if g.Client == nil {
		return true
	}
	if g.Client.State == cl.StateActive {
		return false
	}
	return g.Client.Signon < cl.Signons
}

func (g *Game) runtimeViewModelVisible() bool {
	if g.Client == nil {
		g.runtimeDebugViewLogf("viewmodel_skip", "reason=no_client")
		return false
	}
	// Note: C Ironwail's R_IsViewModelVisible does NOT suppress the
	// viewmodel when the menu is open — the main menu is drawn on top
	// of the 3D scene and the viewmodel/HUD remain visible underneath.
	// Hiding it here caused parity captures running behind the attract-
	// mode menu overlay to show no weapon while C did.
	if g.Client.Intermission != 0 {
		g.runtimeDebugViewLogf("viewmodel_skip", "reason=intermission value=%d", g.Client.Intermission)
		return false
	}
	if !g.Host.CVar.BoolValue("r_drawentities") {
		g.runtimeDebugViewLogf("viewmodel_skip", "reason=r_drawentities")
		return false
	}
	if !g.Host.CVar.BoolValue("r_drawviewmodel") {
		g.runtimeDebugViewLogf("viewmodel_skip", "reason=r_drawviewmodel")
		return false
	}
	if g.Host.CVar.BoolValue("chase_active") {
		g.runtimeDebugViewLogf("viewmodel_skip", "reason=chase_active")
		return false
	}
	if g.currentRuntimeViewSize() >= 130 {
		g.runtimeDebugViewLogf("viewmodel_skip", "reason=viewsize value=%.1f", g.currentRuntimeViewSize())
		return false
	}
	if g.Client.Health() <= 0 {
		g.runtimeDebugViewLogf("viewmodel_skip", "reason=health value=%d", g.Client.Health())
		return false
	}
	if g.Client.Items&cl.ItemInvisibility != 0 {
		g.runtimeDebugViewLogf("viewmodel_skip", "reason=invisibility items=0x%x", g.Client.Items)
		return false
	}
	return true
}

func (g *Game) runtimePauseActive() bool {
	if g.Host != nil {
		if demo := g.Host.DemoState(); demo != nil && demo.Playback && demo.Paused {
			return true
		}
		if g.Host.ServerPaused() {
			return true
		}
	}
	return g.Client != nil && g.Client.Paused
}

func (g *Game) drawMenuBackdrop(rc renderer.RenderContext, w, h int) {
	if rc == nil || w <= 0 || h <= 0 {
		return
	}
	rc.SetCanvas(renderer.CanvasDefault)
	alpha := float32(g.Host.CVar.FloatValue("scr_menubgalpha"))
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	rc.DrawFillAlpha(0, 0, w, h, 0, alpha)
}

func (g *Game) drawRuntimeMenu(rc renderer.RenderContext, w, h int, drawMenu func(renderer.RenderContext)) {
	if rc == nil || drawMenu == nil {
		return
	}
	if setter, ok := rc.(CanvasParamSetter); ok {
		setter.SetCanvasParams(g.runtimeOverlayCanvasParams(w, h))
	}
	g.drawMenuBackdrop(rc, w, h)
	rc.SetCanvas(renderer.CanvasMenu)
	drawMenu(rc)
}

func (g *Game) drawChatInput(rc renderer.RenderContext, w, _ int) {
	prompt := "say: "
	if g.chatTeam {
		prompt = "say_team: "
	}
	fullText := g.clippedChatInput(prompt, g.chatBuffer, max(1, w/8-2))

	y := console.NotifyLineCount() * 8
	x := 8
	currentX := x
	for _, char := range fullText {
		rc.DrawCharacter(currentX, y, int(char))
		currentX += 8
	}
	rc.DrawCharacter(currentX, y, g.runtimeCursorGlyph(time.Now()))
}

func (g *Game) clippedChatInput(prompt, message string, maxChars int) string {
	if maxChars <= 1 {
		return prompt[:min(len(prompt), 1)]
	}
	visiblePrompt := prompt
	if len(visiblePrompt) > maxChars-1 {
		visiblePrompt = visiblePrompt[:maxChars-1]
	}
	remaining := maxChars - len(visiblePrompt) - 1
	if remaining <= 0 {
		return visiblePrompt
	}
	if len(message) > remaining {
		message = message[len(message)-remaining:]
	}
	return visiblePrompt + message
}

func (g *Game) runtimeCursorGlyph(now time.Time) int {
	frame := (now.UnixNano() / int64(time.Second/4)) & 1
	return 10 + int(frame)
}
