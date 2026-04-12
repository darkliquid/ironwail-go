package main

import (
	"fmt"
	"math"
	"path/filepath"
	"strconv"
	"strings"

	cl "github.com/darkliquid/ironwail-go/internal/client"
	"github.com/darkliquid/ironwail-go/internal/cvar"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

func buildRuntimeTelemetryState(conForcedup bool) runtimeTelemetryState {
	state := runtimeTelemetryState{
		ViewSize:        float32(currentRuntimeViewSize()),
		HUDStyle:        cvar.IntValue("hud_style"),
		ShowFPS:         float32(cvar.FloatValue("scr_showfps")),
		ShowClock:       cvar.IntValue("scr_clock"),
		ShowSpeed:       cvar.BoolValue("scr_showspeed"),
		ShowTurtle:      currentShowTurtle(),
		ShowSpeedOfs:    float32(cvar.FloatValue("scr_showspeed_ofs")),
		DemoBarTimeout:  float32(cvar.FloatValue("scr_demobar_timeout")),
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
			state.DemoName = runtimeDemoName(demo.Filename)
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

func runtimeOverlayViewRect(framebufferW, framebufferH int, csqcDrawHUD bool) renderer.ViewRect {
	vidW := framebufferW
	if vidW <= 0 {
		vidW = cvar.IntValue("vid_width")
	}
	vidH := framebufferH
	if vidH <= 0 {
		vidH = cvar.IntValue("vid_height")
	}
	guiW, guiH := runtimeGUIDimensions(framebufferW, framebufferH)
	conW, conH := runtimeConsoleDimensions(guiW, guiH)
	ref, err := renderer.CalcRefdef(renderer.ScreenMetrics{
		GLWidth:        framebufferW,
		GLHeight:       framebufferH,
		VidWidth:       vidW,
		VidHeight:      vidH,
		GUIWidth:       guiW,
		GUIHeight:      guiH,
		ConWidth:       conW,
		ConHeight:      conH,
		ViewSize:       float32(currentRuntimeViewSize()),
		FOV:            currentRuntimeFOV(),
		FOVAdapt:       currentRuntimeFOVAdapt(),
		ZoomFOV:        currentRuntimeZoomFOV(),
		Zoom:           g.Zoom,
		SbarScale:      float32(cvar.FloatValue("scr_sbarscale")),
		SbarAlpha:      currentSbarAlpha(),
		MenuScale:      float32(cvar.FloatValue("scr_menuscale")),
		CrosshairScale: float32(cvar.FloatValue("scr_crosshairscale")),
		Intermission:   g.Client != nil && g.Client.Intermission != 0,
		HudStyle:       cvar.IntValue("hud_style"),
		CSQCDrawHud:    csqcDrawHUD,
	})
	if err != nil {
		return renderer.ViewRect{X: 0, Y: 0, Width: framebufferW, Height: framebufferH}
	}
	return ref.VRect
}

func currentSbarAlpha() float32 {
	alpha := float32(cvar.FloatValue("scr_sbaralpha"))
	if alpha <= 0 {
		return 0
	}
	if alpha > 1 {
		return 1
	}
	return alpha
}

func currentRuntimeFOV() float32 {
	if cv := cvar.Get("fov"); cv != nil && cv.Float32() > 0 {
		return cv.Float32()
	}
	return 90
}

func currentRuntimePixelAspect() float64 {
	cv := cvar.Get("scr_pixelaspect")
	if cv == nil {
		return 1
	}
	if parts := strings.Split(cv.String, ":"); len(parts) == 2 {
		num, errNum := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
		den, errDen := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64)
		if errNum == nil && errDen == nil && num > 0 && den > 0 {
			return clampf64(num/den, 0.5, 2)
		}
	}
	if cv.Float > 0 {
		return clampf64(cv.Float, 0.5, 2)
	}
	return 1
}

func clampf64(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func currentRuntimeViewSize() float64 {
	if cv := cvar.Get("viewsize"); cv != nil && cv.Float > 0 {
		return cv.Float
	}
	if cv := cvar.Get("scr_viewsize"); cv != nil && cv.Float > 0 {
		return cv.Float
	}
	return 100
}

func currentRuntimeZoomFOV() float32 {
	if cv := cvar.Get("zoom_fov"); cv != nil && cv.Float32() > 0 {
		return cv.Float32()
	}
	return 30
}

func currentRuntimeFOVAdapt() bool {
	if cv := cvar.Get("fov_adapt"); cv != nil {
		return cv.Bool()
	}
	return true
}

func currentShowTurtle() bool {
	if cv := cvar.Get("showturtle"); cv != nil {
		return cv.Bool()
	}
	return cvar.BoolValue("scr_showturtle")
}

func drawRuntimeString(rc renderer.RenderContext, x, y int, text string) {
	for _, ch := range text {
		rc.DrawCharacter(x, y, int(ch))
		x += 8
	}
}

func drawRuntimeClock(rc renderer.RenderContext, state runtimeTelemetryState) {
	if rc == nil || state.ShowClock != 1 || state.ViewSize >= 130 {
		return
	}
	minutes := int(state.ClientTime) / 60
	seconds := int(state.ClientTime) % 60
	text := fmt.Sprintf("%d:%02d", minutes, seconds)
	if state.HUDStyle == renderer.HUDClassic {
		rc.SetCanvas(renderer.CanvasBottomRight)
		drawRuntimeString(rc, 320-len(text)*8, 200-8, text)
		return
	}
	rc.SetCanvas(renderer.CanvasTopRight)
	drawRuntimeString(rc, 320-16-len(text)*8, 8, text)
}

func drawRuntimeFPS(rc renderer.RenderContext, state runtimeTelemetryState, overlay *runtimeFPSOverlay) {
	if rc == nil || overlay == nil {
		return
	}
	if state.ConsoleForced {
		overlay.oldTime = state.RealTime
		overlay.oldFrameCount = state.FrameCount
		overlay.lastFPS = 0
		return
	}
	elapsed := state.RealTime - overlay.oldTime
	frames := state.FrameCount - overlay.oldFrameCount
	if elapsed < 0 || frames < 0 {
		overlay.oldTime = state.RealTime
		overlay.oldFrameCount = state.FrameCount
		return
	}
	if elapsed > 0.75 {
		overlay.lastFPS = float64(frames) / elapsed
		overlay.oldTime = state.RealTime
		overlay.oldFrameCount = state.FrameCount
	}
	if state.ShowFPS == 0 || state.ViewSize >= 130 || overlay.lastFPS == 0 {
		return
	}
	text := fmt.Sprintf("%4.0f fps", overlay.lastFPS)
	if state.ShowFPS < 0 || state.ShowFPS >= 2 {
		text = fmt.Sprintf("%.2f ms", 1000.0/overlay.lastFPS)
	}
	if state.HUDStyle == renderer.HUDClassic {
		y := 200 - 8
		if state.ShowClock == 1 {
			y -= 8
		}
		rc.SetCanvas(renderer.CanvasBottomRight)
		drawRuntimeString(rc, 320-len(text)*8, y, text)
		return
	}
	y := 8
	if state.ShowClock == 1 {
		y += 8
	}
	rc.SetCanvas(renderer.CanvasTopRight)
	drawRuntimeString(rc, 320-16-len(text)*8, y, text)
}

func drawRuntimeSpeed(rc renderer.RenderContext, state runtimeTelemetryState, overlay *runtimeSpeedOverlay) {
	if rc == nil || overlay == nil {
		return
	}
	if overlay.lastRealTime == 0 && overlay.displaySpeed == 0 && overlay.maxSpeed == 0 {
		overlay.displaySpeed = -1
	}
	if overlay.lastRealTime > state.RealTime {
		overlay.lastRealTime = 0
		overlay.displaySpeed = -1
		overlay.maxSpeed = 0
	}
	speed := float32(math.Sqrt(float64(state.Velocity[0]*state.Velocity[0] + state.Velocity[1]*state.Velocity[1])))
	if speed > overlay.maxSpeed {
		overlay.maxSpeed = speed
	}
	if state.ShowSpeed && overlay.displaySpeed >= 0 && state.Intermission == 0 && !state.InCutscene && state.ViewSize < 130 {
		text := fmt.Sprintf("%d", int(overlay.displaySpeed))
		rc.SetCanvas(renderer.CanvasCrosshair)
		canvas := rc.Canvas()
		top := canvas.Top
		bottom := canvas.Bottom
		if top == 0 && bottom == 0 {
			top = -100
			bottom = 100
		}
		y := min(max(top, 4+state.ShowSpeedOfs), bottom-8)
		drawRuntimeString(rc, -(len(text) * 4), int(y), text)
	}
	if state.RealTime-overlay.lastRealTime >= 0.05 {
		overlay.lastRealTime = state.RealTime
		overlay.displaySpeed = overlay.maxSpeed
		overlay.maxSpeed = 0
	}
}

type picAlphaRenderContext interface {
	DrawPicAlpha(x, y int, pic *qimage.QPic, alpha float32)
}

func drawRuntimePicAlpha(rc renderer.RenderContext, x, y int, pic *qimage.QPic, alpha float32) {
	if rc == nil || pic == nil || alpha <= 0 {
		return
	}
	if picAlpha, ok := rc.(picAlphaRenderContext); ok {
		picAlpha.DrawPicAlpha(x, y, pic, alpha)
		return
	}
	rc.DrawPic(x, y, pic)
}

func drawRuntimeTextBoxAlpha(rc renderer.RenderContext, pics picProvider, x, y, width, lines int, alpha float32) {
	if rc == nil || pics == nil || alpha <= 0 {
		return
	}
	cx := x
	cy := y

	if pic := pics.GetPic("gfx/box_tl.lmp"); pic != nil {
		drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
	}
	if pic := pics.GetPic("gfx/box_ml.lmp"); pic != nil {
		for n := 0; n < lines; n++ {
			cy += 8
			drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
		}
	}
	if pic := pics.GetPic("gfx/box_bl.lmp"); pic != nil {
		drawRuntimePicAlpha(rc, cx, cy+8, pic, alpha)
	}

	cx += 8
	for remaining := width; remaining > 0; remaining -= 2 {
		cy = y
		if pic := pics.GetPic("gfx/box_tm.lmp"); pic != nil {
			drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
		}
		for n := 0; n < lines; n++ {
			cy += 8
			name := "gfx/box_mm.lmp"
			if n == 1 {
				name = "gfx/box_mm2.lmp"
			}
			if pic := pics.GetPic(name); pic != nil {
				drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
			}
		}
		if pic := pics.GetPic("gfx/box_bm.lmp"); pic != nil {
			drawRuntimePicAlpha(rc, cx, cy+8, pic, alpha)
		}
		cx += 16
	}

	cy = y
	if pic := pics.GetPic("gfx/box_tr.lmp"); pic != nil {
		drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
	}
	if pic := pics.GetPic("gfx/box_mr.lmp"); pic != nil {
		for n := 0; n < lines; n++ {
			cy += 8
			drawRuntimePicAlpha(rc, cx, cy, pic, alpha)
		}
	}
	if pic := pics.GetPic("gfx/box_br.lmp"); pic != nil {
		drawRuntimePicAlpha(rc, cx, cy+8, pic, alpha)
	}
}

func runtimeDemoName(name string) string {
	base := strings.TrimSuffix(filepath.Base(name), filepath.Ext(name))
	if len(base) > 30 {
		base = base[:30]
	}
	return base
}

func formatRuntimeDemoBaseSpeed(speed float32) string {
	if speed == 0 {
		return ""
	}
	absSpeed := math.Abs(float64(speed))
	if absSpeed >= 1 {
		return fmt.Sprintf("%gx", absSpeed)
	}
	return fmt.Sprintf("1/%gx", 1/absSpeed)
}

func drawRuntimeDemoControls(rc renderer.RenderContext, pics picProvider, state runtimeTelemetryState, overlay *runtimeDemoOverlay) {
	if rc == nil || overlay == nil || !state.DemoPlayback || state.DemoBarTimeout < 0 {
		if overlay != nil {
			overlay.showTime = 0
		}
		return
	}
	if state.DemoSpeed != overlay.prevSpeed ||
		state.DemoBaseSpeed != overlay.prevBaseSpeed ||
		math.Abs(float64(state.DemoSpeed)) > math.Abs(float64(state.DemoBaseSpeed)) ||
		state.DemoBarTimeout == 0 {
		overlay.prevSpeed = state.DemoSpeed
		overlay.prevBaseSpeed = state.DemoBaseSpeed
		overlay.showTime = 1
		if state.DemoBarTimeout > 0 {
			overlay.showTime = float64(state.DemoBarTimeout)
		}
	} else {
		overlay.showTime -= state.FrameTime
		if overlay.showTime < 0 {
			overlay.showTime = 0
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

	alpha := currentSbarAlpha()
	drawRuntimeTextBoxAlpha(rc, pics, x-8, y-8, timebarChars, 1, alpha)

	status := ">"
	if state.DemoSpeed == 0 {
		status = "II"
	} else if math.Abs(float64(state.DemoSpeed)) > 1 {
		status = ">>"
	}
	if state.DemoSpeed < 0 {
		status = strings.Repeat("<", len(status))
	}
	drawRuntimeString(rc, x, y, status)

	if base := formatRuntimeDemoBaseSpeed(state.DemoBaseSpeed); base != "" {
		drawRuntimeString(rc, x+(timebarChars-len(base))*8, y, base)
	}
	if state.DemoName != "" {
		drawRuntimeString(rc, 160-len(state.DemoName)*4, y, state.DemoName)
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
	drawRuntimeTextBoxAlpha(rc, pics, timeX-8-(len(timeText)&1)*4, timeY-8, len(timeText)+(len(timeText)&1), 1, alpha)
	drawRuntimeString(rc, timeX, timeY, timeText)
}

func drawRuntimeTurtle(rc renderer.RenderContext, pics picProvider, state runtimeTelemetryState, count *int) {
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
	if turtle := pics.GetPic("turtle"); turtle != nil {
		rc.SetCanvas(renderer.CanvasDefault)
		rc.DrawPic(state.ViewRect.X, state.ViewRect.Y, turtle)
	}
}

func drawRuntimeNet(rc renderer.RenderContext, pics picProvider, state runtimeTelemetryState) {
	if rc == nil || pics == nil || !state.ClientActive || state.DemoPlayback {
		return
	}
	if state.RealTime-state.LastServerMsgAt < 0.3 {
		return
	}
	if netPic := pics.GetPic("net"); netPic != nil {
		rc.SetCanvas(renderer.CanvasDefault)
		rc.DrawPic(state.ViewRect.X+64, state.ViewRect.Y, netPic)
	}
}

func drawRuntimeSavingIndicator(rc renderer.RenderContext, pics picProvider, state runtimeTelemetryState) {
	if rc == nil || pics == nil || !state.SavingActive {
		return
	}
	disc := pics.GetPic("disc")
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

func drawPauseOverlay(dc renderer.RenderContext, pics picProvider) {
	if dc == nil || pics == nil {
		return
	}
	if cv := cvar.Get("showpause"); cv != nil && !cv.Bool() {
		return
	}
	dc.SetCanvas(renderer.CanvasMenu)
	if pause := pics.GetPic("gfx/pause.lmp"); pause != nil {
		dc.DrawMenuPic((320-int(pause.Width))/2, (240-48-int(pause.Height))/2, pause)
	}
}
