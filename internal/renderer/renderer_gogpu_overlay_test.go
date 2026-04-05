//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import "testing"

func TestOverlayDirtyRectUnion(t *testing.T) {
	a := overlayDirtyRect{x: 10, y: 20, w: 30, h: 40}
	b := overlayDirtyRect{x: 25, y: 5, w: 10, h: 30}
	got := unionOverlayDirtyRects(a, b)
	if got != (overlayDirtyRect{x: 10, y: 5, w: 30, h: 55}) {
		t.Fatalf("unionOverlayDirtyRects = %+v, want {x:10 y:5 w:30 h:55}", got)
	}
}

func TestOverlayMarkDirtyRectTracksBounds(t *testing.T) {
	ov := &overlay2D{width: 320, height: 200}
	ov.markDirtyRect(20, 30, 40, 10)
	ov.markDirtyRect(5, 50, 10, 15)
	if got := ov.dirtyRect(); got != (overlayDirtyRect{x: 5, y: 30, w: 55, h: 35}) {
		t.Fatalf("dirtyRect = %+v, want {x:5 y:30 w:55 h:35}", got)
	}
}

func TestOverlayUploadRegionPixelsLocked(t *testing.T) {
	srcWidth := 4
	src := []byte{
		0, 0, 0, 0, 1, 0, 0, 0, 2, 0, 0, 0, 3, 0, 0, 0,
		4, 0, 0, 0, 5, 0, 0, 0, 6, 0, 0, 0, 7, 0, 0, 0,
		8, 0, 0, 0, 9, 0, 0, 0, 10, 0, 0, 0, 11, 0, 0, 0,
	}
	r := &Renderer{}
	got := r.overlayUploadRegionPixelsLocked(src, srcWidth, overlayDirtyRect{x: 1, y: 1, w: 2, h: 2})
	want := []byte{
		5, 0, 0, 0, 6, 0, 0, 0,
		9, 0, 0, 0, 10, 0, 0, 0,
	}
	if len(got) != len(want) {
		t.Fatalf("len(got) = %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}
