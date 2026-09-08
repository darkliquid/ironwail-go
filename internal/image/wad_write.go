package image

import (
	"fmt"
	"image"
	"image/color"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkliquid/ironwail-go/pkg/wad"
)

// WadLump is a single lump for WriteWad.
type WadLump = wad.WadLump

// WriteWad writes a WAD2 archive to w.
var WriteWad = wad.WriteWad

// WriteQPicLump serializes a QPic lump.
var WriteQPicLump = wad.WriteQPicLump

// WriteMipTexLump serializes a MipTex lump.
var WriteMipTexLump = wad.WriteMipTexLump

// EncodePaletted quantizes RGBA pixels to Quake palette indices.
var EncodePaletted = wad.EncodePaletted

// LoadPaletteLmp parses a 768-byte Quake palette.lmp payload into a Palette.
func LoadPaletteLmp(data []byte) (Palette, error) {
	return wad.LoadPaletteBytes(data)
}

// DecodeQuakeImage loads an image file, dispatching on extension: .png via
// LoadPNG, .tga via LoadTGA. Everything else is rejected, matching the
// engine's supported source set for asset conversion.
func DecodeQuakeImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png":
		img, err := LoadPNG(f)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		return img, nil
	case ".tga":
		img, err := LoadTGA(f)
		if err != nil {
			return nil, fmt.Errorf("decode %s: %w", path, err)
		}
		return img, nil
	default:
		return nil, fmt.Errorf("unsupported image format %q (supported: .png, .tga)", filepath.Ext(path))
	}
}

// RGBAFromImage extracts row-major RGBA bytes (4 per pixel) from any
// image.Image, converting non-RGBA colour models as needed. The height of
// the returned buffer is the image's bounds height (top-left origin).
func RGBAFromImage(img image.Image) ([]byte, int, int) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]byte, w*h*4)
	// Fast path for plain *image.RGBA (as produced by png.Decode for
	// 8-bit RGBA inputs and by LoadTGA).
	if rgba, ok := img.(*image.RGBA); ok {
		copy(out, rgba.Pix[:w*h*4])
		return out, w, h
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			c := color.RGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.RGBA)
			i := (y*w + x) * 4
			out[i+0], out[i+1], out[i+2], out[i+3] = c.R, c.G, c.B, c.A
		}
	}
	return out, w, h
}
