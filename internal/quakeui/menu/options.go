package menu

import (
	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/gogpu/ui/widget"
)

// drawOptions renders the Options sub-menu with its five items: Controls,
// Video, Audio, VSync, and Back (legacy drawOptions, menu_options.go). The
// Options page has no LMP art beyond the plaque/title, so it is conchars text
// plus the animated cursor.
func (r *MenuRoot) drawOptions(canvas widget.Canvas) {
	r.drawPlaqueAndTitle(canvas, "gfx/p_option.lmp")

	r.drawText(canvas, 84, 32, "CONTROLS", true)
	r.drawText(canvas, 84, 52, "VIDEO", true)
	r.drawText(canvas, 84, 72, "AUDIO", true)
	r.drawText(canvas, 84, 92, "VSYNC", true)
	r.drawText(canvas, 84, 112, "BACK", true)

	r.drawCursor(canvas, 54, 32+r.mgr.CursorFor(menu.MenuOptions)*20)
}