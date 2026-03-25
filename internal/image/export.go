package image

// export.go provides image export functionality for screenshots and texture
// debugging. WritePNG and WriteTGA encode paletted or RGBA image data to the
// respective formats. RGBAFromPalette expands 8-bit paletted pixels to RGBA.

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
)

func validateExportBuffer(pixels []byte, width, height, bpp int, fn string) (int, error) {
	if bpp != 24 && bpp != 32 {
		return 0, fmt.Errorf("%s: bpp must be 24 or 32, got %d", fn, bpp)
	}
	bytesPerPixel := bpp / 8
	expected := width * height * bytesPerPixel
	if len(pixels) != expected {
		return 0, fmt.Errorf("%s: expected %d bytes, got %d", fn, expected, len(pixels))
	}
	return bytesPerPixel, nil
}

func copyRowsFlipped(pixels []byte, width, height, bytesPerPixel int) []byte {
	rowBytes := width * bytesPerPixel
	flipped := make([]byte, len(pixels))
	for y := 0; y < height; y++ {
		src := y * rowBytes
		dst := (height - 1 - y) * rowBytes
		copy(flipped[dst:dst+rowBytes], pixels[src:src+rowBytes])
	}
	return flipped
}

// WritePNG writes an RGB or RGBA pixel buffer as a PNG image.
// bpp must be 24 (RGB) or 32 (RGBA). When upsidedown is false, scanlines are
// flipped before encode to mirror C Image_WritePNG().
func WritePNG(w io.Writer, pixels []byte, width, height, bpp int, upsidedown bool) error {
	bytesPerPixel, err := validateExportBuffer(pixels, width, height, bpp, "WritePNG")
	if err != nil {
		return err
	}

	src := pixels
	if !upsidedown {
		src = copyRowsFlipped(pixels, width, height, bytesPerPixel)
	}

	if bpp == 24 {
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				si := (y*width + x) * 3
				di := y*img.Stride + x*4
				img.Pix[di+0] = src[si+0]
				img.Pix[di+1] = src[si+1]
				img.Pix[di+2] = src[si+2]
				img.Pix[di+3] = 0xff
			}
		}
		return png.Encode(w, img)
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, src)
	return png.Encode(w, img)
}

// WriteTGA writes an RGB or RGBA pixel buffer as an uncompressed TGA image.
// bpp must be 24 (RGB) or 32 (RGBA). When upsidedown is true, the top-origin
// descriptor bit is set to mirror C Image_WriteTGA().
func WriteTGA(w io.Writer, pixels []byte, width, height, bpp int, upsidedown bool) error {
	bytesPerPixel, err := validateExportBuffer(pixels, width, height, bpp, "WriteTGA")
	if err != nil {
		return err
	}

	header := [18]byte{
		0,
		0,
		2,
		0, 0,
		0, 0,
		0,
		0, 0,
		0, 0,
		byte(width & 0xff), byte((width >> 8) & 0xff),
		byte(height & 0xff), byte((height >> 8) & 0xff),
		byte(bpp),
		0,
	}
	if bpp == 32 {
		header[17] = 8
	}
	if upsidedown {
		header[17] |= 0x20
	}
	if _, err := w.Write(header[:]); err != nil {
		return err
	}

	row := make([]byte, width*bytesPerPixel)
	for y := 0; y < height; y++ {
		off := y * width * bytesPerPixel
		for x := 0; x < width; x++ {
			si := off + x*bytesPerPixel
			di := x * bytesPerPixel
			row[di+0] = pixels[si+2]
			row[di+1] = pixels[si+1]
			row[di+2] = pixels[si+0]
			if bytesPerPixel == 4 {
				row[di+3] = pixels[si+3]
			}
		}
		if _, err := w.Write(row); err != nil {
			return err
		}
	}
	return nil
}

// RGBAFromPalette converts a paletted 8-bit image to RGBA using the given
// 256-color palette. Palette entries are [R, G, B] triples.
func RGBAFromPalette(indexed []byte, palette [256]color.RGBA, width, height int) []byte {
	out := make([]byte, width*height*4)
	for i, idx := range indexed {
		c := palette[idx]
		out[i*4+0] = c.R
		out[i*4+1] = c.G
		out[i*4+2] = c.B
		out[i*4+3] = c.A
	}
	return out
}
