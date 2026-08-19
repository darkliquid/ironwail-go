package quakui

import (
	"image"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/draw"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
)

// TestQPicToImage asserts the pic bridge converts a palette-indexed QPic to
// an RGBA image with palette index 255 transparent (Quake convention,
// ADR-0008).
func TestQPicToImage(t *testing.T) {
	pic := &qimage.QPic{
		Width:  2,
		Height: 2,
		Pixels: []byte{0, 4, 255, 254},
	}
	img := QPicToImage(pic, draw.DefaultQuakePalette())

	if img.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("bounds = %v, want 2x2", img.Bounds())
	}
	// palette[0] = black opaque.
	if r, g, b, a := img.At(0, 0).RGBA(); r != 0 || g != 0 || b != 0 || a != 0xffff {
		t.Fatalf("pixel(0,0) = %d,%d,%d,%d, want opaque black", r, g, b, a)
	}
	// palette[4] = 0x3f3f3f opaque.
	if r, g, b, _ := img.At(1, 0).RGBA(); r>>8 != 0x3f || g>>8 != 0x3f || b>>8 != 0x3f {
		t.Fatalf("pixel(1,0) = %d,%d,%d, want 0x3f3f3f", r>>8, g>>8, b>>8)
	}
	// palette[255] = transparent.
	if _, _, _, a := img.At(0, 1).RGBA(); a != 0 {
		t.Fatalf("pixel(0,1) alpha = %d, want 0 (transparent index 255)", a)
	}
	// palette[254] = white opaque.
	if r, g, b, a := img.At(1, 1).RGBA(); r>>8 != 0xff || g>>8 != 0xff || b>>8 != 0xff || a != 0xffff {
		t.Fatalf("pixel(1,1) = %d,%d,%d,%d, want white opaque", r>>8, g>>8, b>>8, a)
	}
}

// TestQPicToImageNilPalette asserts a nil palette falls back to the standard
// Quake palette without panicking.
func TestQPicToImageNilPalette(t *testing.T) {
	pic := &qimage.QPic{Width: 1, Height: 1, Pixels: []byte{0}}
	img := QPicToImage(pic, nil)
	if img == nil || img.Bounds() != image.Rect(0, 0, 1, 1) {
		t.Fatalf("nil palette produced %v", img)
	}
}

// testConchars returns a 128x128 conchars atlas where each 8x8 glyph cell is
// a hollow square (border pixels set) so glyphs are shaped, not solid.
func testConchars() []byte {
	data := make([]byte, 128*128)
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			cx := x % 8
			cy := y % 8
			if cx == 0 || cy == 0 || cx == 7 || cy == 7 {
				data[y*128+x] = 254 // white
			}
		}
	}
	return data
}

// TestQuakeConcharsGlyph asserts the conchars atlas exposes per-glyph
// SubImage views at the correct 8x8 grid positions.
func TestQuakeConcharsGlyph(t *testing.T) {
	atlas := NewConcharsAtlas(testConchars(), draw.DefaultQuakePalette())
	if atlas == nil {
		t.Fatal("NewConcharsAtlas = nil")
	}
	if b := atlas.Bounds(); b.Dx() != 128 || b.Dy() != 128 {
		t.Fatalf("atlas bounds = %v, want 128x128", b)
	}

	// Glyph 'A' (65) is at col=1, row=4 in the 16x16 grid.
	g := atlas.GlyphImage(65)
	if g == nil {
		t.Fatal("GlyphImage('A') = nil")
	}
	if b := g.Bounds(); b.Dx() != 8 || b.Dy() != 8 {
		t.Fatalf("glyph bounds = %v, want 8x8", b)
	}
	// The SubImage view keeps global atlas coordinates: iterate the cell rect
	// (8..16, 32..40).
	br := g.Bounds()
	nonTransparent := 0
	for y := br.Min.Y; y < br.Max.Y; y++ {
		for x := br.Min.X; x < br.Max.X; x++ {
			if _, _, _, a := g.At(x, y).RGBA(); a != 0 {
				nonTransparent++
			}
		}
	}
	if nonTransparent == 0 || nonTransparent == 64 {
		t.Fatalf("glyph non-transparent = %d, want shaped (0 < n < 64)", nonTransparent)
	}
}

// TestQuakeConcharsTransparent asserts palette index 0 is transparent in the
// atlas (Quake console-font convention, ADR-0008).
func TestQuakeConcharsTransparent(t *testing.T) {
	// A cell with index 0 for the interior: all interior pixels transparent.
	atlas := NewConcharsAtlas(testConchars(), draw.DefaultQuakePalette())
	g := atlas.GlyphImage(0)
	if _, _, _, a := g.At(1, 1).RGBA(); a != 0 {
		t.Fatalf("glyph interior pixel alpha = %d, want 0 (index 0 transparent)", a)
	}
}
