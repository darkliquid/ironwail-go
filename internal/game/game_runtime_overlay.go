package game

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/game/ui"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

func (g *Game) buildRuntimeTelemetryState(conForcedup bool) TelemetryState {
	state := TelemetryState{
		ViewSize:        float32(g.currentRuntimeViewSize()),
		HUDStyle:        g.Host.CVar.IntValue("hud_style"),
		ShowFPS:         float32(g.Host.CVar.FloatValue("scr_showfps")),
		ShowClock:       g.Host.CVar.IntValue("scr_clock"),
		ShowSpeed:       g.Host.CVar.BoolValue("scr_showspeed"),
		ShowTurtle:      g.currentShowTurtle(),
		ShowSpeedOfs:    float32(g.Host.CVar.FloatValue("scr_showspeed_ofs")),
		DemoBarTimeout:  float32(g.Host.CVar.FloatValue("scr_demobar_timeout")),
		ConsoleForced:   conForcedup,
		LastServerMsgAt: g.LastServerMessageAt,
	}
	if g.Host != nil {
		state.RealTime = g.Host.RealTime()
		state.FrameCount = g.Host.FrameCount()
		state.FrameTime = g.Host.FrameTime()
		state.SavingActive = g.Host.SavingIndicatorActive(state.RealTime)
		if demo := g.Host.DemoState(); demo != nil {
			state.DemoPlayback = demo.Playback
			state.DemoSpeed = demo.Speed
			state.DemoBaseSpeed = demo.BaseSpeed
			state.DemoProgress = demo.Progress()
			state.DemoName = g.runtimeDemoName(demo.Filename)
		}
	}
	if g.Client != nil {
		state.ClientTime = g.Client.Time
		state.Intermission = g.Client.Intermission
		state.InCutscene = g.Client.InCutscene()
		state.Velocity = g.Client.Velocity
		state.ClientActive = g.Client.State == cl.StateActive
	}
	return state
}

func (g *Game) runtimeOverlayViewRect(framebufferW, framebufferH int, csqcDrawHUD bool) renderer.ViewRect {
	vidW := framebufferW
	if vidW <= 0 {
		vidW = g.Host.CVar.IntValue("vid_width")
	}
	vidH := framebufferH
	if vidH <= 0 {
		vidH = g.Host.CVar.IntValue("vid_height")
	}
	guiW, guiH := g.runtimeGUIDimensions(framebufferW, framebufferH)
	conW, conH := g.runtimeConsoleDimensions(guiW, guiH)
	ref, err := renderer.CalcRefdef(renderer.ScreenMetrics{
		GLWidth:        framebufferW,
		GLHeight:       framebufferH,
		VidWidth:       vidW,
		VidHeight:      vidH,
		GUIWidth:       guiW,
		GUIHeight:      guiH,
		ConWidth:       conW,
		ConHeight:      conH,
		ViewSize:       float32(g.currentRuntimeViewSize()),
		FOV:            g.currentRuntimeFOV(),
		FOVAdapt:       g.currentRuntimeFOVAdapt(),
		ZoomFOV:        g.currentRuntimeZoomFOV(),
		Zoom:           g.Zoom,
		SbarScale:      float32(g.Host.CVar.FloatValue("scr_sbarscale")),
		SbarAlpha:      g.currentSbarAlpha(),
		MenuScale:      float32(g.Host.CVar.FloatValue("scr_menuscale")),
		CrosshairScale: float32(g.Host.CVar.FloatValue("scr_crosshairscale")),
		Intermission:   g.Client != nil && g.Client.Intermission != 0,
		HudStyle:       g.Host.CVar.IntValue("hud_style"),
		CSQCDrawHud:    csqcDrawHUD,
	})
	if err != nil {
		return renderer.ViewRect{X: 0, Y: 0, Width: framebufferW, Height: framebufferH}
	}
	return ref.VRect
}

func (g *Game) currentSbarAlpha() float32 {
	alpha := float32(g.Host.CVar.FloatValue("scr_sbaralpha"))
	if alpha <= 0 {
		return 0
	}
	if alpha > 1 {
		return 1
	}
	return alpha
}

func (g *Game) currentRuntimeFOV() float32 {
	if cv := g.Host.CVar.Get("fov"); cv != nil && cv.Float32() > 0 {
		return cv.Float32()
	}
	return 90
}

func (g *Game) currentRuntimePixelAspect() float64 {
	cv := g.Host.CVar.Get("scr_pixelaspect")
	if cv == nil {
		return 1
	}
	if parts := strings.Split(cv.String, ":"); len(parts) == 2 {
		num, errNum := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		den, errDen := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if errNum == nil && errDen == nil && num > 0 && den > 0 {
			return g.clampf64(num/den, 0.5, 2)
		}
	}
	if cv.Float > 0 {
		return g.clampf64(cv.Float, 0.5, 2)
	}
	return 1
}

