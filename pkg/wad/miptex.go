package wad

import (
	"encoding/binary"
	"fmt"
	"image"
	"strings"
)

// MipTex represents a Quake mip-mapped texture with up to 4 detail levels.
type MipTex struct {
	Name    string
	Width   uint32
	Height  uint32
	Offsets [4]uint32
	Pixels  []byte // Raw payload including header and all mip levels
}

// CleanupName normalizes a lump name to lowercase, strips trailing NUL bytes
// and spaces, producing a canonical key for map lookups.
func CleanupName(name string) string {
	name = strings.ToLower(name)
	if i := strings.IndexByte(name, 0); i != -1 {
		name = name[:i]
	}
	return strings.TrimRight(name, " ")
}

// ParseMipTex parses a MipTex structure from raw binary data.
func ParseMipTex(data []byte) (*MipTex, error) {
	if len(data) < 40 {
		return nil, fmt.Errorf("miptex data too short: %d bytes (minimum 40)", len(data))
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
func (m *MipTex) MipLevel(level int) ([]byte, int, int, error) {
	if level < 0 || level >= 4 {
		return nil, 0, 0, fmt.Errorf("invalid mip level %d (must be 0-3)", level)
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
	if uint64(off)+uint64(size) > uint64(len(m.Pixels)) {
		return nil, 0, 0, fmt.Errorf("mip level %d data out of bounds (offset %d + size %d > total %d)", level, off, size, len(m.Pixels))
	}
	return m.Pixels[off : off+uint32(size)], w, h, nil
}

// ToRGBA converts the specified mip level into an *image.RGBA using the provided palette.
func (m *MipTex) ToRGBA(palette Palette, level int) *image.RGBA {
	data, w, h, err := m.MipLevel(level)
	if err != nil {
		return nil
	}
	return palette.ToRGBA(data, w, h, false)
}

// AlphaEdgeFix corrects color bleeding artifacts on transparent texture edges
// by averaging the RGB values of neighboring opaque pixels into transparent pixels.
func AlphaEdgeFix(img *image.RGBA) {
	if img == nil || img.Bounds().Dx() == 0 || img.Bounds().Dy() == 0 {
		return
	}
	width := img.Bounds().Dx()
	height := img.Bounds().Dy()
	data := img.Pix

	for i := 0; i < height; i++ {
		lastrow := width * 4 * ((i - 1 + height) % height)
		thisrow := width * 4 * i
		nextrow := width * 4 * ((i + 1) % height)

		for j := 0; j < width; j++ {
			destIdx := thisrow + j*4
			if data[destIdx+3] != 0 {
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
			check(lastrow, thispix)
			check(lastrow, nextpix)
			check(thisrow, lastpix)
			check(thisrow, nextpix)
			check(nextrow, lastpix)
			check(nextrow, thispix)
			check(nextrow, nextpix)

			if n > 0 {
				data[destIdx+0] = byte(r / n)
				data[destIdx+1] = byte(g / n)
				data[destIdx+2] = byte(b / n)
			}
		}
	}
}
