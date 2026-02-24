package renderer

import (
	"math"
	"testing"
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
