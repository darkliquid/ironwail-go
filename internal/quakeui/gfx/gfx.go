// Package gfx provides the Quake graphics bridges shared by the quakeui parent
// package and its widget subpackages (menu, console, hud): palette-indexed
// QPic -> RGBA conversion and the conchars bitmap atlas (ADR-0008, spec §4.3).
//
// It is a leaf package so the quakeui widget subpackages can use the bridges
// without importing the quakeui parent (which would create an import cycle).
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
// index (col = index%16, row = index/16). All 256 glyphs are pre-extracted
// into standalone 8x8 RGBA images with (0,0) origins for fast, zero-copy
// rendering without bounds translation bugs (ADR-0008).
type ConcharsAtlas struct {
	atlas  *image.RGBA
	glyphs [256]*image.RGBA
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
	ca := &ConcharsAtlas{
		atlas: image.NewRGBA(image.Rect(0, 0, 128, 128)),
	}
	for i, idx := range conchars {
		if i >= 128*128 {
			break
		}
		if idx == 0 {
			continue // transparent background
		}
		off := int(idx) * 3
		ca.atlas.Pix[i*4+0] = palette[off]
		ca.atlas.Pix[i*4+1] = palette[off+1]
		ca.atlas.Pix[i*4+2] = palette[off+2]
		ca.atlas.Pix[i*4+3] = 255
	}
	for idx := 0; idx < 256; idx++ {
		col := idx % 16
		row := idx / 16
		g := image.NewRGBA(image.Rect(0, 0, 8, 8))
		for y := 0; y < 8; y++ {
			srcOff := ((row*8+y)*128 + col*8) * 4
			dstOff := y * 8 * 4
			copy(g.Pix[dstOff:dstOff+32], ca.atlas.Pix[srcOff:srcOff+32])
		}
		ca.glyphs[idx] = g
	}
	return ca
}

// Bounds returns the atlas bounds (128x128).
func (a *ConcharsAtlas) Bounds() image.Rectangle {
	if a == nil || a.atlas == nil {
		return image.Rectangle{}
	}
	return a.atlas.Bounds()
}

// GlyphImage returns an 8x8 RGBA glyph image for a character index (0..255).
func (a *ConcharsAtlas) GlyphImage(index byte) image.Image {
	if a == nil {
		return nil
	}
	return a.glyphs[index]
}

// DrawGlyph blits an 8x8 glyph directly into a destination RGBA image with alpha testing.
func (a *ConcharsAtlas) DrawGlyph(dst *image.RGBA, x, y int, index byte) {
	if a == nil || dst == nil {
		return
	}
	g := a.glyphs[index]
	if g == nil {
		return
	}
	dstW := dst.Rect.Dx()
	dstH := dst.Rect.Dy()
	if x < 0 || x+8 > dstW || y < 0 || y+8 > dstH {
		return
	}
	for row := 0; row < 8; row++ {
		srcOff := row * 32
		dstOff := (y+row)*dst.Stride + x*4
		for col := 0; col < 8; col++ {
			alpha := g.Pix[srcOff+col*4+3]
			if alpha > 0 {
				dst.Pix[dstOff+col*4+0] = g.Pix[srcOff+col*4+0]
				dst.Pix[dstOff+col*4+1] = g.Pix[srcOff+col*4+1]
				dst.Pix[dstOff+col*4+2] = g.Pix[srcOff+col*4+2]
				dst.Pix[dstOff+col*4+3] = 255
			}
		}
	}
}

// TranslatePlayerSkinPixels remaps Quake player shirt (16-31) and pants (96-111)
// palette ranges to the selected top/bottom colors.
func TranslatePlayerSkinPixels(pixels []byte, topColor, bottomColor int) []byte {
	if len(pixels) == 0 {
		return nil
	}
	translated := make([]byte, len(pixels))
	copy(translated, pixels)

	topStart := byte((topColor & 15) << 4)
	bottomStart := byte((bottomColor & 15) << 4)

	for i, pixel := range translated {
		switch {
		case pixel >= 16 && pixel < 32:
			translated[i] = translatedPlayerColor(topStart, pixel-16)
		case pixel >= 96 && pixel < 112:
			translated[i] = translatedPlayerColor(bottomStart, pixel-96)
		}
	}
	return translated
}

func translatedPlayerColor(start, offset byte) byte {
	if start < 128 {
		return start + offset
	}
	return start + (15 - offset)
}

// QPicToImageTranslated converts a QPic with top and bottom player color translation.
func QPicToImageTranslated(pic *qimage.QPic, palette []byte, topColor, bottomColor int) *image.RGBA {
	if pic == nil {
		return nil
	}
	translatedPixels := TranslatePlayerSkinPixels(pic.Pixels, topColor, bottomColor)
	translatedPic := &qimage.QPic{
		Width:  pic.Width,
		Height: pic.Height,
		Pixels: translatedPixels,
	}
	return QPicToImage(translatedPic, palette)
}