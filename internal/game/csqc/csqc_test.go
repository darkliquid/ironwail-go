package csqc

import (
	"testing"

	qimage "github.com/darkliquid/ironwail-go/internal/image"
)

func TestNearestPaletteIndex(t *testing.T) {
	palette := []byte{
		255, 0, 0, // index 0: red
		0, 255, 0, // index 1: green
		0, 0, 255, // index 2: blue
	}
	if got := NearestPaletteIndex(1, 0, 0, palette); got != 0 {
		t.Fatalf("NearestPaletteIndex(red) = %d, want 0", got)
	}
	if got := NearestPaletteIndex(0, 1, 0, palette); got != 1 {
		t.Fatalf("NearestPaletteIndex(green) = %d, want 1", got)
	}
	if got := NearestPaletteIndex(0, 0, 1, palette); got != 2 {
		t.Fatalf("NearestPaletteIndex(blue) = %d, want 2", got)
	}
	// Clamped out-of-range values still resolve to the closest entry.
	if got := NearestPaletteIndex(2, 0, 0, palette); got != 0 {
		t.Fatalf("NearestPaletteIndex(over-range red) = %d, want 0", got)
	}
}

func TestClipDrawRectDisabled(t *testing.T) {
	dx, dy, dw, dh, sx, sy, sw, sh, ok := ClipDrawRect(ClipRect{}, 10, 20, 30, 40)
	if !ok || dx != 10 || dy != 20 || dw != 30 || dh != 40 {
		t.Fatalf("disabled clip = (%v,%v,%v,%v) ok=%v, want (10,20,30,40) true", dx, dy, dw, dh, ok)
	}
	if sx != 0 || sy != 0 || sw != 1 || sh != 1 {
		t.Fatalf("disabled clip src = (%v,%v,%v,%v), want (0,0,1,1)", sx, sy, sw, sh)
	}
}

func TestClipDrawRectClips(t *testing.T) {
	clip := ClipRect{Enabled: true, X: 0, Y: 0, Width: 20, Height: 20}
	dx, dy, dw, dh, sx, sy, sw, sh, ok := ClipDrawRect(clip, 10, 10, 30, 30)
	if !ok {
		t.Fatal("expected clip to succeed")
	}
	if dx != 10 || dy != 10 || dw != 10 || dh != 10 {
		t.Fatalf("clipped rect = (%v,%v,%v,%v), want (10,10,10,10)", dx, dy, dw, dh)
	}
	if sx != 0 || sy != 0 {
		t.Fatalf("src origin = (%v,%v), want (0,0)", sx, sy)
	}
	if sw < 0.32 || sw > 0.34 || sh < 0.32 || sh > 0.34 {
		t.Fatalf("src size = (%v,%v), want ~(1/3,1/3)", sw, sh)
	}
}

func TestClipDrawRectFullyOutside(t *testing.T) {
	clip := ClipRect{Enabled: true, X: 0, Y: 0, Width: 10, Height: 10}
	if _, _, _, _, _, _, _, _, ok := ClipDrawRect(clip, 100, 100, 10, 10); ok {
		t.Fatal("expected fully-outside clip to report not-ok")
	}
}

func TestScaleQPicNearestNeighbor(t *testing.T) {
	pic := &qimage.QPic{Width: 2, Height: 1, Pixels: []byte{1, 2}}
	scaled := ScaleQPic(pic, 4, 2)
	if scaled == nil || scaled.Width != 4 || scaled.Height != 2 {
		t.Fatalf("scaled pic = %+v, want 4x2", scaled)
	}
	// Nearest-neighbor upscale duplicates each source pixel twice per axis.
	if scaled.Pixels[0] != 1 || scaled.Pixels[1] != 1 || scaled.Pixels[2] != 2 || scaled.Pixels[3] != 2 {
		t.Fatalf("scaled row 0 = %v, want [1 1 2 2]", scaled.Pixels[0:4])
	}
	if scaled.Pixels[4] != 1 || scaled.Pixels[5] != 1 || scaled.Pixels[6] != 2 || scaled.Pixels[7] != 2 {
		t.Fatalf("scaled row 1 = %v, want [1 1 2 2]", scaled.Pixels[4:8])
	}
}

func TestScaleQPicNoOpWhenSameSize(t *testing.T) {
	pic := &qimage.QPic{Width: 2, Height: 1, Pixels: []byte{1, 2}}
	if got := ScaleQPic(pic, 2, 1); got != pic {
		t.Fatal("expected same-size scale to return the original pic")
	}
}

func TestPreparePicClipsAndScales(t *testing.T) {
	pic := &qimage.QPic{Width: 2, Height: 2, Pixels: []byte{1, 2, 3, 4}}
	clip := ClipRect{Enabled: true, X: 0, Y: 0, Width: 10, Height: 10}
	x, y, drawPic, ok := PreparePic(pic, 5, 5, 20, 20, 0, 0, 1, 1, clip)
	if !ok {
		t.Fatal("expected prepare to succeed")
	}
	if x != 5 || y != 5 {
		t.Fatalf("draw pos = (%v,%v), want (5,5)", x, y)
	}
	if drawPic == nil || drawPic.Width != 5 || drawPic.Height != 5 {
		t.Fatalf("draw pic = %+v, want 5x5", drawPic)
	}
}

func TestSubPicFromNormalizedRectEmpty(t *testing.T) {
	pic := &qimage.QPic{Width: 2, Height: 2, Pixels: []byte{1, 2, 3, 4}}
	got := SubPicFromNormalizedRect(pic, 0.5, 0.5, 0, 0)
	if got == nil || got.Width != 0 || got.Height != 0 {
		t.Fatalf("empty normalized rect = %+v, want empty pic", got)
	}
}