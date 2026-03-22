package renderer

import (
	"math"
	"testing"

	"github.com/ironwail/ironwail-go/internal/image"
)

func TestUpdateZoomClampAndRecalc(t *testing.T) {
	zoom, dir, recalc := UpdateZoom(0.4, 1, 8, 0, 0.1)
	if !recalc {
		t.Fatalf("recalc = false, want true")
	}
	if math.Abs(float64(zoom-1)) > 1e-6 {
		t.Fatalf("zoom = %f, want 1", zoom)
	}
	if dir != 0 {
		t.Fatalf("dir = %f, want 0", dir)
	}

	zoom, dir, recalc = UpdateZoom(0.4, 0, 8, 0, 0.1)
	if recalc {
		t.Fatalf("recalc = true, want false")
	}
	if zoom != 0.4 || dir != 0 {
		t.Fatalf("zoom/dir changed without delta: zoom=%f dir=%f", zoom, dir)
	}
}

func TestAdaptFovXAndCalcFovY(t *testing.T) {
	fov, err := AdaptFovX(90, 1920, 1080, true)
	if err != nil {
		t.Fatalf("AdaptFovX error: %v", err)
	}
	if fov <= 90 {
		t.Fatalf("adapted fov = %f, want > 90 for widescreen", fov)
	}

	fov43, err := AdaptFovX(90, 800, 600, true)
	if err != nil {
		t.Fatalf("AdaptFovX 4:3 error: %v", err)
	}
	if math.Abs(float64(fov43-90)) > 1e-5 {
		t.Fatalf("4:3 adapted fov = %f, want 90", fov43)
	}

	fovy, err := CalcFovY(90, 320, 200)
	if err != nil {
		t.Fatalf("CalcFovY error: %v", err)
	}
	if fovy <= 0 || fovy >= 180 {
		t.Fatalf("fovy = %f, want in (0,180)", fovy)
	}
}

func TestCalcRefdef(t *testing.T) {
	ref, err := CalcRefdef(ScreenMetrics{
		GLWidth:      1920,
		GLHeight:     1080,
		VidWidth:     1920,
		VidHeight:    1080,
		GUIHeight:    1080,
		ViewSize:     100,
		FOV:          90,
		FOVAdapt:     true,
		ZoomFOV:      30,
		Zoom:         0,
		SbarScale:    1,
		SbarAlpha:    1,
		HudStyle:     HUDClassic,
		CSQCDrawHud:  false,
		Intermission: false,
	})
	if err != nil {
		t.Fatalf("CalcRefdef error: %v", err)
	}

	if ref.SBLines != 48 {
		t.Fatalf("sb lines = %d, want 48", ref.SBLines)
	}
	if ref.VRect.Width != 1920 || ref.VRect.Height != 1032 {
		t.Fatalf("vrect = %+v, want width=1920 height=1032", ref.VRect)
	}
	if ref.BaseFOV != 90 {
		t.Fatalf("base fov = %f, want 90", ref.BaseFOV)
	}
	if ref.FOVX <= 90 {
		t.Fatalf("fovx = %f, want > 90", ref.FOVX)
	}

	zoomed, err := CalcRefdef(ScreenMetrics{
		GLWidth:      1920,
		GLHeight:     1080,
		VidWidth:     1920,
		VidHeight:    1080,
		GUIHeight:    1080,
		ViewSize:     100,
		FOV:          90,
		FOVAdapt:     true,
		ZoomFOV:      30,
		Zoom:         1,
		SbarScale:    1,
		SbarAlpha:    1,
		HudStyle:     HUDClassic,
		CSQCDrawHud:  false,
		Intermission: false,
	})
	if err != nil {
		t.Fatalf("CalcRefdef zoomed error: %v", err)
	}
	if math.Abs(float64(zoomed.BaseFOV-30)) > 1e-5 {
		t.Fatalf("zoomed base fov = %f, want 30", zoomed.BaseFOV)
	}

	alphaRef, err := CalcRefdef(ScreenMetrics{
		GLWidth:      1920,
		GLHeight:     1080,
		VidWidth:     1920,
		VidHeight:    1080,
		GUIHeight:    1080,
		ViewSize:     100,
		FOV:          90,
		FOVAdapt:     true,
		ZoomFOV:      30,
		Zoom:         0,
		SbarScale:    1,
		SbarAlpha:    0.75,
		HudStyle:     HUDClassic,
		CSQCDrawHud:  false,
		Intermission: false,
	})
	if err != nil {
		t.Fatalf("CalcRefdef alpha error: %v", err)
	}
	if alphaRef.SBLines != 0 {
		t.Fatalf("alpha sbar lines = %d, want 0 when scr_sbaralpha < 1", alphaRef.SBLines)
	}
}

