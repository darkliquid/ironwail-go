package renderer

import (
	"fmt"
	"math"
)

const (
	HUDClassic = 0
	HUDCount   = 3
)

type ViewRect struct {
	X      int
	Y      int
	Width  int
	Height int
}

type Refdef struct {
	VRect   ViewRect
	BaseFOV float32
	FOVX    float32
	FOVY    float32
	SBLines int
}

type ScreenMetrics struct {
	GLWidth      int
	GLHeight     int
	VidWidth     int
	VidHeight    int
	GUIHeight    int
	ViewSize     float32
	FOV          float32
	FOVAdapt     bool
	ZoomFOV      float32
	Zoom         float32
	SbarScale    float32
	SbarAlpha    float32
	Intermission bool
	HudStyle     int
	CSQCDrawHud  bool
}

func UpdateZoom(zoom, zoomDir, zoomSpeed, oldTime, time float32) (newZoom, newZoomDir float32, recalcRefdef bool) {
	if zoomSpeed <= 0 {
		zoomSpeed = 1e6
	}
	delta := zoomDir * zoomSpeed * (time - oldTime)
	if delta == 0 {
		return zoom, zoomDir, false
	}

	zoom += delta
	switch {
	case zoom >= 1:
		zoom = 1
		zoomDir = 0
	case zoom <= 0:
		zoom = 0
		zoomDir = 0
	}

	return zoom, zoomDir, true
}

func AdaptFovX(fovX, width, height float32, adapt bool) (float32, error) {
	if fovX < 1 || fovX > 179 {
		return 0, fmt.Errorf("bad fov: %f", fovX)
	}
	if !adapt {
		return fovX, nil
	}
	x := height / width
	if x == 0.75 {
		return fovX, nil
	}
	a := math.Atan(0.75 / float64(x) * math.Tan(float64(fovX)/360*math.Pi))
	return float32(a * 360 / math.Pi), nil
}

func CalcFovY(fovX, width, height float32) (float32, error) {
	if fovX < 1 || fovX > 179 {
		return 0, fmt.Errorf("bad fov: %f", fovX)
	}
	x := width / float32(math.Tan(float64(fovX)/360*math.Pi))
	a := math.Atan(float64(height / x))
	return float32(a * 360 / math.Pi), nil
}

func CalcRefdef(m ScreenMetrics) (Refdef, error) {
	if m.GLWidth <= 0 || m.GLHeight <= 0 || m.VidWidth <= 0 || m.VidHeight <= 0 {
		return Refdef{}, fmt.Errorf("invalid dimensions gl=%dx%d vid=%dx%d", m.GLWidth, m.GLHeight, m.VidWidth, m.VidHeight)
	}
	if m.GUIHeight <= 0 {
		return Refdef{}, fmt.Errorf("invalid gui height: %d", m.GUIHeight)
	}

	viewSize := clampf(m.ViewSize, 30, 130)
	fov := clampf(m.FOV, 10, 170)
	zoomFov := clampf(m.ZoomFOV, 10, 170)

	scale := clampf(m.SbarScale, 1, float32(m.GLWidth)/320)
	scale *= float32(m.VidHeight) / float32(m.GUIHeight)

	sbLines := 0
	if !(viewSize >= 120 || m.Intermission || m.SbarAlpha < 1 || m.HudStyle != HUDClassic || m.CSQCDrawHud) {
		if viewSize >= 110 {
			sbLines = int(24 * scale)
		} else {
			sbLines = int(48 * scale)
		}
	}

	size := minf(viewSize, 100) / 100
	vw := maxf(float32(m.GLWidth)*size, 96)
	vh := minInt(int(float32(m.GLHeight)*size), m.GLHeight-sbLines)
	if vh < 0 {
		vh = 0
	}

	vrect := ViewRect{
		Width:  int(vw),
		Height: vh,
	}
	vrect.X = (m.GLWidth - vrect.Width) / 2
	vrect.Y = (m.GLHeight - sbLines - vrect.Height) / 2

	zoom := clampf(m.Zoom, 0, 1)
	zoom = zoom * zoom * (3 - 2*zoom)
	baseFov := lerpf(fov, zoomFov, zoom)
	fovX, err := AdaptFovX(baseFov, float32(m.VidWidth), float32(m.VidHeight), m.FOVAdapt)
	if err != nil {
		return Refdef{}, err
	}
	fovY, err := CalcFovY(fovX, float32(vrect.Width), float32(vrect.Height))
	if err != nil {
		return Refdef{}, err
	}

	return Refdef{
		VRect:   vrect,
		BaseFOV: baseFov,
		FOVX:    fovX,
		FOVY:    fovY,
		SBLines: sbLines,
	}, nil
}

type TileRect struct {
	X int
	Y int
	W int
	H int
}

type TileClearInput struct {
	TileClearUpdates int
	NumPages         int
	GLClear          bool
	VidGamma         float32
	GLWidth          int
	GLHeight         int
	SBLines          int
	VRect            ViewRect
}

type TileClearOutput struct {
	TileClearUpdates int
	Rects            []TileRect
}

func ComputeTileClear(in TileClearInput) TileClearOutput {
	out := TileClearOutput{TileClearUpdates: in.TileClearUpdates}
	if in.TileClearUpdates >= in.NumPages && !in.GLClear && in.VidGamma == 1 {
		return out
	}
	out.TileClearUpdates++

	if in.VRect.X > 0 {
		out.Rects = appendNonEmptyRect(out.Rects,
			TileRect{X: 0, Y: 0, W: in.VRect.X, H: in.GLHeight - in.SBLines},
			TileRect{X: in.VRect.X + in.VRect.Width, Y: 0, W: in.GLWidth - in.VRect.X - in.VRect.Width, H: in.GLHeight - in.SBLines},
		)
	}

	if in.VRect.Y > 0 {
		out.Rects = appendNonEmptyRect(out.Rects,
			TileRect{X: in.VRect.X, Y: 0, W: in.VRect.Width, H: in.VRect.Y},
			TileRect{X: in.VRect.X, Y: in.VRect.Y + in.VRect.Height, W: in.VRect.Width, H: in.GLHeight - in.VRect.Y - in.VRect.Height - in.SBLines},
		)
	}

	return out
}

func appendNonEmptyRect(dst []TileRect, rects ...TileRect) []TileRect {
	for _, r := range rects {
		if r.W > 0 && r.H > 0 {
			dst = append(dst, r)
		}
	}
	return dst
}

func clampf(v, minV, maxV float32) float32 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func lerpf(a, b, t float32) float32 {
	return a + (b-a)*t
}