func (g *Game) clampf64(v, min, max float64) float64 {
	return ui.ClampF64(v, min, max)
}

func (g *Game) currentRuntimeViewSize() float64 {
	if cv := g.Host.CVar.Get("viewsize"); cv != nil && cv.Float > 0 {
		return cv.Float
	}
	if cv := g.Host.CVar.Get("scr_viewsize"); cv != nil && cv.Float > 0 {
		return cv.Float
	}
	return 100
}

func (g *Game) currentRuntimeZoomFOV() float32 {
	if cv := g.Host.CVar.Get("zoom_fov"); cv != nil && cv.Float32() > 0 {
		return cv.Float32()
	}
	return 30
}

func (g *Game) currentRuntimeFOVAdapt() bool {
	if cv := g.Host.CVar.Get("fov_adapt"); cv != nil {
		return cv.Bool()
	}
	return true
}

func (g *Game) currentShowTurtle() bool {
	if cv := g.Host.CVar.Get("showturtle"); cv != nil {
		return cv.Bool()
	}
	return g.Host.CVar.BoolValue("scr_showturtle")
}

func (g *Game) drawRuntimeString(rc renderer.RenderContext, x, y int, text string) {
	for _, ch := range text {
		rc.DrawCharacter(x, y, int(ch))
		x += 8
	}
}

func (g *Game) drawRuntimeClock(rc renderer.RenderContext, state TelemetryState) {
	if rc == nil || state.ShowClock != 1 || state.ViewSize >= 130 {
		return
	}
	minutes := int(state.ClientTime) / 60
	seconds := int(state.ClientTime) % 60
	text := fmt.Sprintf("%d:%02d", minutes, seconds)
	if state.HUDStyle == renderer.HUDClassic {
		rc.SetCanvas(renderer.CanvasBottomRight)
		g.drawRuntimeString(rc, 320-len(text)*8, 200-8, text)
		return
	}
	rc.SetCanvas(renderer.CanvasTopRight)
	g.drawRuntimeString(rc, 320-16-len(text)*8, 8, text)
}

func (g *Game) drawRuntimeFPS(rc renderer.RenderContext, state TelemetryState, overlay *FPSOverlay) {
	if rc == nil || overlay == nil {
		return
	}
	if state.ConsoleForced {
		overlay.OldTime = state.RealTime
		overlay.OldFrameCount = state.FrameCount
		overlay.LastFPS = 0
		return
	}
	elapsed := state.RealTime - overlay.OldTime
	frames := state.FrameCount - overlay.OldFrameCount
	if elapsed < 0 || frames < 0 {
		overlay.OldTime = state.RealTime
		overlay.OldFrameCount = state.FrameCount
		return
	}
	if elapsed > 0.75 {
		overlay.LastFPS = float64(frames) / elapsed
		overlay.OldTime = state.RealTime
		overlay.OldFrameCount = state.FrameCount
	}
	if state.ShowFPS == 0 || state.ViewSize >= 130 || overlay.LastFPS == 0 {
		return
	}
	text := fmt.Sprintf("%4.0f fps", overlay.LastFPS)
	if state.ShowFPS < 0 || state.ShowFPS >= 2 {
		text = fmt.Sprintf("%.2f ms", 1000.0/overlay.LastFPS)
	}
	if state.HUDStyle == renderer.HUDClassic {
		y := 200 - 8
		if state.ShowClock == 1 {
			y -= 8
		}
		rc.SetCanvas(renderer.CanvasBottomRight)
		g.drawRuntimeString(rc, 320-len(text)*8, y, text)
		return
	}
	y := 8
	if state.ShowClock == 1 {
		y += 8
	}
	rc.SetCanvas(renderer.CanvasTopRight)
	g.drawRuntimeString(rc, 320-16-len(text)*8, y, text)
}

