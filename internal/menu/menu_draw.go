package menu

import (
	"fmt"
	"time"

	"github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/renderer"
)

// drawMenuTextBox draws a 9-patch text box (top-left/mid/right, mid-left/mid/
// right, bottom-left/mid/right) using the box_*.lmp graphics. width is in
// 16-pixel columns and lines is the number of 8-pixel text rows inside.
func (m *Manager) drawMenuTextBox(dc renderer.RenderContext, x, y, width, lines int) {
	cx := x
	cy := y

	if pic := m.getPic("gfx/box_tl.lmp"); pic != nil {
		dc.DrawMenuPic(cx, cy, pic)
	}
	if pic := m.getPic("gfx/box_ml.lmp"); pic != nil {
		for n := 0; n < lines; n++ {
			cy += 8
			dc.DrawMenuPic(cx, cy, pic)
		}
	}
	if pic := m.getPic("gfx/box_bl.lmp"); pic != nil {
		dc.DrawMenuPic(cx, cy+8, pic)
	}

	cx += 8
	for remaining := width; remaining > 0; remaining -= 2 {
		cy = y
		if pic := m.getPic("gfx/box_tm.lmp"); pic != nil {
			dc.DrawMenuPic(cx, cy, pic)
		}
		for n := 0; n < lines; n++ {
			cy += 8
			name := "gfx/box_mm.lmp"
			if n == 1 {
				name = "gfx/box_mm2.lmp"
			}
			if pic := m.getPic(name); pic != nil {
				dc.DrawMenuPic(cx, cy, pic)
			}
		}
		if pic := m.getPic("gfx/box_bm.lmp"); pic != nil {
			dc.DrawMenuPic(cx, cy+8, pic)
		}
		cx += 16
	}

	cy = y
	if pic := m.getPic("gfx/box_tr.lmp"); pic != nil {
		dc.DrawMenuPic(cx, cy, pic)
	}
	if pic := m.getPic("gfx/box_mr.lmp"); pic != nil {
		for n := 0; n < lines; n++ {
			cy += 8
			dc.DrawMenuPic(cx, cy, pic)
		}
	}
	if pic := m.getPic("gfx/box_br.lmp"); pic != nil {
		dc.DrawMenuPic(cx, cy+8, pic)
	}
}

// translateSetupPlayerPic creates a copy of the player sprite with the shirt
// (palette indices 16–31) and pants (palette indices 96–111) colour ranges
// remapped to the selected top and bottom colours. This mirrors the player
// color translation used in C Quake's R_TranslatePlayerSkin().
func translateSetupPlayerPic(pic *image.QPic, topColor, bottomColor int) *image.QPic {
	if pic == nil {
		return nil
	}

	return &image.QPic{
		Width:  pic.Width,
		Height: pic.Height,
		Pixels: renderer.TranslatePlayerSkinPixels(pic.Pixels, topColor, bottomColor),
	}
}

// drawPlaqueAndTitle draws the standard Quake menu frame: the Quake plaque
// graphic on the left (gfx/qplaque.lmp) and an optional title banner centered
// at the top. Most menu pages call this first to establish the visual frame.
func (m *Manager) drawPlaqueAndTitle(dc renderer.RenderContext, titlePic string) {
	if pic := m.getPic("gfx/qplaque.lmp"); pic != nil {
		dc.DrawMenuPic(16, 4, pic)
	}

	if titlePic == "" {
		return
	}

	if pic := m.getPic(titlePic); pic != nil {
		x := (320 - int(pic.Width)) / 2
		dc.DrawMenuPic(x, 4, pic)
	}
}

// drawCursor draws the animated menu cursor (spinning Quake dot) at the given
// position. Falls back to m_surfs.lmp, then a plain character glyph if no
// animation frames are available.
func (m *Manager) drawCursor(dc renderer.RenderContext, x, y int) {
	frame := (time.Now().UnixNano()/int64(200*time.Millisecond))%6 + 1
	picName := fmt.Sprintf("gfx/menudot%d.lmp", frame)
	if pic := m.getPic(picName); pic != nil {
		dc.DrawMenuPic(x, y, pic)
		return
	}

	if pic := m.getPic("gfx/m_surfs.lmp"); pic != nil {
		dc.DrawMenuPic(x, y, pic)
		return
	}

	dc.DrawMenuCharacter(x, y, 12)
}

// drawArrowCursor draws a blinking text-mode arrow cursor (characters 12/13)
// that alternates every 250 ms. Used for list-style menus (Load, Save, Controls).
func (m *Manager) drawArrowCursor(dc renderer.RenderContext, x, y int) {
	char := 12 + int((time.Now().UnixNano()/int64(250*time.Millisecond))&1)
	dc.DrawMenuCharacter(x, y, char)
}

// drawText renders a string of characters at the given position using Quake's
// 8×8 character glyphs. If white is true, 128 is added to each character code
// to select the "white" (bright) character set; otherwise the default brownish
// set is used.
func (m *Manager) drawText(dc renderer.RenderContext, x, y int, text string, white bool) {
	runes := []rune(text)
	for idx, r := range runes {
		ch := int(r)
		// Quake fonts only support 0–255; replace unsupported runes with '?'.
		if ch < 0 || ch > 255 {
			ch = int('?')
		}
		if white {
			ch += 128
			if ch > 255 {
				ch = 255
			}
		}
		dc.DrawMenuCharacter(x+idx*8, y, ch)
	}
}
