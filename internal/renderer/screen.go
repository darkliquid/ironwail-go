package renderer

import (
	"fmt"
	"math"
)

const (
	HUDClassic = 0
	HUDCount   = 3
)

// CanvasType identifies the coordinate space used for 2D drawing calls.
// Each canvas type defines its own logical dimensions, scale factor, and
// alignment within the physical viewport. This matches the C Ironwail
// canvastype enum from screen.h.
type CanvasType int

const (
	// CanvasNone means no canvas is active; drawing is undefined.
	CanvasNone CanvasType = iota
	// CanvasDefault is the full-screen GUI canvas at native resolution.
	CanvasDefault
	// CanvasConsole is the console overlay, scaled by scr_conwidth/scr_conscale.
	CanvasConsole
	// CanvasMenu is the 320x200 menu canvas, scaled by scr_menuscale.
	CanvasMenu
	// CanvasSbar is the 320x48 status bar, bottom-aligned, scaled by scr_sbarscale.
	CanvasSbar
	// CanvasSbarQWInv is the 48x48 QuakeWorld inventory, bottom-right-aligned.
	CanvasSbarQWInv
	// CanvasSbar2 is the modern HUD canvas at 400x225 base, centered.
	CanvasSbar2
	// CanvasCrosshair is centered on the viewport midpoint, scaled by scr_crosshairscale.
	CanvasCrosshair
	// CanvasBottomLeft is the 320x200 bottom-left corner (dev stats display).
	CanvasBottomLeft
	// CanvasBottomRight is the 320x200 bottom-right corner (FPS/speed display).
	CanvasBottomRight
	// CanvasTopRight is the 320x200 top-right corner (disc icon).
	CanvasTopRight
	// CanvasCSQC is the client-side QuakeC drawing canvas, using scr_sbarscale.
	CanvasCSQC

	// CanvasInvalid is a sentinel for error states.
	CanvasInvalid CanvasType = -1
)

// String returns the human-readable name of a CanvasType for logging and debugging.
func (c CanvasType) String() string {
	switch c {
	case CanvasNone:
		return "NONE"
	case CanvasDefault:
		return "DEFAULT"
	case CanvasConsole:
		return "CONSOLE"
	case CanvasMenu:
		return "MENU"
	case CanvasSbar:
		return "SBAR"
	case CanvasSbarQWInv:
		return "SBAR_QW_INV"
	case CanvasSbar2:
		return "SBAR2"
	case CanvasCrosshair:
		return "CROSSHAIR"
	case CanvasBottomLeft:
		return "BOTTOMLEFT"
	case CanvasBottomRight:
		return "BOTTOMRIGHT"
	case CanvasTopRight:
		return "TOPRIGHT"
	case CanvasCSQC:
		return "CSQC"
	case CanvasInvalid:
		return "INVALID"
	default:
		return "UNKNOWN"
	}
}

// Canvas alignment constants control how a canvas is positioned within
// the physical viewport. They correspond to the C CANVAS_ALIGN_* macros
// in gl_draw.c and are used as fractional multipliers (0.0 = left/top,
// 0.5 = center, 1.0 = right/bottom).
const (
	CanvasAlignLeft    = 0.0
	CanvasAlignCenterX = 0.5
	CanvasAlignRight   = 1.0
	CanvasAlignTop     = 0.0
	CanvasAlignCenterY = 0.5
	CanvasAlignBottom  = 1.0
)

// DrawTransform maps canvas-space pixel coordinates to normalised device
// coordinates (NDC, -1 to +1). Each canvas type produces a different
// transform depending on its logical size, scale cvar, and alignment.
//
// Vertex transformation:  ndc = vertex * Scale + Offset
//
// Scale[0] converts canvas X pixels to NDC width.
// Scale[1] converts canvas Y pixels to NDC height (negative for top-down Y).
// Offset[0] shifts the canvas horizontally within the viewport.
// Offset[1] shifts the canvas vertically within the viewport.
type DrawTransform struct {
	Scale  [2]float32
	Offset [2]float32
}

