// Package widgets implements the custom Quake widgets for the gogpu/ui menu,
// console, and HUD (IRONWAIL-SPEC-001 §3.1). Per the T1 spike (ADR-0004),
// text is rendered by a QuakeTextWidget backed by the conchars atlas with a
// static 8px advance table, never a raw per-char canvas loop outside the
// widget. No stock core/* widgets are used (AC8: BYO-kit).
package widgets

import (
	"image"

	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// QuakeText is a widget that renders Quake conchars text from the engine's
// 128x128 conchars atlas (ADR-0004, T1 decision). It measures with a static
// 8px/char advance table and draws each glyph as an image.RGBA.SubImage view
// into the atlas. Layout (Box, alignment, wrap) still comes from gogpu/ui.
//
// The high-bit row (char + 128) is the bright glyph row: GlyphFor reports it
// so callers can apply a bright fill color via the quakeui ThemeExtension.
type QuakeText struct {
	widget.WidgetBase

	// atlas is the RGBA conchars atlas (128x128), palette index 0
	// transparent (Quake console-font convention).
	atlas *image.RGBA

	// prompt is the console input prompt glyph (']').
	prompt rune
}

// NewQuakeText builds a QuakeText widget from the raw 128x128 conchars pixel
// data and the 768-byte Quake palette. A nil or short conchars buffer yields
// an empty atlas (no glyphs drawn).
func NewQuakeText(conchars []byte, palette []byte) *QuakeText {
	wt := &QuakeText{prompt: ']'}
	wt.SetVisible(true)
	wt.SetEnabled(true)
	if len(conchars) >= 128*128 {
		wt.atlas = concharsToRGBA(conchars, palette)
	}
	return wt
}

// concharsToRGBA converts the 128x128 indexed conchars atlas to RGBA with
// palette index 0 transparent (Quake console-font convention, matching the
// renderer's ConvertConcharsToRGBA).
func concharsToRGBA(conchars []byte, palette []byte) *image.RGBA {
	if len(palette) < 768 {
		palette = draw.DefaultQuakePalette()
	}
	img := image.NewRGBA(image.Rect(0, 0, 128, 128))
	for i, idx := range conchars {
		if i >= 128*128 {
			break
		}
		var c [4]byte
		if idx == 0 {
			c = [4]byte{0, 0, 0, 0} // transparent background
		} else {
			off := int(idx) * 3
			c = [4]byte{palette[off], palette[off+1], palette[off+2], 255}
		}
		img.Pix[i*4+0] = c[0]
		img.Pix[i*4+1] = c[1]
		img.Pix[i*4+2] = c[2]
		img.Pix[i*4+3] = c[3]
	}
	return img
}

// Measure returns the width in pixels of the given text using the static 8px
// advance per character (ADR-0004). High-bit chars count as one cell.
func (wt *QuakeText) Measure(text string) int {
	return len([]rune(text)) * 8
}

// GlyphFor resolves a character to its conchars atlas index and whether it is
// on the bright row (char + 128, ADR-0004 bright set).
func (wt *QuakeText) GlyphFor(ch rune) (index byte, bright bool) {
	if ch >= 128 {
		return byte(ch), true
	}
	return byte(ch), false
}

// Prompt returns the console input prompt glyph (']').
func (wt *QuakeText) Prompt() rune {
	if wt == nil {
		return ']'
	}
	return wt.prompt
}

// CursorGlyph returns the blink cursor glyph for the given time in seconds.
// The cursor alternates between glyphs 10 and 11 at 4Hz (250ms period),
// matching the legacy console draw (console/draw.go).
func (wt *QuakeText) CursorGlyph(t float64) int {
	if int(t/0.25)&1 == 0 {
		return 10
	}
	return 11
}

// Atlas returns the RGBA conchars atlas (128x128), or nil if none was built.
func (wt *QuakeText) Atlas() *image.RGBA {
	if wt == nil {
		return nil
	}
	return wt.atlas
}

// GlyphImage returns an 8x8 sub-image view into the atlas for a character
// index. The returned image shares the atlas backing pixels (zero-copy).
func (wt *QuakeText) GlyphImage(index byte) image.Image {
	if wt == nil || wt.atlas == nil {
		return nil
	}
	col := int(index) % 16
	row := int(index) / 16
	r := image.Rect(col*8, row*8, col*8+8, row*8+8)
	return wt.atlas.SubImage(r)
}

// DrawString renders the given text at the top-left position (x, y) using
// the conchars atlas, 8px per glyph. High-bit characters (char + 128) are
// drawn from the bright glyph row. Characters with no atlas glyph are skipped.
func (wt *QuakeText) DrawString(canvas widget.Canvas, x, y float32, text string) {
	if wt == nil || wt.atlas == nil || canvas == nil {
		return
	}
	cx := x
	for _, ch := range text {
		if ch < 0 || ch > 255 {
			continue
		}
		img := wt.GlyphImage(byte(ch))
		if img != nil {
			canvas.DrawImage(img, geometry.Pt(cx, y))
		}
		cx += 8
	}
}

// Layout sizes the widget to its content (default 8px glyph cell).
func (wt *QuakeText) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	size := c.Constrain(geometry.Sz(8, 8))
	wt.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	return size
}

// Draw renders the widget. The conchars glyphs are drawn by the host canvas
// via GlyphImage; the widget itself is a leaf (no children) and draws nothing
// beyond its bounds marker.
func (wt *QuakeText) Draw(ctx widget.Context, canvas widget.Canvas) {}

// Event consumes no input.
func (wt *QuakeText) Event(ctx widget.Context, e event.Event) bool { return false }
