package quakui

import (
	"image"

	"github.com/darkliquid/ironwail-go/internal/draw"
)

// ConcharsAtlas is the conchars bitmap font as an RGBA atlas (128x128,
// 16x16 grid of 8x8 glyphs). Palette index 0 is transparent (Quake
// console-font convention); glyph cells fall at col*8, row*8 for char
// index (col = index%16, row = index/16). Text is drawn per-glyph via
// GlyphImage SubImage views (ADR-0008 — no TTF for menu text).
type ConcharsAtlas struct {
	atlas *image.RGBA
}

// NewConcharsAtlas builds the atlas from the raw 128x128 indexed conchars
// pixels and the 768-byte Quake palette. Returns nil for a nil/short buffer.
func NewConcharsAtlas(conchars []byte, palette []byte) *ConcharsAtlas {
	if len(conchars) < 128*128 {
		return nil
	}
	if len(palette) < 768 {
		palette = draw.DefaultQuakePalette()
	}
	atlas := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for i, idx := range conchars {
		if i >= 128*128 {
			break
		}
		if idx == 0 {
			continue // transparent background
		}
		off := int(idx) * 3
		atlas.Pix[i*4+0] = palette[off]
		atlas.Pix[i*4+1] = palette[off+1]
		atlas.Pix[i*4+2] = palette[off+2]
		atlas.Pix[i*4+3] = 255
	}
	return &ConcharsAtlas{atlas: atlas}
}

// Bounds returns the atlas bounds (128x128).
func (a *ConcharsAtlas) Bounds() image.Rectangle {
	if a == nil || a.atlas == nil {
		return image.Rectangle{}
	}
	return a.atlas.Bounds()
}

// GlyphImage returns an 8x8 sub-image view into the atlas for a character
// index (col = index%16, row = index/16). The view shares the atlas backing
// pixels (zero-copy).
func (a *ConcharsAtlas) GlyphImage(index byte) image.Image {
	if a == nil || a.atlas == nil {
		return nil
	}
	col := int(index) % 16
	row := int(index) / 16
	r := image.Rect(col*8, row*8, col*8+8, row*8+8)
	return a.atlas.SubImage(r)
}