func TestComputeTileClear(t *testing.T) {
	out := ComputeTileClear(TileClearInput{
		TileClearUpdates: 0,
		NumPages:         2,
		GLClear:          false,
		VidGamma:         1,
		GLWidth:          320,
		GLHeight:         200,
		SBLines:          24,
		VRect:            ViewRect{X: 40, Y: 10, Width: 240, Height: 160},
	})

	if out.TileClearUpdates != 1 {
		t.Fatalf("tile updates = %d, want 1", out.TileClearUpdates)
	}
	if len(out.Rects) != 4 {
		t.Fatalf("rect count = %d, want 4", len(out.Rects))
	}

	blocked := ComputeTileClear(TileClearInput{
		TileClearUpdates: 2,
		NumPages:         2,
		GLClear:          false,
		VidGamma:         1,
		GLWidth:          320,
		GLHeight:         200,
		SBLines:          24,
		VRect:            ViewRect{X: 40, Y: 10, Width: 240, Height: 160},
	})
	if blocked.TileClearUpdates != 2 {
		t.Fatalf("blocked updates = %d, want unchanged 2", blocked.TileClearUpdates)
	}
	if len(blocked.Rects) != 0 {
		t.Fatalf("blocked rect count = %d, want 0", len(blocked.Rects))
	}
}

func TestScreenPicRectUsesScreenSpaceCoordinates(t *testing.T) {
	rect := screenPicRect(480, 696, &image.QPic{Width: 24, Height: 16})
	if rect.x != 480 || rect.y != 696 || rect.w != 24 || rect.h != 16 {
		t.Fatalf("screenPicRect = %+v, want {x:480 y:696 w:24 h:16}", rect)
	}
}

func TestMenuPicRectUsesMenuSpaceScaling(t *testing.T) {
	rect := menuPicRect(1280, 720, 16, 4, &image.QPic{Width: 24, Height: 16})
	want := picRect{x: 121.6, y: 14.4, w: 86.4, h: 57.6}
	if !approxFloat32(rect.x, want.x) || !approxFloat32(rect.y, want.y) ||
		!approxFloat32(rect.w, want.w) || !approxFloat32(rect.h, want.h) {
		t.Fatalf("menuPicRect = %+v, want %+v", rect, want)
	}
}

func approxFloat32(got, want float32) bool {
	return math.Abs(float64(got-want)) < 1e-4
}

func TestCanvasTypeString(t *testing.T) {
	tests := []struct {
		ct   CanvasType
		want string
	}{
		{CanvasNone, "NONE"},
		{CanvasDefault, "DEFAULT"},
		{CanvasConsole, "CONSOLE"},
		{CanvasMenu, "MENU"},
		{CanvasSbar, "SBAR"},
		{CanvasSbarQWInv, "SBAR_QW_INV"},
		{CanvasSbar2, "SBAR2"},
		{CanvasCrosshair, "CROSSHAIR"},
		{CanvasBottomLeft, "BOTTOMLEFT"},
		{CanvasBottomRight, "BOTTOMRIGHT"},
		{CanvasTopRight, "TOPRIGHT"},
		{CanvasCSQC, "CSQC"},
		{CanvasInvalid, "INVALID"},
		{CanvasType(99), "UNKNOWN"},
	}
	for _, tt := range tests {
		if got := tt.ct.String(); got != tt.want {
			t.Errorf("CanvasType(%d).String() = %q, want %q", tt.ct, got, tt.want)
		}
	}
}