func (g *Game) drawRuntimeSpeed(rc renderer.RenderContext, state TelemetryState, overlay *SpeedOverlay) {
	if rc == nil || overlay == nil {
		return
	}
	if overlay.LastRealTime == 0 && overlay.DisplaySpeed == 0 && overlay.MaxSpeed == 0 {
		overlay.DisplaySpeed = -1
	}
	if overlay.LastRealTime > state.RealTime {
		overlay.LastRealTime = 0
		overlay.DisplaySpeed = -1
		overlay.MaxSpeed = 0
	}
	speed := float32(math.Sqrt(float64(state.Velocity.X*state.Velocity.X + state.Velocity.Y*state.Velocity.Y)))
	if speed > overlay.MaxSpeed {
		overlay.MaxSpeed = speed
	}
	if state.ShowSpeed && overlay.DisplaySpeed >= 0 && state.Intermission == 0 && !state.InCutscene && state.ViewSize < 130 {
		text := fmt.Sprintf("%d", int(overlay.DisplaySpeed))
		rc.SetCanvas(renderer.CanvasCrosshair)
		canvas := rc.Canvas()
		top := canvas.Top
		bottom := canvas.Bottom
		if top == 0 && bottom == 0 {
			top = -100
			bottom = 100
		}
		y := min(max(top, 4+state.ShowSpeedOfs), bottom-8)
		g.drawRuntimeString(rc, -(len(text) * 4), int(y), text)
	}
	if state.RealTime-overlay.LastRealTime >= 0.05 {
		overlay.LastRealTime = state.RealTime
		overlay.DisplaySpeed = overlay.MaxSpeed
		overlay.MaxSpeed = 0
	}
}

type picAlphaRenderContext interface {
	DrawPicAlpha(x, y int, pic *qimage.QPic, alpha float32)
}

func (g *Game) drawRuntimePicAlpha(rc renderer.RenderContext, x, y int, pic *qimage.QPic, alpha float32) {
	if rc == nil || pic == nil || alpha <= 0 {
		return
	}
	if picAlpha, ok := rc.(picAlphaRenderContext); ok {
		picAlpha.DrawPicAlpha(x, y, pic, alpha)
		return
	}
	rc.DrawPic(x, y, pic)
}

func (g *Game) drawRuntimeTextBoxAlpha(rc renderer.RenderContext, pics picProvider, x, y, width, lines int, alpha float32) {
	if rc == nil || pics == nil || alpha <= 0 {
		return
	}
	cx := x
	cy := y

	if pic := pics.Pic("gfx/box_tl.lmp"); pic != nil {
		g.drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
	}
	if pic := pics.Pic("gfx/box_ml.lmp"); pic != nil {
		for n := 0; n < lines; n++ {
			cy += 8
			g.drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
		}
	}
	if pic := pics.Pic("gfx/box_bl.lmp"); pic != nil {
		g.drawRuntimePicAlpha(rc, cx, cy+8, pic, alpha)
	}

	cx += 8
	for remaining := width; remaining > 0; remaining -= 2 {
		cy = y
		if pic := pics.Pic("gfx/box_tm.lmp"); pic != nil {
			g.drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
		}
		for n := 0; n < lines; n++ {
			cy += 8
			name := "gfx/box_mm.lmp"
			if n == 1 {
				name = "gfx/box_mm2.lmp"
			}
			if pic := pics.Pic(name); pic != nil {
				g.drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
			}
		}
		if pic := pics.Pic("gfx/box_bm.lmp"); pic != nil {
			g.drawRuntimePicAlpha(rc, cx, cy+8, pic, alpha)
		}
		cx += 16
	}

	cy = y
	if pic := pics.Pic("gfx/box_tr.lmp"); pic != nil {
		g.drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
	}
	if pic := pics.Pic("gfx/box_mr.lmp"); pic != nil {
		for n := 0; n < lines; n++ {
			cy += 8
			g.drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
		}
	}
	if pic := pics.Pic("gfx/box_br.lmp"); pic != nil {
		g.drawRuntimePicAlpha(rc, cx, cy+8, pic, alpha)
	}
}

func (g *Game) runtimeDemoName(name string) string {
	return ui.DemoName(name)
}

func (g *Game) formatRuntimeDemoBaseSpeed(speed float32) string {
	return ui.FormatDemoBaseSpeed(speed)
}

