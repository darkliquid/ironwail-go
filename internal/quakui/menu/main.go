package menu

import (
	"log/slog"

	"github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/gogpu/ui/widget"
)

// drawPlaqueAndTitle draws the standard Quake menu frame: the Quake plaque
// graphic on the left (gfx/qplaque.lmp) and an optional title banner centered
// at the top (legacy drawPlaqueAndTitle, menu_draw.go).
func (r *MenuRoot) drawPlaqueAndTitle(canvas widget.Canvas, titlePic string) {
	if img := r.pic("gfx/qplaque.lmp"); img != nil {
		r.drawPic(canvas, 16, 4, img)
	}
	if titlePic == "" {
		return
	}
	if img := r.pic(titlePic); img != nil {
		x := (320 - img.Bounds().Dx()) / 2
		r.drawPic(canvas, x, 4, img)
	}
}

// drawMain renders the main menu (legacy drawMain, menu_main.go): the Quake
// plaque + ttl_main title, the mainmenu.lmp art (split when mods are present),
// and the animated cursor at the active item.
func (r *MenuRoot) drawMain(canvas widget.Canvas) {
	r.drawPlaqueAndTitle(canvas, "gfx/ttl_main.lmp")

	pic := r.pic("gfx/mainmenu.lmp")
	if r.drawCount <= 5 {
		slog.Debug("quakui drawMain",
			"has_plaque", r.pic("gfx/qplaque.lmp") != nil,
			"has_title", r.pic("gfx/ttl_main.lmp") != nil,
			"has_mainmenu", pic != nil,
		)
	}
	if pic == nil {
		// Text-only fallback (no graphics loaded).
		r.drawText(canvas, 84, 32, "SINGLE PLAYER", true)
		r.drawText(canvas, 84, 52, "MULTIPLAYER", true)
		r.drawText(canvas, 84, 72, "OPTIONS", true)
		if len(r.mgr.Mods()) > 0 {
			r.drawText(canvas, 84, 92, "MODS", true)
			r.drawText(canvas, 84, 112, "HELP", true)
			r.drawText(canvas, 84, 132, "QUIT", true)
		} else {
			r.drawText(canvas, 84, 92, "HELP", true)
			r.drawText(canvas, 84, 112, "QUIT", true)
		}
		r.drawMainCursor(canvas)
		return
	}

	if len(r.mgr.Mods()) > 0 {
		// Split the graphic and insert MODS between OPTIONS and HELP.
		const split = 60 // pixel row to split at (after OPTIONS)
		r.drawPic(canvas, 72, 32, subPic(pic, 0, 0, pic.Bounds().Dx(), split))
		if modsPic := r.pic("gfx/menumods.lmp"); modsPic != nil {
			r.drawPic(canvas, 72, 32+split, modsPic)
		} else {
			r.drawText(canvas, 74, 32+split+1, "MODS", true)
		}
		r.drawPic(canvas, 72, 32+split+20, subPic(pic, 0, split, pic.Bounds().Dx(), pic.Bounds().Dy()-split))
	} else {
		r.drawPic(canvas, 72, 32, pic)
	}

	r.drawMainCursor(canvas)
}

// drawMainCursor draws the animated main menu cursor at the correct visual
// position (legacy drawMainCursor). When no mods are present, items after the
// mods slot shift up visually.
func (r *MenuRoot) drawMainCursor(canvas widget.Canvas) {
	cursor := r.mgr.CursorFor(menu.MenuMain)
	// When no mods, items after the mods slot shift up visually.
	if len(r.mgr.Mods()) == 0 && cursor > 3 {
		cursor--
	}
	r.drawCursor(canvas, 54, 32+cursor*20)
}

// drawSinglePlayer renders the Single Player sub-menu with its three items
// (New Game, Load, Save) using either a graphic sprite or text fallback
// (legacy drawSinglePlayer).
func (r *MenuRoot) drawSinglePlayer(canvas widget.Canvas) {
	r.drawPlaqueAndTitle(canvas, "gfx/ttl_sgl.lmp")

	if pic := r.pic("gfx/sp_menu.lmp"); pic != nil {
		r.drawPic(canvas, 72, 32, pic)
	} else {
		r.drawText(canvas, 84, 32, "NEW GAME", true)
		r.drawText(canvas, 84, 52, "LOAD", true)
		r.drawText(canvas, 84, 72, "SAVE", true)
	}

	r.drawCursor(canvas, 54, 32+r.mgr.CursorFor(menu.MenuSinglePlayer)*20)
}