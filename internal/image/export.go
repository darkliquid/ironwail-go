package image

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
)

// WritePNG writes an RGBA pixel buffer as a PNG image.
// pixels must be width*height*4 bytes (RGBA, top-to-bottom scanlines).
// Mirrors C Image_WritePNG() from image.c.
func WritePNG(w io.Writer, pixels []byte, width, height int) error {
	if len(pixels) != width*height*4 {
		return fmt.Errorf("WritePNG: expected %d bytes, got %d", width*height*4, len(pixels))
	}

	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	copy(img.Pix, pixels)
	return png.Encode(w, img)
}

// WriteTGA writes an RGBA pixel buffer as a TGA image (uncompressed, 32-bit).
// pixels must be width*height*4 bytes (RGBA, top-to-bottom scanlines).
// Mirrors C Image_WriteTGA() from image.c.
func WriteTGA(w io.Writer, pixels []byte, width, height int) error {
	if len(pixels) != width*height*4 {
		return fmt.Errorf("WriteTGA: expected %d bytes, got %d", width*height*4, len(pixels))
	}

	// TGA header: 18 bytes
	header := [18]byte{
		0,    // id length
		0,    // color map type
		2,    // image type: uncompressed true-color
		0, 0, // color map offset
		0, 0, // color map length
		0,    // color map depth
		0, 0, // x origin
		0, 0, // y origin
		byte(width & 0xff), byte((width >> 8) & 0xff),
		byte(height & 0xff), byte((height >> 8) & 0xff),
		32, // bits per pixel
		8,  // image descriptor: 8 attribute bits (alpha), origin upper-left
	}
	if _, err := w.Write(header[:]); err != nil {
		return err
	}

	// TGA stores BGRA, so convert from RGBA.
	row := make([]byte, width*4)
	for y := 0; y < height; y++ {
		off := y * width * 4
		for x := 0; x < width; x++ {
			si := off + x*4
			di := x * 4
			row[di+0] = pixels[si+2] // B
			row[di+1] = pixels[si+1] // G
			row[di+2] = pixels[si+0] // R
			row[di+3] = pixels[si+3] // A
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
