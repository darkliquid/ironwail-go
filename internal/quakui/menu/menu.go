// Package menu implements the gogpu/ui menu widget tree for the v2 UI
// rewrite (IRONWAIL-SPEC-002, ADR-0008). It renders the real gfx/*.lmp menu
// art via the quakui pic bridge and conchars bitmap text at the legacy 320x200
// layout positions. The legacy menu.Manager state machine remains the source
// of truth: the widget reads State()/CursorFor()/accessors each frame and
// routes key/char events back through M_Key/M_Char. Only the presentation
// moves to gogpu/ui; the action side is shared verbatim.
//
// The package is self-contained: it imports only the legacy menu/draw state
// machines, internal/quakui (for the pic + conchars bridges), and gogpu/ui.
// It never imports internal/game or internal/renderer (AC7, ADR-0009).
package menu

import (
	"fmt"
	"image"
	"time"

	"github.com/darkliquid/ironwail-go/internal/draw"
	legacymenu "github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/darkliquid/ironwail-go/internal/quakui/gfx"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// MenuRoot is the gogpu/ui menu widget. It reads the legacy menu.Manager
// state each frame and renders the active page (main, singleplayer, options)
// with real LMP pics and conchars text at the legacy 320x200 positions. Key
// and char events are routed back to the manager's M_Key/M_Char action path.
type MenuRoot struct {
	widget.WidgetBase

	mgr        *legacymenu.Manager
	drawMgr    *draw.Manager
	atlas      *gfx.ConcharsAtlas
	lastActive bool
}

// NewMenuRoot builds the menu widget. mgr is the legacy menu state machine;
// drawMgr provides the gfx/*.lmp pics; conchars/palette feed the text atlas.
// drawMgr may be nil (headless tests) — pages fall back to conchars text.
func NewMenuRoot(mgr *legacymenu.Manager, drawMgr *draw.Manager, conchars, palette []byte) *MenuRoot {
	active := mgr != nil && mgr.IsActive()
	r := &MenuRoot{
		mgr:        mgr,
		drawMgr:    drawMgr,
		atlas:      gfx.NewConcharsAtlas(conchars, palette),
		lastActive: active,
	}
	r.SetVisible(active)
	r.SetEnabled(true)
	r.SetRepaintBoundary(true)
	return r
}

// IsVisible reports whether the menu widget should be drawn and accept events.
func (r *MenuRoot) IsVisible() bool {
	if r == nil {
		return false
	}
	if r.mgr != nil {
		active := r.mgr.IsActive()
		if active != r.lastActive {
			r.lastActive = active
			r.SetVisible(active)
			r.SetNeedsRedraw(true)
			r.InvalidateScene()
		}
		return active && r.WidgetBase.IsVisible()
	}
	return r.WidgetBase.IsVisible()
}

// Layout sizes the menu root to the full window. The 320x200 menu viewport
// scaling is applied by the M2.2 CANVAS_MENU transform; the widget draws at
// legacy 320x200 coordinates within the window.
func (r *MenuRoot) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	size := geometry.Sz(c.MaxWidth, c.MaxHeight)
	r.SetBounds(geometry.FromPointSize(geometry.Pt(0, 0), size))
	return size
}

// Draw renders the active page.
func (r *MenuRoot) Draw(ctx widget.Context, canvas widget.Canvas) {
	if r == nil || !r.IsVisible() || r.mgr == nil || canvas == nil {
		return
	}
	if sched, ok := ctx.(widget.AnimationScheduler); ok {
		sched.ScheduleAnimationFrame()
		r.SetNeedsRedraw(true)
	} else if ctx != nil {
		ctx.Invalidate()
	}

	winSize := geometry.Sz(320, 200)
	if ctx != nil {
		winSize = ctx.WindowSize()
	}
	tf := ComputeMenuTransform(MenuScaleParams{
		WindowWidth:  winSize.Width,
		WindowHeight: winSize.Height,
	})

	canvas.PushTransform(geometry.Pt(tf.OriginX, tf.OriginY))
	defer canvas.PopTransform()

	switch r.mgr.State() {
	case legacymenu.MenuMain:
		r.drawMain(canvas)
	case legacymenu.MenuSinglePlayer:
		r.drawSinglePlayer(canvas)
	case legacymenu.MenuOptions:
		r.drawOptions(canvas)
	case legacymenu.MenuLoad:
		r.drawLoad(canvas)
	case legacymenu.MenuSave:
		r.drawSave(canvas)
	case legacymenu.MenuMultiPlayer:
		r.drawMultiPlayer(canvas)
	case legacymenu.MenuJoinGame:
		r.drawJoinGame(canvas)
	case legacymenu.MenuHostGame:
		r.drawHostGame(canvas)
	case legacymenu.MenuControls:
		r.drawControls(canvas)
	case legacymenu.MenuVideo:
		r.drawVideo(canvas)
	case legacymenu.MenuAudio:
		r.drawAudio(canvas)
	case legacymenu.MenuHelp:
		r.drawHelp(canvas)
	case legacymenu.MenuQuit:
		r.drawQuit(canvas)
	case legacymenu.MenuSetup:
		r.drawSetup(canvas)
	case legacymenu.MenuMods:
		r.drawMods(canvas)
	}
}

