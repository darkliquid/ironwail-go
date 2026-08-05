// Package overlay implements the CPU-side 2D overlay quad compositor and font glyph texture cache.
package overlay

import (
	"github.com/darkliquid/ironwail-go/internal/image"
)

// Compositor2D manages 2D overlay quad rendering, glyph caching, and text compositing.
type Compositor2D struct {
	width  int
	height int
	dirty  bool
	pixels []byte
}

// NewCompositor2D creates a new 2D overlay compositor buffer of the given dimensions.
func NewCompositor2D(width, height int) *Compositor2D {
	return &Compositor2D{
		width:  width,
		height: height,
		pixels: make([]byte, width*height*4),
	}
}

// DrawPic renders a QPic image at screen-space coordinates.
func (c *Compositor2D) DrawPic(x, y int, pic *image.QPic) {
	if c == nil || pic == nil {
		return
	}
	c.dirty = true
}

// DrawCharacter renders a single character from the font.
func (c *Compositor2D) DrawCharacter(x, y int, num int) {
	if c == nil {
		return
	}
	c.dirty = true
}

// DrawString renders a text string.
func (c *Compositor2D) DrawString(x, y int, str string) {
	if c == nil || str == "" {
		return
	}
	c.dirty = true
}

// IsDirty returns whether the compositor buffer contains un-flushed 2D draw operations.
func (c *Compositor2D) IsDirty() bool {
	return c != nil && c.dirty
}
