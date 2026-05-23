package game

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

func TestDrawRuntimeClockAndFPSUseBottomRightCanvasForClassicHUD(t *testing.T) {
	g := New()
	dc := &telemetryOverlayDrawContext{}
	state := TelemetryState{
		RealTime:   1,
		FrameCount: 100,
		ViewSize:   100,
		HUDStyle:   renderer.HUDClassic,
		ShowFPS:    1,
		ShowClock:  1,
		ClientTime: 125,
	}
	fps := &FPSOverlay{}

	g.drawRuntimeClock(dc, state)
	g.drawRuntimeFPS(dc, state, fps)

	if len(dc.chars) != len("2:05")+len(" 100 fps") {
		t.Fatalf("char count = %d, want %d", len(dc.chars), len("2:05")+len(" 100 fps"))
	}
	if got := dc.chars[0]; got.canvas != renderer.CanvasBottomRight || got.x != 288 || got.y != 192 || got.num != '2' {
		t.Fatalf("clock first char = %+v, want bottom-right at 288,192 with '2'", got)
	}
	if got := dc.chars[len("2:05")]; got.canvas != renderer.CanvasBottomRight || got.x != 256 || got.y != 184 {
		t.Fatalf("fps first char = %+v, want bottom-right at 256,184", got)
	}
}

func TestDrawRuntimeFPSUsesMillisecondsModeForShowFPS2(t *testing.T) {
	g := New()
	dc := &telemetryOverlayDrawContext{}
	state := TelemetryState{
		RealTime:   1,
		FrameCount: 100,
		ViewSize:   100,
		HUDStyle:   renderer.HUDClassic,
		ShowFPS:    2,
	}
	fps := &FPSOverlay{}

	g.drawRuntimeFPS(dc, state, fps)

	if len(dc.chars) != len("10.00 ms") {
		t.Fatalf("char count = %d, want %d", len(dc.chars), len("10.00 ms"))
	}
	got := string(charsToRunes(dc.chars))
	if got != "10.00 ms" {
		t.Fatalf("fps text = %q, want %q", got, "10.00 ms")
	}
}

func TestDrawRuntimeSpeedUsesCrosshairCanvas(t *testing.T) {
	g := New()
	dc := &telemetryOverlayDrawContext{}
	state := TelemetryState{
		RealTime:     0.05,
		ViewSize:     100,
		ShowSpeed:    true,
		ShowSpeedOfs: 10,
		Velocity:     [3]float32{300, 400, 200},
	}
	speed := &SpeedOverlay{}

	g.drawRuntimeSpeed(dc, state, speed)
	state.RealTime = 0.10
	state.Velocity = [3]float32{}
	g.drawRuntimeSpeed(dc, state, speed)

	if len(dc.chars) != len("500") {
		t.Fatalf("char count = %d, want %d", len(dc.chars), len("500"))
	}
	if got := dc.chars[0]; got.canvas != renderer.CanvasCrosshair || got.x != -12 || got.y != 14 || got.num != '5' {
		t.Fatalf("speed first char = %+v, want crosshair at -12,14 with '5'", got)
	}
}

func TestDrawRuntimeTurtleUsesViewportOriginAfterThreeSlowFrames(t *testing.T) {
	g := New()
	dc := &telemetryOverlayDrawContext{}
	pics := overlayTestPics{pics: map[string]*qimage.QPic{"turtle": {Width: 16, Height: 16}}}
	state := TelemetryState{
		ShowTurtle: true,
		FrameTime:  0.1,
		ViewRect:   renderer.ViewRect{X: 12, Y: 34, Width: 200, Height: 100},
	}
	count := 0

	g.drawRuntimeTurtle(dc, pics, state, &count)
	g.drawRuntimeTurtle(dc, pics, state, &count)
	g.drawRuntimeTurtle(dc, pics, state, &count)

	if len(dc.pics) != 1 {
		t.Fatalf("pic count = %d, want 1", len(dc.pics))
	}
	if got := dc.pics[0]; got.canvas != renderer.CanvasDefault || got.x != 12 || got.y != 34 {
		t.Fatalf("turtle draw = %+v, want default canvas at 12,34", got)
	}
}

func TestDrawRuntimeNetUsesViewportOffsetAfterLag(t *testing.T) {
	g := New()
	dc := &telemetryOverlayDrawContext{}
	pics := overlayTestPics{pics: map[string]*qimage.QPic{"net": {Width: 16, Height: 16}}}
	state := TelemetryState{
		RealTime:        10,
		LastServerMsgAt: 9.6,
		ClientActive:    true,
		ViewRect:        renderer.ViewRect{X: 20, Y: 40, Width: 200, Height: 100},
	}

	g.drawRuntimeNet(dc, pics, state)

	if len(dc.pics) != 1 {
		t.Fatalf("pic count = %d, want 1", len(dc.pics))
	}
	if got := dc.pics[0]; got.canvas != renderer.CanvasDefault || got.x != 84 || got.y != 40 {
		t.Fatalf("net draw = %+v, want default canvas at 84,40", got)
	}
}