// Event routes key/char events to the manager action path. Text runes go
// through M_Char in text-entry screens and M_Key otherwise; navigation keys go
// through M_Key. Only press events are forwarded (Quake menus act on press).
func (r *MenuRoot) Event(ctx widget.Context, e event.Event) bool {
	if r == nil || !r.IsVisible() || r.mgr == nil {
		return false
	}
	ke, ok := e.(*event.KeyEvent)
	if !ok || ke.KeyType != event.KeyPress {
		return false
	}
	if ke.Rune != 0 {
		switch r.mgr.State() {
		case legacymenu.MenuSetup, legacymenu.MenuJoinGame, legacymenu.MenuHostGame:
			r.mgr.M_Char(ke.Rune)
		default:
			if ke.Rune < 128 {
				r.mgr.M_Key(int(ke.Rune))
			}
		}
		r.SetNeedsRedraw(true)
		r.InvalidateScene()
		if ctx != nil {
			ctx.Invalidate()
		}
		return true
	}
	if key := keyEventToEngine(ke); key >= 0 {
		r.mgr.M_Key(key)
		r.SetNeedsRedraw(true)
		r.InvalidateScene()
		if ctx != nil {
			ctx.Invalidate()
		}
		return true
	}
	return false
}

// pic returns the RGBA image for a gfx/*.lmp name, or nil if unavailable.
func (r *MenuRoot) pic(name string) *image.RGBA {
	if r == nil || r.drawMgr == nil {
		return nil
	}
	q := r.drawMgr.Pic(name)
	if q == nil {
		return nil
	}
	pal := []byte(nil)
	if r.drawMgr != nil {
		pal = r.drawMgr.Palette()
	}
	return gfx.QPicToImage(q, pal)
}

// subPic returns a rectangular sub-image view of an RGBA pic (zero-copy).
func subPic(img *image.RGBA, x, y, w, h int) *image.RGBA {
	if img == nil {
		return nil
	}
	b := img.Bounds()
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+w > b.Dx() {
		w = b.Dx() - x
	}
	if y+h > b.Dy() {
		h = b.Dy() - y
	}
	if w <= 0 || h <= 0 {
		return nil
	}
	sub := img.SubImage(image.Rect(b.Min.X+x, b.Min.Y+y, b.Min.X+x+w, b.Min.Y+y+h))
	if rgba, ok := sub.(*image.RGBA); ok {
		return rgba
	}
	return nil
}

// drawPic draws a menu pic at legacy 320x200 coordinates.
func (r *MenuRoot) drawPic(canvas widget.Canvas, x, y int, img *image.RGBA) {
	if img == nil || canvas == nil {
		return
	}
	canvas.DrawImage(img, geometry.Pt(float32(x), float32(y)))
}

func (r *MenuRoot) ensureAtlas() *gfx.ConcharsAtlas {
	if r == nil {
		return nil
	}
	if r.atlas != nil {
		return r.atlas
	}
	if r.drawMgr != nil {
		if conchars := r.drawMgr.ConcharsData(); len(conchars) >= 128*128 {
			r.atlas = gfx.NewConcharsAtlas(conchars, r.drawMgr.Palette())
		}
	}
	return r.atlas
}

// drawText renders a conchars string at legacy 320x200 coordinates. If white
// is true the high-bit bright row is used (char + 128), matching the legacy
// drawText.
func (r *MenuRoot) drawText(canvas widget.Canvas, x, y int, text string, white bool) {
	atlas := r.ensureAtlas()
	if r == nil || atlas == nil || canvas == nil {
		return
	}
	cx := float32(x)
	for _, ch := range text {
		if ch == 0 || ch == ' ' {
			cx += 8
			continue
		}
		index := ch
		if ch < 0 || ch > 255 {
			index = '?'
		}
		if white {
			index += 128
			if index > 255 {
				index = 255
			}
		}
		if img := atlas.GlyphImage(byte(index)); img != nil {
			canvas.DrawImage(img, geometry.Pt(cx, float32(y)))
		}
		cx += 8
	}
}

// drawCursor renders the animated menu cursor (spinning Quake dot) at the
// given legacy position. It picks menudot1..6 by time, falling back to
// m_surfs.lmp then a plain glyph, mirroring the legacy drawCursor.
func (r *MenuRoot) drawCursor(canvas widget.Canvas, x, y int) {
	frame := (time.Now().UnixNano()/int64(200*time.Millisecond))%6 + 1
	if img := r.pic(fmt.Sprintf("gfx/menudot%d.lmp", frame)); img != nil {
		r.drawPic(canvas, x, y, img)
		return
	}
	if img := r.pic("gfx/m_surfs.lmp"); img != nil {
		r.drawPic(canvas, x, y, img)
		return
	}
	r.drawText(canvas, x, y, string(rune(12)), false)
}