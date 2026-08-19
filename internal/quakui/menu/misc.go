package menu

import (
	"fmt"
	"image"
	"time"

	"github.com/darkliquid/ironwail-go/internal/quakui/gfx"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// drawHelp renders one of the six help screens. If the corresponding graphic
// (gfx/helpN.lmp) is available it fills the screen; otherwise a text fallback
// with page number and navigation instructions is shown.
func (r *MenuRoot) drawHelp(canvas widget.Canvas) {
	picName := fmt.Sprintf("gfx/help%d.lmp", r.mgr.HelpPage())
	if pic := r.pic(picName); pic != nil {
		r.drawPic(canvas, 0, 0, pic)
		return
	}

	r.drawPlaqueAndTitle(canvas, "gfx/ttl_main.lmp")
	r.drawText(canvas, 48, 64, "HELP PAGE", true)
	r.drawText(canvas, 136, 64, fmt.Sprintf("%d/6", r.mgr.HelpPage()+1), true)
	r.drawText(canvas, 48, 88, "LEFT/RIGHT OR MOUSE1 TO CHANGE", true)
	r.drawText(canvas, 48, 104, "ESC TO RETURN", true)
}

// drawQuit renders the quit confirmation screen.
func (r *MenuRoot) drawQuit(canvas widget.Canvas) {
	r.drawPlaqueAndTitle(canvas, "")
	lines := r.mgr.ConfirmLines()
	if lines[0] == "" && lines[1] == "" && lines[2] == "" {
		lines = [3]string{
			"ARE YOU SURE YOU WANT TO QUIT?",
			"PRESS Y OR ENTER TO QUIT",
			"PRESS N OR ESC TO CANCEL",
		}
	}
	r.drawText(canvas, 56, 64, lines[0], true)
	r.drawText(canvas, 56, 88, lines[1], true)
	r.drawText(canvas, 56, 104, lines[2], true)
}

// drawMenuTextBox draws a 9-patch text box using box_*.lmp graphics. width is in
// 16-pixel columns and lines is the number of 8-pixel text rows inside.
func (r *MenuRoot) drawMenuTextBox(canvas widget.Canvas, x, y, width, lines int) {
	cx := x
	cy := y

	if pic := r.pic("gfx/box_tl.lmp"); pic != nil {
		r.drawPic(canvas, cx, cy, pic)
	}
	if pic := r.pic("gfx/box_ml.lmp"); pic != nil {
		for n := 0; n < lines; n++ {
			cy += 8
			r.drawPic(canvas, cx, cy, pic)
		}
	}
	if pic := r.pic("gfx/box_bl.lmp"); pic != nil {
		r.drawPic(canvas, cx, cy+8, pic)
	}

	cx += 8
	for remaining := width; remaining > 0; remaining -= 2 {
		cy = y
		if pic := r.pic("gfx/box_tm.lmp"); pic != nil {
			r.drawPic(canvas, cx, cy, pic)
		}
		for n := 0; n < lines; n++ {
			cy += 8
			name := "gfx/box_mm.lmp"
			if n == 1 {
				name = "gfx/box_mm2.lmp"
			}
			if pic := r.pic(name); pic != nil {
				r.drawPic(canvas, cx, cy, pic)
			}
		}
		if pic := r.pic("gfx/box_bm.lmp"); pic != nil {
			r.drawPic(canvas, cx, cy+8, pic)
		}
		cx += 16
	}

	cy = y
	if pic := r.pic("gfx/box_tr.lmp"); pic != nil {
		r.drawPic(canvas, cx, cy, pic)
	}
	if pic := r.pic("gfx/box_mr.lmp"); pic != nil {
		for n := 0; n < lines; n++ {
			cy += 8
			r.drawPic(canvas, cx, cy, pic)
		}
	}
	if pic := r.pic("gfx/box_br.lmp"); pic != nil {
		r.drawPic(canvas, cx, cy+8, pic)
	}
}

// drawArrowCursor renders the blinking text-mode arrow cursor (characters 12/13).
func (r *MenuRoot) drawArrowCursor(canvas widget.Canvas, x, y int) {
	char := 12 + int((time.Now().UnixNano()/int64(250*time.Millisecond))&1)
	r.drawCharacter(canvas, x, y, char)
}

// drawCharacter renders a single character index from conchars at (x, y).
func (r *MenuRoot) drawCharacter(canvas widget.Canvas, x, y, char int) {
	if r == nil || r.atlas == nil || canvas == nil {
		return
	}
	if char < 0 || char > 255 {
		char = '?'
	}
	if img := r.atlas.GlyphImage(byte(char)); img != nil {
		canvas.DrawImage(img, geometry.Pt(float32(x), float32(y)))
	}
}

// blinkingCursorChar returns character 10 or 11 alternating every 250ms for text fields.
func blinkingCursorChar() int {
	return 10 + int((time.Now().UnixNano()/int64(250*time.Millisecond))&1)
}

// setupPlayerPic returns the color-remapped player sprite for the Setup screen.
func (r *MenuRoot) setupPlayerPic() *image.RGBA {
	if r == nil || r.drawMgr == nil || r.mgr == nil {
		return nil
	}
	pic := r.drawMgr.Pic("gfx/menuplyr.lmp")
	if pic == nil {
		return nil
	}
	return gfx.QPicToImageTranslated(pic, r.drawMgr.Palette(), r.mgr.SetupTopColor(), r.mgr.SetupBottomColor())
}

func boolLabel(val bool) string {
	if val {
		return "ON"
	}
	return "OFF"
}
