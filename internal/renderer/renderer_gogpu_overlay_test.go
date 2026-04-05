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

func TestOverlayBlitConcharsStringMatchesRGBAPath(t *testing.T) {
	palette := make([]byte, 256*3)
	palette[3] = 10
	palette[4] = 20
	palette[5] = 30
	conchars := make([]byte, 128*128)
	// Character 1, row 0: a 2x2 opaque block in the top-left corner.
	conchars[8] = 1
	conchars[9] = 1
	conchars[128+8] = 1
	conchars[128+9] = 1
	text := []byte{1}

	got := &overlay2D{
		pixels: make([]byte, 4*4*4),
		width:  4,
		height: 4,
	}
	got.blitConcharsString(conchars, palette, text, 0, 0, 4, 4)

	want := &overlay2D{
		pixels: make([]byte, 4*4*4),
		width:  4,
		height: 4,
	}
	rgba := ConvertConcharsToRGBA([]byte{
		1, 1, 0, 0, 0, 0, 0, 0,
		1, 1, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
		0, 0, 0, 0, 0, 0, 0, 0,
	}, palette)
	want.blitRGBA(rgba, 8, 8, 0, 0, 4, 4, 1)

	for i := range want.pixels {
		if got.pixels[i] != want.pixels[i] {
			t.Fatalf("pixel[%d] = %d, want %d", i, got.pixels[i], want.pixels[i])
		}
	}
	if got.dirtyRect() != (overlayDirtyRect{x: 0, y: 0, w: 4, h: 4}) {
		t.Fatalf("dirtyRect = %+v, want full 4x4", got.dirtyRect())
	}
}
