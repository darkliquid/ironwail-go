package image

import (
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
)

type Palette [256]color.RGBA

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

func LoadPNG(r io.Reader) (image.Image, error) {
	return png.Decode(r)
}

type MipTex struct {
	Name    string
	Width   uint32
	Height  uint32
	Offsets [4]uint32
	Pixels  []byte // All mip levels
}

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
