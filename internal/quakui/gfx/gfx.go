// Package gfx provides the Quake graphics bridges shared by the quakui parent
// package and its widget subpackages (menu, console, hud): palette-indexed
// QPic -> RGBA conversion and the conchars bitmap atlas (ADR-0008, spec §4.3).
//
// It is a leaf package so the quakui widget subpackages can use the bridges
// without importing the quakui parent (which would create an import cycle).
// The parent package re-exports these for backward compatibility.
package gfx

import (
	"image"
	"image/color"

	"github.com/darkliquid/ironwail-go/internal/draw"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
)

// QPicToImage converts a palette-indexed Quake QPic into an RGBA image
// suitable for canvas.DrawImage (ADR-0008, spec §4.3). Palette index 255 is
// treated as fully transparent, matching Quake's masked-pic convention.
//
// A nil palette falls back to the standard Quake palette so callers can
// bridge pics before the draw manager is initialized.
func QPicToImage(pic *qimage.QPic, palette []byte) *image.RGBA {
	if pic == nil {
		return nil
	}
	if len(palette) < 768 {
		palette = draw.DefaultQuakePalette()
	}

	w := int(pic.Width)
	h := int(pic.Height)
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i, idx := range pic.Pixels {
		if i >= w*h {
			break
		}
		var c color.RGBA
		if idx == 255 {
			c = color.RGBA{A: 0}
		} else {
			off := int(idx) * 3
			c = color.RGBA{
				R: palette[off],
				G: palette[off+1],
				B: palette[off+2],
				A: 255,
			}
		}
		img.Set(i%w, i/w, c)
	}
	return img
}

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