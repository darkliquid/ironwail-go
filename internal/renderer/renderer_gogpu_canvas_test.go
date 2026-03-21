//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import "testing"

func TestGoGPUCanvasRectToScreenUsesSbarCanvasTransform(t *testing.T) {
	dc := &DrawContext{}
	dc.SetCanvasParams(CanvasTransformParams{
		GUIWidth:  640,
		GUIHeight: 480,
		GLWidth:   640,
		GLHeight:  480,
		ConWidth:  640,
		ConHeight: 480,
		SbarScale: 1,
	})
	dc.SetCanvas(CanvasSbar)

	x, y, w, h := dc.canvasRectToScreen(0, 0, 320, 48)
	if x != 160 || y != 431 || w != 320 || h != 48 {
		t.Fatalf("canvasRectToScreen(SBAR) = (%d,%d %dx%d), want (160,431 320x48)", x, y, w, h)
	}
}

func TestGoGPUCanvasRectToScreenFallsBackWithoutCanvasTransform(t *testing.T) {
	dc := &DrawContext{}

	x, y, w, h := dc.canvasRectToScreen(12, 34, 56, 78)
	if x != 12 || y != 34 || w != 56 || h != 78 {
		t.Fatalf("canvasRectToScreen(raw) = (%d,%d %dx%d), want (12,34 56x78)", x, y, w, h)
	}
}
