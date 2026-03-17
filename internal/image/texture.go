package image

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
)

// Palette represents the Quake 256-color palette as an array of RGBA colors.
//
// Quake (and other id Software engines of that era) use 8-bit paletted textures
// to save memory and disk space. Every pixel in the game's textures, sprites,
// and 2D art is stored as a single byte — an index (0–255) into this palette.
// Modern GPUs do not natively support paletted rendering, so during texture
// upload the engine expands each palette index into a full 32-bit RGBA color
// using this table. The palette is defined once in palette.lmp and shared
// across the entire engine.
type Palette [256]color.RGBA

// LoadPalette reads a 768-byte Quake palette from the given reader and returns
// a Palette array with full alpha (A=255) for every entry.
//
// The on-disk format is simply 256 consecutive RGB triplets (3 bytes each,
// no header). This matches Quake's palette.lmp format found in pak0.pak.
// Alpha is set to 255 (fully opaque) for all colors because the palette file
// does not store alpha; transparency in Quake is handled per-pixel by treating
// palette index 255 as the transparent color (see ToRGBA).
func LoadPalette(r io.Reader) (Palette, error) {
	var p Palette
	data := make([]byte, 768)
	if _, err := io.ReadFull(r, data); err != nil {
		return p, err
	}
	for i := 0; i < 256; i++ {
		p[i] = color.RGBA{
			R: data[i*3+0],
			G: data[i*3+1],
			B: data[i*3+2],
			A: 255,
		}
	}
	return p, nil
}

// ToRGBA converts palette-indexed pixel data into a standard Go RGBA image
// suitable for GPU texture upload.
//
// Each byte in data is treated as a palette index. The method looks up the
// corresponding RGBA color and writes it into the output image's pixel buffer.
//
// When transparent is true, palette index 255 is treated as fully transparent
// (alpha = 0). This is Quake's convention for masked textures such as fence
// textures, grates, and sprites — the artists painted transparent areas with
// color index 255 (typically a bright magenta/cyan in the palette), and the
// engine skips those pixels during rendering or sets alpha to zero for
// alpha-blended drawing.
func (p Palette) ToRGBA(data []byte, width, height int, transparent bool) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for i, idx := range data {
		if i >= width*height {
			break
		}
		c := p[idx]
		if transparent && idx == 255 {
			c.A = 0
		}
		img.Pix[i*4+0] = c.R
		img.Pix[i*4+1] = c.G
		img.Pix[i*4+2] = c.B
		img.Pix[i*4+3] = c.A
	}
	return img
}

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
//
// Mip-mapping is a technique where each texture is stored at multiple
// resolutions: full size (mip 0), half size (mip 1), quarter size (mip 2),
// and eighth size (mip 3). The GPU selects the appropriate mip level based
// on the distance of the surface from the camera, which reduces aliasing
// artifacts on distant surfaces and improves cache performance.
//
// The on-disk format is a 40-byte header (16-byte name + width + height +
// 4 offsets), followed by the pixel data for all four mip levels stored
// contiguously. Offsets are relative to the start of the MipTex data.
// Each pixel is a single byte indexing into the Quake palette.
//
// MipTex is used for world textures (walls, floors, ceilings) in BSP files,
// and also appears as a lump type in WAD files. The conchars font bitmap
// is stored as a MipTex-type lump in gfx.wad (though without the header).
type MipTex struct {
	Name    string
	Width   uint32
	Height  uint32
	Offsets [4]uint32
	Pixels  []byte // All mip levels
}

// ParseMipTex parses a MipTex structure from raw binary data.
//
// The input data must be at least 40 bytes (the fixed-size header). The
// pixel data referenced by the four mip offsets is retained as a slice of
// the original data buffer, so the caller must not modify data after parsing.
//
// The 16-byte name field may contain trailing NUL bytes or spaces, which
// are cleaned up by CleanupName to produce a normalized lowercase key
// suitable for texture lookups.
func ParseMipTex(data []byte) (*MipTex, error) {
	if len(data) < 40 {
		return nil, fmt.Errorf("miptex data too short")
	}
	name := CleanupName(string(data[:16]))
	width := binary.LittleEndian.Uint32(data[16:20])
	height := binary.LittleEndian.Uint32(data[20:24])
	var offsets [4]uint32
	for i := 0; i < 4; i++ {
		offsets[i] = binary.LittleEndian.Uint32(data[24+i*4 : 28+i*4])
	}
	return &MipTex{
		Name:    name,
		Width:   width,
		Height:  height,
		Offsets: offsets,
		Pixels:  data,
	}, nil
}

