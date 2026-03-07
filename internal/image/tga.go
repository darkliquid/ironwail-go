package image

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"io"
)

const (
	tgaTypeUncompressedTrueColor = 2
	tgaTypeUncompressedGray      = 3
	tgaTypeRLETrueColor          = 10
	tgaTypeRLEGray               = 11
)

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
func LoadTGA(r io.Reader) (image.Image, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	return DecodeTGA(data)
}

// DecodeTGA decodes uncompressed/RLE truecolor and grayscale TGA image data.
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