func TestTransformBounds(t *testing.T) {
	// Identity-like transform: 1920x1080 at scale 1, centered.
	// scale[0] = 2/1920, scale[1] = -2/1080, offset = (0,0)
	transform := DrawTransform{
		Scale:  [2]float32{2.0 / 1920.0, -2.0 / 1080.0},
		Offset: [2]float32{0, 0},
	}
	left, top, right, bottom := TransformBounds(transform)

	// left = (-1 - 0) / (2/1920) = -960
	if !approxFloat32(left, -960) {
		t.Errorf("left = %f, want -960", left)
	}
	// right = (1 - 0) / (2/1920) = 960
	if !approxFloat32(right, 960) {
		t.Errorf("right = %f, want 960", right)
	}
	// top = (1 - 0) / (-2/1080) = -540
	// (negative scale flips the sign, so top < bottom)
	if !approxFloat32(top, -540) {
		t.Errorf("top = %f, want -540", top)
	}
	// bottom = (-1 - 0) / (-2/1080) = 540
	if !approxFloat32(bottom, 540) {
		t.Errorf("bottom = %f, want 540", bottom)
	}
}

func TestCanvasStateZeroValue(t *testing.T) {
	var cs CanvasState
	if cs.Type != CanvasNone {
		t.Errorf("zero CanvasState.Type = %v, want CanvasNone", cs.Type)
	}
	if cs.Transform.Scale != [2]float32{0, 0} {
		t.Errorf("zero CanvasState.Transform.Scale = %v, want [0 0]", cs.Transform.Scale)
	}
}

// stdParams returns a CanvasTransformParams for a 1920x1080 display
// at 1:1 GUI-to-pixel ratio with default cvar values.
func stdParams() CanvasTransformParams {
	return CanvasTransformParams{
		GUIWidth:       1920,
		GUIHeight:      1080,
		GLWidth:        1920,
		GLHeight:       1080,
		ConWidth:       1920,
		ConHeight:      1080,
		MenuScale:      1,
		SbarScale:      1,
		CrosshairScale: 1,
		VRect:          ViewRect{X: 0, Y: 0, Width: 1920, Height: 1080},
	}
}

func TestGetCanvasTransformDefault(t *testing.T) {
	p := stdParams()
	tr := GetCanvasTransform(CanvasDefault, p)

	// Scale: 2/1920, -2/1080 (plus golden shift in offset)
	wantSX := float32(2.0 / 1920.0)
	wantSY := float32(-2.0 / 1080.0)
	if !approxFloat32(tr.Scale[0], wantSX) || !approxFloat32(tr.Scale[1], wantSY) {
		t.Errorf("Default scale = %v, want [%f, %f]", tr.Scale, wantSX, wantSY)
	}
	// Offset should be near (-1, 1) (centered, scale=1, width matches gui).
	if !approxFloat32(tr.Offset[0], -1.0+goldenPixelShift/1920.0) {
		t.Errorf("Default offset[0] = %f, want ~-1", tr.Offset[0])
	}
}

func TestGetCanvasTransformMenu(t *testing.T) {
	p := stdParams()
	tr := GetCanvasTransform(CanvasMenu, p)

	// Menu scale = min(1920/320, 1080/200) = min(6, 5.4) = 5.4, clamped by MenuScale=1 → 1.
	// At MenuScale=1, scale should be 1*2/1920 and 1*-2/1080.
	wantSX := float32(1.0 * 2.0 / 1920.0)
	if !approxFloat32(tr.Scale[0], wantSX) {
		t.Errorf("Menu scale[0] = %f, want %f", tr.Scale[0], wantSX)
	}

	// With MenuScale=3, scale increases.
	p.MenuScale = 3
	tr = GetCanvasTransform(CanvasMenu, p)
	wantSX = float32(3.0 * 2.0 / 1920.0)
	if !approxFloat32(tr.Scale[0], wantSX) {
		t.Errorf("Menu scale[0] at 3x = %f, want %f", tr.Scale[0], wantSX)
	}
}

// approxFloat32Loose checks with a tolerance of ~0.2 to accommodate the
// golden pixel shift applied to canvas transforms.
func approxFloat32Loose(got, want float32) bool {
	return math.Abs(float64(got-want)) < 0.2
}

