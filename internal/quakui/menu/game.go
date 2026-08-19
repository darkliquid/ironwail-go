package menu

import (
	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/gogpu/ui/widget"
)

// drawLoad renders the Load Game menu showing all save slots with their labels
// and a blinking arrow cursor.
func (r *MenuRoot) drawLoad(canvas widget.Canvas) {
	r.drawPlaqueAndTitle(canvas, "gfx/p_load.lmp")

	slots := r.mgr.SaveSlots()
	for i, label := range slots {
		r.drawText(canvas, 24, 32+i*8, label, true)
	}
	r.drawArrowCursor(canvas, 8, 32+r.mgr.CursorFor(menu.MenuLoad)*8)
}

// drawSave renders the Save Game menu showing all save slots with their labels
// and a blinking arrow cursor.
func (r *MenuRoot) drawSave(canvas widget.Canvas) {
	r.drawPlaqueAndTitle(canvas, "gfx/p_save.lmp")

	slots := r.mgr.SaveSlots()
	for i, label := range slots {
		r.drawText(canvas, 24, 32+i*8, label, true)
	}
	r.drawArrowCursor(canvas, 8, 32+r.mgr.CursorFor(menu.MenuSave)*8)
}
