//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import "testing"

func TestOpenGLSetCanvasPreservesExplicitCanvasParams(t *testing.T) {
	dc := &glDrawContext{}
	dc.viewport.width = 1280
	dc.viewport.height = 720

	params := CanvasTransformParams{
		GUIWidth:  853,
		GUIHeight: 720,
		GLWidth:   1280,
		GLHeight:  720,
		ConWidth:  640,
		ConHeight: 540,
		MenuScale: 2.25,
	}
	dc.SetCanvasParams(params)
	dc.SetCanvas(CanvasMenu)

	want := GetCanvasTransform(CanvasMenu, params)
	got := dc.Canvas()
	if got.Transform != want {
		t.Fatalf("OpenGL menu transform = %+v, want %+v", got.Transform, want)
	}
}