func TestDrawRuntimeSavingIndicatorUsesTopRightOffset(t *testing.T) {
	g := New()
	dc := &telemetryOverlayDrawContext{}
	pics := overlayTestPics{pics: map[string]*qimage.QPic{"disc": {Width: 24, Height: 24}}}
	state := TelemetryState{
		SavingActive: true,
		HUDStyle:     renderer.HUDCompact,
		ViewSize:     100,
		ShowClock:    1,
		ShowFPS:      1,
	}

	g.drawRuntimeSavingIndicator(dc, pics, state)

	if len(dc.pics) != 1 {
		t.Fatalf("pic count = %d, want 1", len(dc.pics))
	}
	if got := dc.pics[0]; got.canvas != renderer.CanvasTopRight || got.x != 280 || got.y != 32 {
		t.Fatalf("saving draw = %+v, want top-right canvas at 280,32", got)
	}
}

func TestDrawRuntimeDemoControlsUsesSbarCanvas(t *testing.T) {
	g := New()
	g.Host.CVar.Register("scr_demobar_timeout", "1", cvar.FlagArchive, "")
	dc := &telemetryOverlayDrawContext{}
	state := TelemetryState{
		DemoPlayback:   true,
		DemoSpeed:      1,
		DemoBaseSpeed:  1,
		DemoProgress:   0.5,
		DemoName:       "demo1",
		DemoBarTimeout: 1,
		ClientTime:     125,
		FrameTime:      0.1,
	}
	overlay := &DemoOverlay{}

	g.drawRuntimeDemoControls(dc, nil, state, overlay)

	if len(dc.chars) == 0 {
		t.Fatal("expected demo control characters")
	}
	if got := dc.chars[0]; got.canvas != renderer.CanvasSbar || got.x != 8 || got.y != -20 || got.num != 13 {
		t.Fatalf("first demo control char = %+v, want sbar canvas at 8,-20 with glyph 13", got)
	}
	foundCursor := false
	for _, ch := range dc.chars {
		if ch.canvas == renderer.CanvasSbar && ch.num == 131 {
			foundCursor = true
			break
		}
	}
	if !foundCursor {
		t.Fatal("expected demo seek cursor character")
	}
}

func TestDrawRuntimeDemoControlsTimeoutHidesOverlay(t *testing.T) {
	g := New()
	g.Host.CVar.Register("scr_demobar_timeout", "1", cvar.FlagArchive, "")
	state := TelemetryState{
		DemoPlayback:   true,
		DemoSpeed:      1,
		DemoBaseSpeed:  1,
		DemoProgress:   0.25,
		DemoName:       "demo1",
		DemoBarTimeout: 1,
		ClientTime:     10,
		FrameTime:      0.1,
	}
	overlay := &DemoOverlay{}
	g.drawRuntimeDemoControls(&telemetryOverlayDrawContext{}, nil, state, overlay)

	dc := &telemetryOverlayDrawContext{}
	state.FrameTime = 1.1
	g.drawRuntimeDemoControls(dc, nil, state, overlay)

	if len(dc.chars) != 0 || len(dc.pics) != 0 {
		t.Fatalf("expected timed-out demo overlay to draw nothing, got chars=%d pics=%d", len(dc.chars), len(dc.pics))
	}
}

func TestDrawRuntimeDemoControlsUsesMenuCanvasDuringIntermission(t *testing.T) {
	g := New()
	g.Host.CVar.Register("scr_demobar_timeout", "1", cvar.FlagArchive, "")
	dc := &telemetryOverlayDrawContext{}
	state := TelemetryState{
		DemoPlayback:   true,
		DemoSpeed:      0,
		DemoBaseSpeed:  1,
		DemoProgress:   0.5,
		DemoName:       "demo1",
		DemoBarTimeout: 1,
		ClientTime:     5,
		FrameTime:      0.1,
		Intermission:   1,
	}
	overlay := &DemoOverlay{}

	g.drawRuntimeDemoControls(dc, nil, state, overlay)

	if len(dc.chars) == 0 {
		t.Fatal("expected intermission demo control characters")
	}
	if got := dc.chars[0]; got.canvas != renderer.CanvasMenu || got.y != 25 || got.num != 'I' {
		t.Fatalf("first intermission demo control char = %+v, want menu canvas at y=25 with 'I'", got)
	}
}