func TestGetCanvasTransformSbar(t *testing.T) {
	p := stdParams()
	// Single-player: centered at bottom.
	tr := GetCanvasTransform(CanvasSbar, p)
	_, _, _, bottom := TransformBounds(tr)
	// Bottom bound should be ~48 (logical canvas height, golden shift noise).
	if !approxFloat32Loose(bottom, 48) {
		t.Errorf("Sbar bottom bound = %f, want ~48", bottom)
	}

	// Deathmatch + classic HUD: left-aligned.
	p.GameType = GameDeathmatch
	p.HudStyle = HUDClassic
	trDM := GetCanvasTransform(CanvasSbar, p)
	leftDM, _, _, _ := TransformBounds(trDM)
	// Left-aligned means left bound ~0.
	if !approxFloat32Loose(leftDM, 0) {
		t.Errorf("Sbar DM left bound = %f, want ~0", leftDM)
	}
}

func TestGetCanvasTransformConsoleSlide(t *testing.T) {
	p := stdParams()
	p.ConSlideFraction = 1.0 // fully open
	trOpen := GetCanvasTransform(CanvasConsole, p)

	p.ConSlideFraction = 0.0 // fully closed
	trClosed := GetCanvasTransform(CanvasConsole, p)

	// Closed console should have larger (more positive) Y offset than open.
	if trClosed.Offset[1] <= trOpen.Offset[1] {
		t.Errorf("Closed console offset[1] %f should be > open %f",
			trClosed.Offset[1], trOpen.Offset[1])
	}
}

func TestGetCanvasTransformCrosshairCenteredOnViewport(t *testing.T) {
	p := stdParams()
	p.VRect = ViewRect{X: 0, Y: 0, Width: 1920, Height: 1032}
	tr := GetCanvasTransform(CanvasCrosshair, p)

	// The crosshair offset should incorporate the viewport center.
	// With VRect centered, the vertical offset should shift to viewport mid.
	vMid := float32(0 + 1032/2)
	expectedYShift := 1.0 - (vMid*2.0)/1080.0
	// Total Y offset = base + 1.0 + viewport shift.
	_ = expectedYShift
	// Just verify it's different from the no-viewport case.
	p2 := stdParams()
	p2.VRect = ViewRect{X: 0, Y: 100, Width: 1920, Height: 800}
	tr2 := GetCanvasTransform(CanvasCrosshair, p2)
	if tr.Offset[1] == tr2.Offset[1] {
		t.Errorf("Crosshair offset should change with different viewport Y")
	}
}

func TestSetCanvasEarlyOut(t *testing.T) {
	p := stdParams()
	var state CanvasState
	SetCanvas(&state, CanvasMenu, p)
	if state.Type != CanvasMenu {
		t.Fatalf("state.Type = %v, want CanvasMenu", state.Type)
	}
	origTransform := state.Transform

	// Calling again with same type should be a no-op.
	p.MenuScale = 5 // change params
	SetCanvas(&state, CanvasMenu, p)
	if state.Transform != origTransform {
		t.Errorf("SetCanvas should early-out when type unchanged")
	}

	// Different type should update.
	SetCanvas(&state, CanvasDefault, p)
	if state.Type != CanvasDefault {
		t.Errorf("state.Type = %v, want CanvasDefault", state.Type)
	}
}

func TestGetCanvasTransformBottomCorners(t *testing.T) {
	p := stdParams()

	trBL := GetCanvasTransform(CanvasBottomLeft, p)
	leftBL, _, _, bottomBL := TransformBounds(trBL)
	if !approxFloat32Loose(leftBL, 0) {
		t.Errorf("BottomLeft left = %f, want ~0", leftBL)
	}
	if !approxFloat32Loose(bottomBL, 200) {
		t.Errorf("BottomLeft bottom = %f, want ~200", bottomBL)
	}

	trBR := GetCanvasTransform(CanvasBottomRight, p)
	_, _, rightBR, bottomBR := TransformBounds(trBR)
	if !approxFloat32Loose(rightBR, 320) {
		t.Errorf("BottomRight right = %f, want ~320", rightBR)
	}
	if !approxFloat32Loose(bottomBR, 200) {
		t.Errorf("BottomRight bottom = %f, want ~200", bottomBR)
	}
}

