package image

import (
	"image"
	"image/png"
	"io"

	"github.com/darkliquid/ironwail-go/pkg/wad"
)

// Palette represents the Quake 256-color palette as an array of RGBA colors.
type Palette = wad.Palette

// LoadPalette reads a 768-byte Quake palette from r.
var LoadPalette = wad.LoadPalette

// LoadPNG decodes a PNG image from the given reader, returning a standard Go
// image.Image. This is a thin wrapper around the standard library's png.Decode.
//
// PNG support allows the engine to load high-resolution replacement textures,
// HD skins, and other modern assets that the Quake modding community provides.
// While original Quake used only paletted formats (WAD lumps, .lmp files),
// source ports commonly support PNG/TGA/JPEG for texture packs that replace
// or enhance the original 8-bit art with full-color, higher-resolution images.
func LoadPNG(r io.Reader) (image.Image, error) {
	return png.Decode(r)
}

// MipTex represents a Quake mip-mapped texture with up to 4 detail levels.
type MipTex = wad.MipTex

// ParseMipTex parses a MipTex structure from raw binary data.
var ParseMipTex = wad.ParseMipTex

// AlphaEdgeFix corrects color bleeding artifacts on transparent texture edges.
func AlphaEdgeFix(img *image.RGBA) {
	wad.AlphaEdgeFix(img)
}

