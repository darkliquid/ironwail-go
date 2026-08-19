package menu

import (
	"fmt"

	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/gogpu/ui/widget"
)

// drawVideo renders the Video settings menu.
func (r *MenuRoot) drawVideo(canvas widget.Canvas) {
	r.drawPlaqueAndTitle(canvas, "gfx/p_option.lmp")

	rows := r.mgr.VideoRows()
	for i, row := range rows {
		y := 32 + i*8
		r.drawText(canvas, 56, y, row.Label, true)
		if row.Value != "" {
			r.drawText(canvas, 184, y, row.Value, true)
		}
	}

	r.drawArrowCursor(canvas, 40, 32+r.mgr.CursorFor(menu.MenuVideo)*8)
	r.drawText(canvas, 40, 192, "VIDEO CHANGES ARE SAVED TO CONFIG", true)
}

// drawControls renders the Controls settings menu.
func (r *MenuRoot) drawControls(canvas widget.Canvas) {
	r.drawPlaqueAndTitle(canvas, "gfx/p_option.lmp")

	r.drawText(canvas, 32, 32, "MOUSE SPEED", true)
	r.drawText(canvas, 208, 32, fmt.Sprintf("%.1f", r.mgr.ControlMouseSpeed()), true)
	r.drawText(canvas, 32, 40, "INVERT MOUSE", true)
	r.drawText(canvas, 208, 40, boolLabel(r.mgr.ControlInvertMouse()), true)
	r.drawText(canvas, 32, 48, "ALWAYS RUN", true)
	r.drawText(canvas, 208, 48, boolLabel(r.mgr.ControlAlwaysRun()), true)
	r.drawText(canvas, 32, 56, "MOUSE LOOK", true)
	r.drawText(canvas, 208, 56, boolLabel(r.mgr.ControlFreeLook()), true)

	bindings := r.mgr.ControlBindings()
	for i, b := range bindings {
		y := 64 + i*8
		r.drawText(canvas, 40, y, b.Label, true)
		r.drawText(canvas, 200, y, b.Binding, true)
	}
	backY := 64 + len(bindings)*8
	r.drawText(canvas, 40, backY, "BACK", true)

	cursor := r.mgr.CursorFor(menu.MenuControls)
	cursorY := 32 + cursor*8
	if cursor >= 4 {
		cursorY = 64 + (cursor-4)*8
	}
	if cursor == 4+len(bindings) {
		cursorY = backY
	}
	r.drawArrowCursor(canvas, 24, cursorY)

	if r.mgr.ControlsRebinding() {
		r.drawText(canvas, 24, 176, "PRESS A KEY OR ESC TO CANCEL", true)
		return
	}
	if cursor < 4 {
		r.drawText(canvas, 24, 176, "LEFT/RIGHT/ENTER CHANGE, ESC BACK", true)
		return
	}
	r.drawText(canvas, 24, 176, "ENTER/RIGHT BIND LEFT/BKSP CLEAR", true)
}

// drawAudio renders the Audio settings menu.
func (r *MenuRoot) drawAudio(canvas widget.Canvas) {
	r.drawPlaqueAndTitle(canvas, "gfx/p_option.lmp")

	r.drawText(canvas, 72, 56, "SOUND VOLUME", true)
	r.drawText(canvas, 200, 56, fmt.Sprintf("%d%%", r.mgr.AudioVolume()), true)
	r.drawText(canvas, 72, 88, "BACK", true)

	r.drawArrowCursor(canvas, 56, 56+r.mgr.CursorFor(menu.MenuAudio)*32)
}