// MipLevel extracts the pixel data for the specified mip level (0–3) and
// returns the raw bytes along with the level's width and height.
//
// Each successive mip level halves both dimensions (clamped to a minimum of 1).
// For example, a 64×64 base texture has mip levels of 64×64, 32×32, 16×16,
// and 8×8. The offset stored in the MipTex header points to where each level's
// contiguous block of palette-indexed pixels begins within the Pixels slice.
//
// This method validates that the requested level is in range and that the
// pixel data does not exceed the buffer, returning an error otherwise.
func (m *MipTex) MipLevel(level int) ([]byte, int, int, error) {
	if level < 0 || level >= 4 {
		return nil, 0, 0, fmt.Errorf("invalid mip level")
	}
	off := m.Offsets[level]
	w := int(m.Width) >> level
	h := int(m.Height) >> level
	if w == 0 {
		w = 1
	}
	if h == 0 {
		h = 1
	}
	size := w * h
	if int(off)+size > len(m.Pixels) {
		return nil, 0, 0, fmt.Errorf("mip level data out of bounds")
	}
	return m.Pixels[off : int(off)+size], w, h, nil
}

// AlphaEdgeFix corrects color bleeding artifacts on transparent texture edges
// by averaging the RGB values of neighboring opaque pixels into transparent pixels.
//
// When a texture with transparency (e.g., a fence texture or sprite) is rendered
// with bilinear or trilinear filtering on the GPU, the hardware interpolates
// between adjacent texels. If a transparent pixel has arbitrary RGB values
// (often black or garbage), the interpolation produces visible dark or colored
// fringes around the edges of the opaque regions — a common artifact known as
// "dark halos" or "alpha bleeding."
//
// This function fixes the problem by examining each transparent pixel's 8
// neighbors (with toroidal wrapping for tiling textures). If any neighbors are
// opaque, their RGB values are averaged and written into the transparent pixel.
// The alpha channel remains zero, so the pixel is still invisible, but when the
// GPU interpolates between this pixel and an adjacent opaque one, the blended
// color will be a smooth continuation rather than an ugly fringe.
//
// This is a standard technique used in many game engines and is equivalent to
// the "premultiplied alpha edge padding" step in texture processing pipelines.
func AlphaEdgeFix(img *image.RGBA) {
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	data := img.Pix

	for i := 0; i < height; i++ {
		lastrow := width * 4 * ((i - 1 + height) % height)
		thisrow := width * 4 * i
		nextrow := width * 4 * ((i + 1) % height)

		for j := 0; j < width; j++ {
			destIdx := thisrow + j*4
			if data[destIdx+3] != 0 { // not transparent
				continue
			}

			lastpix := 4 * ((j - 1 + width) % width)
			thispix := 4 * j
			nextpix := 4 * ((j + 1) % width)

			var r, g, b, n int
			check := func(row, pix int) {
				idx := row + pix
				if data[idx+3] != 0 {
					r += int(data[idx+0])
					g += int(data[idx+1])
					b += int(data[idx+2])
					n++
				}
			}

			check(lastrow, lastpix)
			check(thisrow, lastpix)
			check(nextrow, lastpix)
			check(lastrow, thispix)
			check(nextrow, thispix)
			check(lastrow, nextpix)
			check(thisrow, nextpix)
			check(nextrow, nextpix)

			if n > 0 {
				data[destIdx+0] = uint8(r / n)
				data[destIdx+1] = uint8(g / n)
				data[destIdx+2] = uint8(b / n)
			}
		}
	}
}
