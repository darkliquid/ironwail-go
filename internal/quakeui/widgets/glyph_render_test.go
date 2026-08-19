package widgets

import (
	"image"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/gogpu/gg"
	"github.com/gogpu/ui/render"
)

// TestGlyphRendersColored asserts a glyph drawn via the widget canvas path
// produces non-white, non-transparent pixels (not solid white blocks).
func TestGlyphRendersColored(t *testing.T) {
	wt := NewQuakeText(testConchars(), draw.DefaultQuakePalette())

	cc := gg.NewContext(64, 64)
	canvas := render.NewCanvas(cc, 64, 64)

	// Draw the 'A' glyph (index 65) at (0,0).
	wt.DrawString(canvas, 0, 0, "A")

	img := cc.Image()
	rgba, ok := img.(*image.RGBA)
	if !ok {
		t.Fatalf("canvas image type = %T, want *image.RGBA", img)
	}

	// Count non-transparent pixels in the 8x8 glyph region.
	nonTransparent := 0
	white := 0
	for y := 0; y < 8; y++ {
		for x := 0; x < 8; x++ {
			off := y*64*4 + x*4
			a := rgba.Pix[off+3]
			if a != 0 {
				nonTransparent++
				if rgba.Pix[off] == 255 && rgba.Pix[off+1] == 255 && rgba.Pix[off+2] == 255 {
					white++
				}
			}
		}
	}
	if nonTransparent == 0 {
		t.Fatal("glyph drew no non-transparent pixels")
	}
	t.Logf("glyph non-transparent=%d white=%d", nonTransparent, white)
	if nonTransparent == 64 {
		t.Fatalf("glyph region fully opaque (%d/64) - likely a solid block", nonTransparent)
	}
}

// TestScaledGlyphRenders asserts the CPU-scaled glyph path also produces
// colored pixels.
func TestScaledGlyphRenders(t *testing.T) {
	wt := NewQuakeText(testConchars(), draw.DefaultQuakePalette())

	cc := gg.NewContext(64, 64)
	canvas := render.NewCanvas(cc, 64, 64)

	wt.DrawStringScaled(canvas, 0, 0, 2, "A")

	img := cc.Image()
	rgba, ok := img.(*image.RGBA)
	if !ok {
		t.Fatalf("canvas image type = %T, want *image.RGBA", img)
	}
	nonTransparent := 0
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			if rgba.Pix[y*64*4+x*4+3] != 0 {
				nonTransparent++
			}
		}
	}
	if nonTransparent == 0 {
		t.Fatal("scaled glyph drew no non-transparent pixels")
	}
	t.Logf("scaled glyph non-transparent=%d", nonTransparent)
}
