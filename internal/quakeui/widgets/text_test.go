package widgets

import (
	"image"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// testConchars returns a minimal 128x128 conchars atlas: each glyph cell is
// filled with its char index so glyphs are distinguishable.
func testConchars() []byte {
	data := make([]byte, 128*128)
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			// Set only the border of each 8x8 cell (a hollow square glyph) so
			// glyphs are shaped, not solid blocks.
			cx := x % 8
			cy := y % 8
			if cx == 0 || cy == 0 || cx == 7 || cy == 7 {
				data[y*128+x] = 254 // white
			}
		}
	}
	return data
}

// TestQuakeTextMeasure asserts the widget measures text with an 8px advance
// per character (ADR-0004: static 8px/char advance table).
func TestQuakeTextMeasure(t *testing.T) {
	wt := NewQuakeText(testConchars(), draw.DefaultQuakePalette())

	if got := wt.Measure("QUAKE"); got != 40 {
		t.Fatalf("Measure(QUAKE) = %d, want 40 (5 chars * 8px)", got)
	}
	if got := wt.Measure(""); got != 0 {
		t.Fatalf("Measure(empty) = %d, want 0", got)
	}
	if got := wt.Measure("A B"); got != 24 {
		t.Fatalf("Measure('A B') = %d, want 24 (3 chars * 8px)", got)
	}
}

// TestQuakeTextBrightRow asserts the widget maps high-bit chars (char + 128)
// to the bright glyph row (ADR-0004: PUA bright set / bright fill).
func TestQuakeTextBrightRow(t *testing.T) {
	wt := NewQuakeText(testConchars(), draw.DefaultQuakePalette())

	// A high-bit char (e.g. 'A'+128) must resolve to a bright glyph index.
	normal, bright := wt.GlyphFor('A')
	if bright {
		t.Fatal("GlyphFor('A') reported bright, want false")
	}
	if normal != 'A' {
		t.Fatalf("GlyphFor('A') index = %d, want %d", normal, 'A')
	}

	hiNormal, hiBright := wt.GlyphFor('A' + 128)
	if !hiBright {
		t.Fatal("GlyphFor('A'+128) not bright, want true")
	}
	if hiNormal != 'A'+128 {
		t.Fatalf("GlyphFor('A'+128) index = %d, want %d", hiNormal, 'A'+128)
	}
}

// TestQuakeTextPrompt asserts the prompt glyph is ']' and the blink cursor
// alternates between glyphs 10 and 11 at 4Hz.
func TestQuakeTextPrompt(t *testing.T) {
	wt := NewQuakeText(testConchars(), draw.DefaultQuakePalette())

	if wt.Prompt() != ']' {
		t.Fatalf("Prompt() = %q, want ']'", wt.Prompt())
	}

	// Cursor glyph alternates at 4Hz: glyph 10 for [0, 0.25)s, glyph 11 for
	// [0.25, 0.5)s, wrapping every 0.25s (matches legacy console draw).
	if g := wt.CursorGlyph(0.0); g != 10 {
		t.Fatalf("CursorGlyph(0) = %d, want 10", g)
	}
	if g := wt.CursorGlyph(0.24); g != 10 {
		t.Fatalf("CursorGlyph(0.24) = %d, want 10 (first half)", g)
	}
	if g := wt.CursorGlyph(0.25); g != 11 {
		t.Fatalf("CursorGlyph(0.25) = %d, want 11 (second half)", g)
	}
	if g := wt.CursorGlyph(0.49); g != 11 {
		t.Fatalf("CursorGlyph(0.49) = %d, want 11", g)
	}
	if g := wt.CursorGlyph(0.50); g != 10 {
		t.Fatalf("CursorGlyph(0.50) = %d, want 10 (wraps)", g)
	}
}

// TestQuakeTextLayout asserts the widget lays out at the requested size and
// exposes the atlas image for glyph drawing.
func TestQuakeTextLayout(t *testing.T) {
	wt := NewQuakeText(testConchars(), draw.DefaultQuakePalette())

	ctx := widget.NewContext()
	size := wt.Layout(ctx, geometry.Expand())
	if size.Width <= 0 || size.Height <= 0 {
		t.Fatalf("Layout size = %v, want positive", size)
	}

	atlas := wt.Atlas()
	if atlas == nil {
		t.Fatal("Atlas() = nil")
	}
	if b := atlas.Bounds(); b.Dx() != 128 || b.Dy() != 128 {
		t.Fatalf("Atlas bounds = %v, want 128x128", b)
	}
}

// TestQuakeTextGlyphImage asserts GlyphImage returns an 8x8 sub-image for a
// character index.
func TestQuakeTextGlyphImage(t *testing.T) {
	wt := NewQuakeText(testConchars(), draw.DefaultQuakePalette())

	img := wt.GlyphImage('A')
	if img == nil {
		t.Fatal("GlyphImage('A') = nil")
	}
	if b := img.Bounds(); b.Dx() != 8 || b.Dy() != 8 {
		t.Fatalf("GlyphImage bounds = %v, want 8x8", b)
	}

	// The sub-image must be a view into the atlas (shares backing pixels).
	atlas := wt.Atlas()
	sub, ok := img.(*image.RGBA)
	if !ok {
		t.Fatalf("GlyphImage type = %T, want *image.RGBA", img)
	}
	// Glyph 'A' (65) is at col=1, row=4 in the 16x16 grid. The sub-image's
	// first pixel must match the atlas pixel at that cell origin.
	col := 65 % 16
	row := 65 / 16
	atlasPix := atlas.Pix[(row*8*128+col*8)*4:]
	subPix := sub.Pix[:4]
	if subPix[0] != atlasPix[0] || subPix[1] != atlasPix[1] || subPix[2] != atlasPix[2] {
		t.Fatal("GlyphImage does not match atlas backing pixels")
	}
}
