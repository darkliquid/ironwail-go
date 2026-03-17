package image

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
)

// TGA image type constants, matching the TGA specification's image type field.
//
// TGA (Truevision Targa) supports several encoding modes. Quake source ports
// only use a subset relevant to game assets:
//   - Type 2: Uncompressed truecolor (24-bit RGB or 32-bit RGBA)
//   - Type 3: Uncompressed grayscale (8-bit)
//   - Type 10: Run-length encoded (RLE) truecolor
//   - Type 11: Run-length encoded (RLE) grayscale
//
// Color-mapped (paletted) TGA types (1, 9) are not supported because Quake
// replacement textures are always stored as direct-color images.
const (
	tgaTypeUncompressedTrueColor = 2
	tgaTypeUncompressedGray      = 3
	tgaTypeRLETrueColor          = 10
	tgaTypeRLEGray               = 11
)

// tgaHeader is the 18-byte fixed header at the start of every TGA file.
//
// The header describes the image dimensions, pixel format, and encoding.
// Key fields for Quake asset loading:
//   - ImageType: selects between uncompressed/RLE and truecolor/grayscale
//   - PixelDepth: bits per pixel (24 for RGB, 32 for RGBA, 8 for grayscale)
//   - ImageDescriptor bit 5: origin location (0 = bottom-left, 1 = top-left)
//
// TGA uses bottom-left origin by default (matching OpenGL's convention),
// but many tools export top-left origin images. The decoder checks bit 5
// of ImageDescriptor to handle both orientations correctly.
//
// ColorMap fields are present in the header format but must be zero for
// the image types this decoder supports (non-color-mapped).
type tgaHeader struct {
	IDLength        uint8
	ColorMapType    uint8
	ImageType       uint8
	ColorMapStart   uint16
	ColorMapLength  uint16
	ColorMapDepth   uint8
	XOrigin         uint16
	YOrigin         uint16
	Width           uint16
	Height          uint16
	PixelDepth      uint8
	ImageDescriptor uint8
}

// LoadTGA decodes a subset of TGA images used by Quake assets.
//
// This is a convenience wrapper that reads all data from the reader into
// memory and delegates to DecodeTGA. It exists to provide a streaming
// interface (io.Reader) consistent with LoadPNG and other image loaders,
// even though TGA decoding requires random access to the full data.
//
// TGA is the primary format for replacement textures, skyboxes, and
// high-resolution assets in Quake source ports. Modders and texture pack
// authors use TGA because it supports an alpha channel (32-bit RGBA),
// is simple to produce from image editors, and was the first external
// texture format supported by early GLQuake derivatives.
func LoadTGA(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return DecodeTGA(data)
}

