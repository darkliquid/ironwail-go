package main

import (
	"time"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

func runtimeGUIDimensions(framebufferW, framebufferH int) (int, int) {
	guiW := framebufferW
	guiH := framebufferH
	if guiW <= 0 {
		guiW = cvar.IntValue("vid_width")
	}
	if guiH <= 0 {
		guiH = cvar.IntValue("vid_height")
	}
	pixelAspect := currentRuntimePixelAspect()
	if pixelAspect > 1 {
		guiW = int(float64(guiW)/pixelAspect + 0.5)
	} else if pixelAspect > 0 && pixelAspect < 1 {
		guiH = int(float64(guiH)*pixelAspect + 0.5)
	}
	return guiW, guiH
}

func runtimeConsoleDimensions(guiW, guiH int) (int, int) {
	if guiW <= 0 || guiH <= 0 {
		return 0, 0
	}
	conWidth := guiW
	if override := cvar.FloatValue("scr_conwidth"); override > 0 {
		conWidth = int(override)
	} else if scale := cvar.FloatValue("scr_conscale"); scale > 0 {
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

func runtimeCanvasParams(framebufferW, framebufferH int, slideFraction float32) renderer.CanvasTransformParams {
	guiW, guiH := runtimeGUIDimensions(framebufferW, framebufferH)
	conW, conH := runtimeConsoleDimensions(guiW, guiH)
	return renderer.CanvasTransformParams{
		GUIWidth:         float32(guiW),
		GUIHeight:        float32(guiH),
		GLWidth:          float32(framebufferW),
		GLHeight:         float32(framebufferH),
		ConWidth:         float32(conW),
		ConHeight:        float32(conH),
		MenuScale:        float32(cvar.FloatValue("scr_menuscale")),
		SbarScale:        float32(cvar.FloatValue("scr_sbarscale")),
		CrosshairScale:   float32(cvar.FloatValue("scr_crosshairscale")),
		ConSlideFraction: slideFraction,
	}
}

func runtimeOverlayCanvasParams(framebufferW, framebufferH int) renderer.CanvasTransformParams {
	return runtimeCanvasParams(framebufferW, framebufferH, clampUnitFloat32(g.ConsoleSlideFraction))
}

func runtimeConsoleCanvasParams(framebufferW, framebufferH int, slideFraction float32) renderer.CanvasTransformParams {
	return runtimeCanvasParams(framebufferW, framebufferH, slideFraction)
}

func runtimeConsoleBackgroundPic() *qimage.QPic {
	if g.Draw == nil {
		return nil
	}
	return g.Draw.GetPic("gfx/conback.lmp")
}

func drawRuntimeConsole(overlay renderer.RenderContext, framebufferW, framebufferH int, full, forcedup bool) {
	slideFraction := clampUnitFloat32(g.ConsoleSlideFraction)
	if forcedup {
		slideFraction = 1
	}
	params := runtimeConsoleCanvasParams(framebufferW, framebufferH, slideFraction)
	if setter, ok := overlay.(canvasParamSetter); ok {
		setter.SetCanvasParams(params)
	}
	overlay.SetCanvas(renderer.CanvasConsole)
	var background *qimage.QPic
	if full {
		background = runtimeConsoleBackgroundPic()
	}
	console.Draw(overlay, int(params.ConWidth), int(params.ConHeight), full, background, forcedup)
}

func updateRuntimeConsoleSlide(dt float64, consoleVisible, forcedup bool) {
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

	speed := float32(cvar.FloatValue("scr_conspeed"))
	if speed <= 0 {
		speed = 1e6
	}
	step := speed * float32(dt) / 300
	current := clampUnitFloat32(g.ConsoleSlideFraction)
	if current < target {
		current = min(current+step, target)
	} else if current > target {
		current = max(current-step, target)
	}
	g.ConsoleSlideFraction = clampUnitFloat32(current)
}

func runtimeConsoleAnimating() bool {
	return g.ConsoleSlideFraction > 0
}

func clampUnitFloat32(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func runtimeConsoleForcedUp() bool {
	if g.Client == nil {
		return true
	}
	if g.Client.State == cl.StateActive {
		return false
	}
	return g.Client.Signon < cl.Signons
}

func runtimeViewModelVisible() bool {
	if g.Client == nil {
		return false
	}
	if g.Menu != nil && g.Menu.IsActive() {
		return false
	}
	if g.Client.Intermission != 0 {
		return false
	}
	if !cvar.BoolValue("r_drawentities") {
		return false
	}
	if !cvar.BoolValue("r_drawviewmodel") {
		return false
	}
	if cvar.BoolValue("chase_active") {
		return false
	}
	if currentRuntimeViewSize() >= 130 {
		return false
	}
	if g.Client.Health() <= 0 {
		return false
	}
	return g.Client.Items&cl.ItemInvisibility == 0
}

func runtimePauseActive() bool {
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

func drawMenuBackdrop(rc renderer.RenderContext, w, h int) {
	if rc == nil || w <= 0 || h <= 0 {
		return
	}
	rc.SetCanvas(renderer.CanvasDefault)
	alpha := float32(cvar.FloatValue("scr_menubgalpha"))
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	rc.DrawFillAlpha(0, 0, w, h, 0, alpha)
}

func drawRuntimeMenu(rc renderer.RenderContext, w, h int, drawMenu func(renderer.RenderContext)) {
	if rc == nil || drawMenu == nil {
		return
	}
	if setter, ok := rc.(canvasParamSetter); ok {
		setter.SetCanvasParams(runtimeOverlayCanvasParams(w, h))
	}
	drawMenuBackdrop(rc, w, h)
	rc.SetCanvas(renderer.CanvasMenu)
	drawMenu(rc)
}

func drawChatInput(rc renderer.RenderContext, w, _ int) {
	prompt := "say: "
	if chatTeam {
		prompt = "say_team: "
	}
	fullText := clippedChatInput(prompt, chatBuffer, max(1, w/8-2))

	y := console.NotifyLineCount() * 8
	x := 8
	currentX := x
	for _, char := range fullText {
		rc.DrawCharacter(currentX, y, int(char))
		currentX += 8
	}
	rc.DrawCharacter(currentX, y, runtimeCursorGlyph(runtimeNow()))
}

func clippedChatInput(prompt, message string, maxChars int) string {
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

func runtimeCursorGlyph(now time.Time) int {
	frame := (now.UnixNano() / int64(time.Second/4)) & 1
	return 10 + int(frame)
}