func (g *Game) drawRuntimeDemoControls(rc renderer.RenderContext, pics picProvider, state TelemetryState, overlay *DemoOverlay) {
	if rc == nil || overlay == nil || !state.DemoPlayback || state.DemoBarTimeout < 0 {
		if overlay != nil {
			overlay.ShowTime = 0
		}
		return
	}
	if state.DemoSpeed != overlay.PrevSpeed ||
		state.DemoBaseSpeed != overlay.PrevBaseSpeed ||
		math.Abs(float64(state.DemoSpeed)) > math.Abs(float64(state.DemoBaseSpeed)) ||
		state.DemoBarTimeout == 0 {
		overlay.PrevSpeed = state.DemoSpeed
		overlay.PrevBaseSpeed = state.DemoBaseSpeed
		overlay.ShowTime = 1
		if state.DemoBarTimeout > 0 {
			overlay.ShowTime = float64(state.DemoBarTimeout)
		}
	} else {
		overlay.ShowTime -= state.FrameTime
		if overlay.ShowTime < 0 {
			overlay.ShowTime = 0
			return
		}
	}

	const timebarChars = 38
	x := 160 - timebarChars/2*8
	y := -20
	rc.SetCanvas(renderer.CanvasSbar)
	if state.Intermission != 0 {
		rc.SetCanvas(renderer.CanvasMenu)
		y = 25
	}

	alpha := g.currentSbarAlpha()
	g.drawRuntimeTextBoxAlpha(rc, pics, x-8, y-8, timebarChars, 1, alpha)

	status := string([]byte{13})
	if g.Draw != nil && g.Draw.CustomConchars() {
		status = ">"
	}
	if state.DemoSpeed == 0 {
		status = "II"
	} else if math.Abs(float64(state.DemoSpeed)) > 1 {
		status += status
	}
	if state.DemoSpeed < 0 {
		status = strings.Repeat("<", len(status))
	}
	g.drawRuntimeString(rc, x, y, status)

	if base := g.formatRuntimeDemoBaseSpeed(state.DemoBaseSpeed); base != "" {
		g.drawRuntimeString(rc, x+(timebarChars-len(base))*8, y, base)
	}
	if state.DemoName != "" {
		g.drawRuntimeString(rc, 160-len(state.DemoName)*4, y, state.DemoName)
	}

	barY := y - 8
	rc.DrawCharacter(x-8, barY, 128)
	for i := 0; i < timebarChars; i++ {
		rc.DrawCharacter(x+i*8, barY, 129)
	}
	rc.DrawCharacter(x+timebarChars*8, barY, 130)

	progress := state.DemoProgress
	if progress < 0 {
		progress = 0
	}
	if progress > 1 {
		progress = 1
	}
	cursorX := x + int(float64((timebarChars-1)*8)*progress)
	rc.DrawCharacter(cursorX, barY, 131)

	seconds := int(state.ClientTime)
	timeText := fmt.Sprintf("%d:%02d", seconds/60, seconds%60)
	timeX := cursorX
	if colon := strings.IndexByte(timeText, ':'); colon >= 0 {
		timeX -= colon * 8
	}
	timeY := barY - 11
	g.drawRuntimeTextBoxAlpha(rc, pics, timeX-8-(len(timeText)&1)*4, timeY-8, len(timeText)+(len(timeText)&1), 1, alpha)
	g.drawRuntimeString(rc, timeX, timeY, timeText)
}

func (g *Game) drawRuntimeTurtle(rc renderer.RenderContext, pics picProvider, state TelemetryState, count *int) {
	if rc == nil || pics == nil || count == nil || !state.ShowTurtle {
		return
	}
	if state.FrameTime < 0.1 {
		*count = 0
		return
	}
	*count++
	if *count < 3 {
		return
	}
	if turtle := pics.Pic("turtle"); turtle != nil {
		rc.SetCanvas(renderer.CanvasDefault)
		rc.DrawPic(state.ViewRect.X, state.ViewRect.Y, turtle)
	}
}

func (g *Game) drawRuntimeNet(rc renderer.RenderContext, pics picProvider, state TelemetryState) {
	if rc == nil || pics == nil || !state.ClientActive || state.DemoPlayback {
		return
	}
	if state.RealTime-state.LastServerMsgAt < 0.3 {
		return
	}
	if netPic := pics.Pic("net"); netPic != nil {
		rc.SetCanvas(renderer.CanvasDefault)
		rc.DrawPic(state.ViewRect.X+64, state.ViewRect.Y, netPic)
	}
}

func (g *Game) drawRuntimeSavingIndicator(rc renderer.RenderContext, pics picProvider, state TelemetryState) {
	if rc == nil || pics == nil || !state.SavingActive {
		return
	}
	disc := pics.Pic("disc")
	if disc == nil {
		return
	}
	y := 8
	if state.HUDStyle != renderer.HUDClassic && state.ViewSize < 130 {
		if state.ShowClock == 1 {
			y += 8
		}
		if state.ShowFPS != 0 {
			y += 8
		}
		if y != 8 {
			y += 8
		}
	}
	rc.SetCanvas(renderer.CanvasTopRight)
	rc.DrawPic(320-16-int(disc.Width), y, disc)
}

func (g *Game) drawPauseOverlay(dc renderer.RenderContext, pics picProvider) {
	if dc == nil || pics == nil {
		return
	}
	if cv := g.Host.CVar.Get("showpause"); cv != nil && !cv.Bool() {
		return
	}
	dc.SetCanvas(renderer.CanvasMenu)
	if pause := pics.Pic("gfx/pause.lmp"); pause != nil {
		dc.DrawMenuPic((320-int(pause.Width))/2, (240-48-int(pause.Height))/2, pause)
	}
}