// DecodeTGA decodes uncompressed/RLE truecolor and grayscale TGA image data.
//
// The decoding process:
//  1. Parse the 18-byte header to determine image type, dimensions, and pixel depth.
//  2. Validate that the image type and pixel depth are in the supported subset.
//  3. Skip past the optional image ID field (IDLength bytes after the header).
//  4. Decode the pixel payload — either a direct copy (uncompressed) or RLE
//     decompression (see decodeTGAPixels).
//  5. Convert the decoded pixels into a standard Go RGBA image, handling:
//     - TGA's BGR/BGRA byte order → RGBA reordering
//     - Bottom-left vs. top-left origin (ImageDescriptor bit 5)
//     - Grayscale expansion (replicate the gray value into R, G, B channels)
//
// Note on byte order: TGA stores color channels as B, G, R[, A] — the reverse
// of the RGBA order used by Go's image.RGBA. The inner loop swaps channels
// accordingly (raw[idx+2] → R, raw[idx+1] → G, raw[idx] → B).
func DecodeTGA(data []byte) (*image.RGBA, error) {
	if len(data) < 18 {
		return nil, fmt.Errorf("tga data too short")
	}
	var hdr tgaHeader
	if err := binary.Read(bytes.NewReader(data[:18]), binary.LittleEndian, &hdr); err != nil {
		return nil, fmt.Errorf("read tga header: %w", err)
	}
	if hdr.ColorMapType != 0 {
		return nil, fmt.Errorf("unsupported tga colormap type %d", hdr.ColorMapType)
	}
	switch hdr.ImageType {
	case tgaTypeUncompressedTrueColor, tgaTypeUncompressedGray, tgaTypeRLETrueColor, tgaTypeRLEGray:
	default:
		return nil, fmt.Errorf("unsupported tga image type %d", hdr.ImageType)
	}
	if hdr.Width == 0 || hdr.Height == 0 {
		return nil, fmt.Errorf("invalid tga size %dx%d", hdr.Width, hdr.Height)
	}

	bytesPerPixel := int(hdr.PixelDepth) / 8
	switch hdr.ImageType {
	case tgaTypeUncompressedGray, tgaTypeRLEGray:
		if bytesPerPixel != 1 {
			return nil, fmt.Errorf("unsupported tga grayscale depth %d", hdr.PixelDepth)
		}
	default:
		if bytesPerPixel != 3 && bytesPerPixel != 4 {
			return nil, fmt.Errorf("unsupported tga truecolor depth %d", hdr.PixelDepth)
		}
	}

	offset := 18 + int(hdr.IDLength)
	if offset > len(data) {
		return nil, fmt.Errorf("invalid tga id length")
	}
	payload := data[offset:]
	pixelCount := int(hdr.Width) * int(hdr.Height)
	raw, err := decodeTGAPixels(payload, hdr.ImageType, bytesPerPixel, pixelCount)
	if err != nil {
		return nil, err
	}

	img := image.NewRGBA(image.Rect(0, 0, int(hdr.Width), int(hdr.Height)))
	topOrigin := hdr.ImageDescriptor&0x20 != 0
	for y := 0; y < int(hdr.Height); y++ {
		dstY := y
		if !topOrigin {
			dstY = int(hdr.Height) - 1 - y
		}
		for x := 0; x < int(hdr.Width); x++ {
			idx := (y*int(hdr.Width) + x) * bytesPerPixel
			switch bytesPerPixel {
			case 1:
				gray := raw[idx]
				img.SetRGBA(x, dstY, color.RGBA{R: gray, G: gray, B: gray, A: 255})
			case 3:
				img.SetRGBA(x, dstY, color.RGBA{R: raw[idx+2], G: raw[idx+1], B: raw[idx], A: 255})
			case 4:
				img.SetRGBA(x, dstY, color.RGBA{R: raw[idx+2], G: raw[idx+1], B: raw[idx], A: raw[idx+3]})
			}
		}
	}

	return img, nil
}

// decodeTGAPixels extracts raw pixel bytes from the TGA payload, handling
// both uncompressed and RLE-compressed data.
//
// For uncompressed images (types 2, 3), the payload is simply copied verbatim.
//
// For RLE-compressed images (types 10, 11), the payload is a stream of packets:
//   - Each packet starts with a 1-byte header. Bit 7 selects the packet type:
//   - 1 (RLE packet): The next pixel value is repeated (header & 0x7F) + 1 times.
//     This efficiently encodes runs of identical pixels (e.g., solid-color regions).
//   - 0 (raw packet): The next (header & 0x7F) + 1 pixels are stored literally.
//     This handles areas with varying colors where RLE would be counterproductive.
//
// RLE compression in TGA typically achieves 2:1 to 4:1 ratios on game textures
// that have large solid-color or gradient regions (skyboxes, UI elements).
// The decompression loop accumulates decoded pixels until the expected total
// (pixelCount × bytesPerPixel) is reached.
func decodeTGAPixels(payload []byte, imageType uint8, bytesPerPixel, pixelCount int) ([]byte, error) {
	totalBytes := pixelCount * bytesPerPixel
	switch imageType {
	case tgaTypeUncompressedTrueColor, tgaTypeUncompressedGray:
		if len(payload) < totalBytes {
			return nil, fmt.Errorf("truncated tga payload")
		}
		return append([]byte(nil), payload[:totalBytes]...), nil
	case tgaTypeRLETrueColor, tgaTypeRLEGray:
		decoded := make([]byte, 0, totalBytes)
		i := 0
		for len(decoded) < totalBytes {
			if i >= len(payload) {
				return nil, fmt.Errorf("truncated tga rle payload")
			}
			packet := payload[i]
			i++
			run := int(packet&0x7f) + 1
			if packet&0x80 != 0 {
				if i+bytesPerPixel > len(payload) {
					return nil, fmt.Errorf("truncated tga rle packet")
				}
				pixel := payload[i : i+bytesPerPixel]
				i += bytesPerPixel
				for j := 0; j < run; j++ {
					decoded = append(decoded, pixel...)
				}
				continue
			}
			readBytes := run * bytesPerPixel
			if i+readBytes > len(payload) {
				return nil, fmt.Errorf("truncated tga raw packet")
			}
			decoded = append(decoded, payload[i:i+readBytes]...)
			i += readBytes
		}
		return decoded[:totalBytes], nil
	default:
		return nil, fmt.Errorf("unsupported tga image type %d", imageType)
	}
}
