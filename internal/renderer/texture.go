// Package renderer provides texture management for QPic graphics.
//
// This package handles uploading Quake picture (QPic) data to GPU textures,
// managing palette conversion, and handling transparency (palette index 255).
//
// Note: The actual texture upload is performed per-backend in the
// respective renderer implementation (OpenGL or GoGPU). This package
// provides shared utilities for texture management.
package renderer

import (
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/image"
)

// Texture represents a GPU texture created from a QPic.
// It provides platform-independent texture management.
type Texture struct {
	// Backend-specific texture handle (varies by renderer)
	backendHandle interface{}
	// Original QPic dimensions
	width  uint32
	height uint32
}

// UploadQPic uploads a QPic to a GPU texture.
// The exact upload method depends on the active renderer backend.
//
// For transparency: palette index 255 should be rendered as fully transparent.
// This is handled by the renderer-specific implementations.
func UploadQPic(pic *image.QPic) *Texture {
	if pic == nil {
		slog.Warn("Attempted to upload nil QPic")
		return nil
	}

	// Create texture wrapper
	tex := &Texture{
		width:  pic.Width,
		height: pic.Height,
	}

	// Note: The actual GPU upload happens in the renderer backends
	// when DrawPic is called. This wrapper provides a platform-agnostic
	// interface that can be extended for caching in the future.

	return tex
}

// menuScale computes the uniform scale factor and centering offsets for
// rendering the Quake 320×200 virtual menu space onto a physical screen.
// Matches the original engine's M_DrawPic centering behaviour on widescreen.
func menuScale(screenW, screenH int) (scale, xOff, yOff float32) {
	sx := float32(screenW) / 320.0
	sy := float32(screenH) / 200.0
	if sy < sx {
		scale = sy
	} else {
		scale = sx
	}
	xOff = (float32(screenW) - 320.0*scale) / 2.0
	yOff = (float32(screenH) - 200.0*scale) / 2.0
	return
}

type picRect struct {
	x float32
	y float32
	w float32
	h float32
}

// screenPicRect calculates pixel-aligned destination rectangles for HUD/console pictures in screen-space coordinates.
func screenPicRect(x, y int, pic *image.QPic) picRect {
	if pic == nil {
		return picRect{}
	}
	return picRect{
		x: float32(x),
		y: float32(y),
		w: float32(pic.Width),
		h: float32(pic.Height),
	}
}

// menuPicRect computes menu image placement rectangles while preserving legacy UI layout expectations.
func menuPicRect(screenW, screenH, x, y int, pic *image.QPic) picRect {
	rect := screenPicRect(x, y, pic)
	scale, xOff, yOff := menuScale(screenW, screenH)
	rect.x = rect.x*scale + xOff
	rect.y = rect.y*scale + yOff
	rect.w *= scale
	rect.h *= scale
	return rect
}

// IsTransparentIndex returns true if a palette index represents transparency.
// In Quake, palette index 255 is the transparent color.
func IsTransparentIndex(color byte) bool {
	return color == 255
}

// GetPaletteColor converts a palette index to RGB values.
// This requires access to the loaded palette (768 bytes: 256 colors * 3 RGB).
// Returns R, G, B values (0-255).
func GetPaletteColor(index byte, palette []byte) (r, g, b byte) {
	if len(palette) < 768 {
		// Invalid palette - return gray
		return index, index, index
	}

	offset := int(index) * 3
	if offset >= len(palette)-2 {
		// Out of range - return gray
		return index, index, index
	}

	return palette[offset], palette[offset+1], palette[offset+2]
}

// ConvertConcharsToRGBA converts conchars pixel data to RGBA.
// Unlike ConvertPaletteToRGBA, palette index 0 is treated as fully transparent
// (Quake convention for console character backgrounds).
func ConvertConcharsToRGBA(pixels []byte, palette []byte) []byte {
	rgba := make([]byte, len(pixels)*4)
	for i, p := range pixels {
		if p == 0 {
			// Transparent background
			rgba[i*4+3] = 0
			continue
		}
		if IsTransparentIndex(p) {
			rgba[i*4+3] = 0
			continue
		}
		r, g, b := GetPaletteColor(p, palette)
		rgba[i*4] = r
		rgba[i*4+1] = g
		rgba[i*4+2] = b
		rgba[i*4+3] = 255
	}
	return rgba
}

// ConvertPaletteToFullbrightRGBA creates a fullbright-only texture.
// Pixels with palette index 224-254 retain their color at full brightness.
// All other pixels (including 255 which is transparent) become fully transparent (0,0,0,0).
// Returns the RGBA data and whether any fullbright pixels were found.
func ConvertPaletteToFullbrightRGBA(pixels []byte, palette []byte) ([]byte, bool) {
	rgba := make([]byte, len(pixels)*4)
	hasFullbright := false
	for i, idx := range pixels {
		if idx >= 224 && idx != 255 { // 224-254 are fullbright, 255 is transparent
			hasFullbright = true
			r, g, b := GetPaletteColor(idx, palette)
			rgba[i*4+0] = r
			rgba[i*4+1] = g
			rgba[i*4+2] = b
			rgba[i*4+3] = 255
		} else {
			// All non-fullbright pixels become transparent
			rgba[i*4+0] = 0
			rgba[i*4+1] = 0
			rgba[i*4+2] = 0
			rgba[i*4+3] = 0
		}
	}
	return rgba, hasFullbright
}

// ConvertPaletteToRGBA converts a palette-indexed image to RGBA format.
// This is useful for texture uploaders that require RGBA input.
func ConvertPaletteToRGBA(pixels []byte, palette []byte) []byte {
	if len(palette) < 768 {
		// Invalid palette - return grayscale RGBA
		rgba := make([]byte, len(pixels)*4)
		for i, p := range pixels {
			rgba[i*4] = p
			rgba[i*4+1] = p
			rgba[i*4+2] = p
			rgba[i*4+3] = 255
		}
		return rgba
	}

	rgba := make([]byte, len(pixels)*4)
	for i, p := range pixels {
		if IsTransparentIndex(p) {
			rgba[i*4] = 0
			rgba[i*4+1] = 0
			rgba[i*4+2] = 0
			rgba[i*4+3] = 0 // Fully transparent
		} else {
			r, g, b := GetPaletteColor(p, palette)
			rgba[i*4] = r
			rgba[i*4+1] = g
			rgba[i*4+2] = b
			rgba[i*4+3] = 255 // Opaque
		}
	}
	return rgba
}