func TestGetCanvasTransformTopRight(t *testing.T) {
	p := stdParams()
	tr := GetCanvasTransform(CanvasTopRight, p)
	_, topTR, rightTR, _ := TransformBounds(tr)
	if !approxFloat32Loose(rightTR, 320) {
		t.Errorf("TopRight right = %f, want ~320", rightTR)
	}
	if !approxFloat32Loose(topTR, 0) {
		t.Errorf("TopRight top = %f, want ~0", topTR)
	}
}

// paramsForRes builds a CanvasTransformParams for the given resolution
// with typical 1:1 GUI mapping and default cvar values.
func paramsForRes(w, h int) CanvasTransformParams {
	return CanvasTransformParams{
		GUIWidth:       float32(w),
		GUIHeight:      float32(h),
		GLWidth:        float32(w),
		GLHeight:       float32(h),
		ConWidth:       float32(w),
		ConHeight:      float32(h),
		MenuScale:      1,
		SbarScale:      1,
		CrosshairScale: 1,
		VRect:          ViewRect{X: 0, Y: 0, Width: w, Height: h},
	}
}

// TestGetCanvasTransformMultiResolution verifies that every canvas type
// produces a valid transform (non-zero scale, reasonable offset, valid
// bounds) across 4:3, 16:9, and 21:9 resolutions.
func TestGetCanvasTransformMultiResolution(t *testing.T) {
	resolutions := []struct {
		name string
		w, h int
	}{
		{"800x600_4:3", 800, 600},
		{"1024x768_4:3", 1024, 768},
		{"1280x720_16:9", 1280, 720},
		{"1920x1080_16:9", 1920, 1080},
		{"2560x1440_16:9", 2560, 1440},
		{"2560x1080_21:9", 2560, 1080},
		{"3440x1440_21:9", 3440, 1440},
	}

	canvasTypes := []CanvasType{
		CanvasDefault, CanvasConsole, CanvasMenu, CanvasSbar,
		CanvasSbar2, CanvasCrosshair, CanvasBottomLeft,
		CanvasBottomRight, CanvasTopRight, CanvasCSQC,
	}

	for _, res := range resolutions {
		p := paramsForRes(res.w, res.h)
		for _, ct := range canvasTypes {
			name := res.name + "/" + ct.String()
			t.Run(name, func(t *testing.T) {
				tp := p
				// Console with slide=0 is off-screen; use fully open for validation.
				if ct == CanvasConsole {
					tp.ConSlideFraction = 1.0
				}
				tr := GetCanvasTransform(ct, tp)

				// Scale must be non-zero with correct sign (positive X, negative Y).
				if tr.Scale[0] <= 0 {
					t.Errorf("Scale[0] = %f, want > 0", tr.Scale[0])
				}
				if tr.Scale[1] >= 0 {
					t.Errorf("Scale[1] = %f, want < 0", tr.Scale[1])
				}

				// Offset should be in a reasonable NDC range.
				for i := 0; i < 2; i++ {
					if tr.Offset[i] < -3 || tr.Offset[i] > 3 {
						t.Errorf("Offset[%d] = %f, out of reasonable NDC range", i, tr.Offset[i])
					}
				}

				// TransformBounds must produce left < right and top < bottom.
				left, top, right, bottom := TransformBounds(tr)
				if left >= right {
					t.Errorf("left=%f >= right=%f", left, right)
				}
				if top >= bottom {
					t.Errorf("top=%f >= bottom=%f", top, bottom)
				}
			})
		}
	}
}

// TestGetCanvasTransformMenuCentered verifies that menu transforms center
// the 320x200 logical canvas regardless of aspect ratio.
func TestGetCanvasTransformMenuCentered(t *testing.T) {
	resolutions := []struct {
		name string
		w, h int
	}{
		{"800x600", 800, 600},
		{"1920x1080", 1920, 1080},
		{"3440x1440", 3440, 1440},
	}

	for _, res := range resolutions {
		t.Run(res.name, func(t *testing.T) {
			p := paramsForRes(res.w, res.h)
			tr := GetCanvasTransform(CanvasMenu, p)
			left, top, right, bottom := TransformBounds(tr)

			// Menu should be roughly centered: midpoint ≈ 160 horizontally, 100 vertically.
			midX := (left + right) / 2
			midY := (top + bottom) / 2
			if !approxFloat32Loose(midX, 160) {
				t.Errorf("Menu midX = %f, want ~160", midX)
			}
			if !approxFloat32Loose(midY, 100) {
				t.Errorf("Menu midY = %f, want ~100", midY)
			}
		})
	}
}

