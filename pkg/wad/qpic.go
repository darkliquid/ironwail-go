package wad

import (
	"encoding/binary"
	"fmt"
	"image"
	"math"
)

// QPic represents a Quake 2D picture: a simple width×height image stored
// as palette-indexed pixel data with an 8-byte header.
type QPic struct {
	Width  uint32
	Height uint32
	Pixels []byte
}

// ParseQPic decodes a QPic from raw binary data.
func ParseQPic(data []byte) (*QPic, error) {
	if len(data) < 8 {
		return nil, fmt.Errorf("qpic data too short: %d bytes (minimum 8)", len(data))
	}

	width := binary.LittleEndian.Uint32(data[0:4])
	height := binary.LittleEndian.Uint32(data[4:8])
	if width == 0 || height == 0 {
		return nil, fmt.Errorf("qpic has invalid dimensions %dx%d", width, height)
	}

	numPixels := uint64(width) * uint64(height)
	if numPixels > math.MaxInt-8 || uint64(len(data)) < 8+numPixels {
		return nil, fmt.Errorf("qpic data too short: %d bytes for %dx%d (need %d)", len(data), width, height, 8+numPixels)
	}

	return &QPic{
		Width:  width,
		Height: height,
		Pixels: data[8 : 8+numPixels],
	}, nil
}

// SubPic returns a new QPic containing the specified rectangular region
// of the source image. Coordinates are clamped to source bounds.
func (p *QPic) SubPic(srcX, srcY, srcW, srcH int) *QPic {
	if srcX < 0 {
		srcX = 0
	}
	if srcY < 0 {
		srcY = 0
	}
	w := int(p.Width)
	h := int(p.Height)
	if srcX+srcW > w {
		srcW = w - srcX
	}
	if srcY+srcH > h {
		srcH = h - srcY
	}
	if srcW <= 0 || srcH <= 0 {
		return &QPic{Width: 0, Height: 0}
	}

	sub := &QPic{
		Width:  uint32(srcW),
		Height: uint32(srcH),
		Pixels: make([]byte, srcW*srcH),
	}
	for row := 0; row < srcH; row++ {
		srcOff := (srcY+row)*w + srcX
		dstOff := row * srcW
		copy(sub.Pixels[dstOff:dstOff+srcW], p.Pixels[srcOff:srcOff+srcW])
	}
	return sub
}

// ToRGBA converts the QPic's indexed pixels to an image.RGBA using the specified palette.
func (p *QPic) ToRGBA(palette Palette, transparent bool) *image.RGBA {
	return palette.ToRGBA(p.Pixels, int(p.Width), int(p.Height), transparent)
}
