package quakui

import (
	"image"

	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/quakui/gfx"
)

// QPicToImage converts a palette-indexed Quake QPic into an RGBA image
// suitable for canvas.DrawImage (ADR-0008, spec §4.3). Palette index 255 is
// treated as fully transparent, matching Quake's masked-pic convention.
//
// A nil palette falls back to the standard Quake palette so callers can
// bridge pics before the draw manager is initialized.
func QPicToImage(pic *qimage.QPic, palette []byte) *image.RGBA {
	return gfx.QPicToImage(pic, palette)
}

// ConcharsAtlas is the conchars bitmap font as an RGBA atlas (128x128,
// 16x16 grid of 8x8 glyphs). Palette index 0 is transparent (Quake
// console-font convention); glyph cells fall at col*8, row*8 for char
// index (col = index%16, row = index/16). Text is drawn per-glyph via
// GlyphImage SubImage views (ADR-0008 — no TTF for menu text).
type ConcharsAtlas = gfx.ConcharsAtlas

// NewConcharsAtlas builds the atlas from the raw 128x128 indexed conchars
// pixels and the 768-byte Quake palette. Returns nil for a nil/short buffer.
func NewConcharsAtlas(conchars []byte, palette []byte) *ConcharsAtlas {
	return gfx.NewConcharsAtlas(conchars, palette)
}