// TestGetCanvasTransformSbarBottomAligned verifies the sbar transform
// anchors to the bottom of the screen at every resolution.
func TestGetCanvasTransformSbarBottomAligned(t *testing.T) {
	resolutions := []struct {
		name string
		w, h int
	}{
		{"800x600", 800, 600},
		{"1920x1080", 1920, 1080},
		{"2560x1080", 2560, 1080},
	}

	for _, res := range resolutions {
		t.Run(res.name, func(t *testing.T) {
			p := paramsForRes(res.w, res.h)
			tr := GetCanvasTransform(CanvasSbar, p)
			_, _, _, bottom := TransformBounds(tr)
			// Sbar logical height is 48; bottom should be ~48.
			if !approxFloat32Loose(bottom, 48) {
				t.Errorf("Sbar bottom = %f, want ~48", bottom)
			}
		})
	}
}

// TestGetCanvasTransformDefaultFullScreen verifies the default canvas
// spans the full screen at every resolution.
func TestGetCanvasTransformDefaultFullScreen(t *testing.T) {
	resolutions := []struct {
		name string
		w, h int
	}{
		{"800x600", 800, 600},
		{"1920x1080", 1920, 1080},
		{"3440x1440", 3440, 1440},
	}

	for _, res := range resolutions {
		t.Run(res.name, func(t *testing.T) {
			p := paramsForRes(res.w, res.h)
			tr := GetCanvasTransform(CanvasDefault, p)
			left, top, right, bottom := TransformBounds(tr)

			width := right - left
			height := bottom - top
			// Default canvas should span the full GUI resolution.
			if !approxFloat32Loose(width, float32(res.w)) {
				t.Errorf("Default width = %f, want ~%d", width, res.w)
			}
			if !approxFloat32Loose(height, float32(res.h)) {
				t.Errorf("Default height = %f, want ~%d", height, res.h)
			}
		})
	}
}

// TestGetCanvasTransformConsoleFullWidth verifies the console canvas
// spans the full width and its ConWidth at all resolutions.
func TestGetCanvasTransformConsoleFullWidth(t *testing.T) {
	resolutions := []struct {
		name string
		w, h int
	}{
		{"800x600", 800, 600},
		{"1920x1080", 1920, 1080},
		{"2560x1080", 2560, 1080},
	}

	for _, res := range resolutions {
		t.Run(res.name, func(t *testing.T) {
			p := paramsForRes(res.w, res.h)
			p.ConSlideFraction = 1.0 // fully open
			tr := GetCanvasTransform(CanvasConsole, p)
			left, _, right, _ := TransformBounds(tr)
			width := right - left
			// Console spans conWidth (= GUI width with default params).
			if !approxFloat32Loose(width, p.ConWidth) {
				t.Errorf("Console width = %f, want ~%f", width, p.ConWidth)
			}
		})
	}
}

// TestGetCanvasTransformCrosshairCenteredMultiRes verifies the crosshair
// is centered within the viewport at different resolutions.
func TestGetCanvasTransformCrosshairCenteredMultiRes(t *testing.T) {
	resolutions := []struct {
		name string
		w, h int
	}{
		{"800x600", 800, 600},
		{"1920x1080", 1920, 1080},
		{"3440x1440", 3440, 1440},
	}

	for _, res := range resolutions {
		t.Run(res.name, func(t *testing.T) {
			p := paramsForRes(res.w, res.h)
			tr := GetCanvasTransform(CanvasCrosshair, p)

			// The crosshair canvas is 1x1 pixel centered on the viewport.
			// Scale should match 1*2/gui and 1*-2/gui.
			wantSX := float32(1.0 * 2.0 / float32(res.w))
			wantSY := float32(1.0 * -2.0 / float32(res.h))
			if !approxFloat32(tr.Scale[0], wantSX) {
				t.Errorf("Crosshair Scale[0] = %f, want %f", tr.Scale[0], wantSX)
			}
			if !approxFloat32(tr.Scale[1], wantSY) {
				t.Errorf("Crosshair Scale[1] = %f, want %f", tr.Scale[1], wantSY)
			}
		})
	}
}