// CanvasState tracks the active 2D drawing canvas. It caches the current
// canvas type and its computed transform so redundant GL_SetCanvas calls
// are skipped (matching C Ironwail's early-out in GL_SetCanvas).
type CanvasState struct {
	Type      CanvasType
	Transform DrawTransform
	// Left, Top, Right, Bottom are the canvas-space clipping bounds
	// derived from the transform. They define the drawable area in
	// canvas coordinates.
	Left   float32
	Top    float32
	Right  float32
	Bottom float32
}

// TransformBounds computes canvas-space clipping bounds from a DrawTransform.
// These bounds define the visible rectangle in canvas coordinates and are
// used for text wrapping and element positioning. Matches C Ironwail's
// Draw_GetTransformBounds in gl_draw.c.
func TransformBounds(t DrawTransform) (left, top, right, bottom float32) {
	left = (-1 - t.Offset[0]) / t.Scale[0]
	right = (1 - t.Offset[0]) / t.Scale[0]
	bottom = (-1 - t.Offset[1]) / t.Scale[1]
	top = (1 - t.Offset[1]) / t.Scale[1]
	return
}

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
	GLWidth        int
	GLHeight       int
	VidWidth       int
	VidHeight      int
	GUIWidth       int // GUI-space width (physical pixels, matches C vid.guiwidth)
	GUIHeight      int
	ConWidth       int // Console logical width (controlled by scr_conwidth/scr_conscale)
	ConHeight      int // Console logical height
	ViewSize       float32
	FOV            float32
	FOVAdapt       bool
	ZoomFOV        float32
	Zoom           float32
	SbarScale      float32
	SbarAlpha      float32
	MenuScale      float32 // scr_menuscale cvar value
	CrosshairScale float32 // scr_crosshairscale cvar value
	Intermission   bool
	HudStyle       int
	CSQCDrawHud    bool
}

// UpdateZoom updates zoom-derived field-of-view values so scoped views change perspective consistently while keeping aspect-correct horizontal/vertical FOV math.
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

// AdaptFovX remaps horizontal FOV to the current aspect ratio, preserving gameplay feel across widescreen and legacy resolutions.
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

// CalcFovY derives vertical FOV from horizontal FOV and viewport geometry, ensuring projection matrices keep object scale consistent.
func CalcFovY(fovX, width, height float32) (float32, error) {
	if fovX < 1 || fovX > 179 {
		return 0, fmt.Errorf("bad fov: %f", fovX)
	}
	x := width / float32(math.Tan(float64(fovX)/360*math.Pi))
	a := math.Atan(float64(height / x))
	return float32(a * 360 / math.Pi), nil
}

// CalcRefdef computes final per-frame view rectangles and projection parameters, the bridge between UI layout decisions and 3D camera setup.
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

// ComputeTileClear determines which screen areas need tile background redraw when the 3D view does not cover the full window.
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

// appendNonEmptyRect appends clip rectangles only when they have positive area, avoiding useless clear/draw work.
func appendNonEmptyRect(dst []TileRect, rects ...TileRect) []TileRect {
	for _, r := range rects {
		if r.W > 0 && r.H > 0 {
			dst = append(dst, r)
		}
	}
	return dst
}

// clampf bounds scalar values to a safe range used by FOV and blend calculations that must stay numerically stable.
func clampf(v, minV, maxV float32) float32 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

// minf returns the smaller scalar and is used in viewport and effect calculations where conservative bounds are required.
func minf(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

// maxf performs its step in this part of the renderer; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func maxf(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

// minInt performs its step in this part of the renderer; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// lerpf performs its step in this part of the renderer; this helper exists to keep the frame pipeline deterministic and easier to reason about for engine learners.
func lerpf(a, b, t float32) float32 {
	return a + (b-a)*t
}
