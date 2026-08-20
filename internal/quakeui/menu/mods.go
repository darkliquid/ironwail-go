package menu

import (
	"strings"

	legacymenu "github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/gogpu/ui/widget"
)

// drawMods renders the Mods browser screen.
func (r *MenuRoot) drawMods(canvas widget.Canvas) {
	r.drawPlaqueAndTitle(canvas, "gfx/ttl_main.lmp")
	r.drawText(canvas, 84, 16, "MODS", true)

	mods := r.mgr.Mods()
	if len(mods) == 0 {
		r.drawText(canvas, 48, 56, "NO MODS FOUND", true)
		r.drawText(canvas, 48, 80, "PLACE MOD DIRECTORIES", true)
		r.drawText(canvas, 48, 96, "NEXT TO ID1 IN YOUR", true)
		r.drawText(canvas, 48, 112, "QUAKE DIRECTORY", true)
		r.drawText(canvas, 48, 136, "BACK", true)
		r.drawArrowCursor(canvas, 32, 136)
		return
	}

	const startY = 32
	const lineH = 8
	currentMod := r.mgr.CurrentMod()
	for i, mod := range mods {
		y := startY + i*lineH
		label := strings.ToUpper(mod.Name)
		if strings.EqualFold(mod.Name, currentMod) {
			label += " *"
		}
		r.drawText(canvas, 48, y, label, true)
	}

	backY := startY + len(mods)*lineH + lineH
	r.drawText(canvas, 48, backY, "BACK", true)

	cursor := r.mgr.CursorFor(legacymenu.MenuMods)
	cursorY := startY + cursor*lineH
	if cursor == len(mods) {
		cursorY = backY
	}
	r.drawArrowCursor(canvas, 32, cursorY)
}
