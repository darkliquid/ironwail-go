package console

import (
	"image"
	"strings"
	"time"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/draw"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/quakui/gfx"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// ConsoleRoot is the top-level gogpu/ui widget rendering Quake's drop-down
// console and fading notification messages (ADR-0008, M3.1a).
type ConsoleRoot struct {
	widget.WidgetBase

	con           *console.Console
	drawMgr       *draw.Manager
	atlas         *gfx.ConcharsAtlas
	slideFraction float32
	forcedUp      bool
	onCommand     func(cmd string)
	onToggle      func()
	matches       []string

	// Cached converted background pic
	conbackImg *image.RGBA
}

// NewConsoleRoot constructs a new ConsoleRoot widget.
func NewConsoleRoot(con *console.Console, drawMgr *draw.Manager, atlas *gfx.ConcharsAtlas) *ConsoleRoot {
	if con == nil {
		con = console.Global()
	}
	r := &ConsoleRoot{
		con:           con,
		drawMgr:       drawMgr,
		atlas:         atlas,
		slideFraction: 0,
	}
	r.SetVisible(true)
	r.SetEnabled(true)
	r.SetRepaintBoundary(true)
	return r
}

// IsVisible reports whether the console widget is active (dropdown open/animating, forced up, or fading notifications).
func (r *ConsoleRoot) IsVisible() bool {
	if r == nil {
		return false
	}
	active := r.slideFraction > 0 || r.forcedUp
	if !active && r.con != nil {
		active = r.con.HasNotify()
	}
	return active
}

// SetSlideFraction updates the drop-down slide animation progress (0.0 to 1.0).
func (r *ConsoleRoot) SetSlideFraction(f float32) {
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	if r.slideFraction != f {
		r.slideFraction = f
		r.SetVisible(f > 0 || r.forcedUp)
		r.SetNeedsRedraw(true)
		r.InvalidateScene()
	}
}

// SlideFraction returns the current drop-down slide fraction.
func (r *ConsoleRoot) SlideFraction() float32 {
	return r.slideFraction
}

// SetForcedUp configures whether the console must fill the full screen (e.g. before level load).
func (r *ConsoleRoot) SetForcedUp(forced bool) {
	if r.forcedUp != forced {
		r.forcedUp = forced
		r.SetVisible(forced || r.slideFraction > 0)
		r.SetNeedsRedraw(true)
		r.InvalidateScene()
	}
}

// IsForcedUp reports whether the console is forced full screen.
func (r *ConsoleRoot) IsForcedUp() bool {
	return r.forcedUp
}

// SetOnCommand sets the callback invoked when a command is entered (Enter key).
func (r *ConsoleRoot) SetOnCommand(fn func(cmd string)) {
	r.onCommand = fn
}

// SetOnToggle sets the callback invoked when the console toggle key is pressed.
func (r *ConsoleRoot) SetOnToggle(fn func()) {
	r.onToggle = fn
}

// Layout sizes the console widget to occupy the available window bounds.
func (r *ConsoleRoot) Layout(ctx widget.Context, c geometry.Constraints) geometry.Size {
	w := c.MaxWidth
	h := c.MaxHeight
	if w <= 0 {
		w = 320
	}
	if h <= 0 {
		h = 200
	}
	charsWide := int(w) / 8
	if charsWide > 2 && r.con != nil {
		r.con.Resize(charsWide - 2)
	}
	sz := geometry.Sz(w, h)
	r.SetBounds(geometry.NewRect(0, 0, w, h))
	return sz
}

// Draw renders either the full drop-down console or the floating notification lines.
func (r *ConsoleRoot) Draw(ctx widget.Context, canvas widget.Canvas) {
	if r == nil || canvas == nil {
		return
	}

	winSize := geometry.Sz(320, 200)
	if ctx != nil {
		winSize = ctx.WindowSize()
	}

	// Schedule animation frame if animating or active
	if r.slideFraction > 0 || r.forcedUp {
		if sched, ok := ctx.(interface{ ScheduleAnimationFrame() }); ok {
			sched.ScheduleAnimationFrame()
			r.SetNeedsRedraw(true)
		} else if ctx != nil {
			ctx.Invalidate()
		}
		r.drawDropdown(canvas, winSize.Width, winSize.Height)
		return
	}

	// In notify mode, check if any active notify lines exist to schedule redraws
	r.drawNotify(canvas, winSize.Width)
}

// drawDropdown renders the full drop-down console background, text, prompt, and title.
func (r *ConsoleRoot) drawDropdown(canvas widget.Canvas, screenW, screenH float32) {
	conH := screenH * 0.5
	if r.forcedUp || conH < 32 {
		conH = screenH
	}

	visibleH := conH * r.slideFraction
	if r.forcedUp {
		visibleH = conH
	}
	if visibleH <= 0 {
		return
	}

	charsWide := int(screenW) / 8
	if charsWide < 2 {
		charsWide = 2
	}

	// Background: solid black base for dropdown overlay
	canvas.DrawRect(geometry.NewRect(0, 0, screenW, visibleH), widget.ColorBlack)
	if r.drawMgr != nil {
		reqW := int(screenW)
		reqH := int(conH)
		if r.conbackImg == nil || r.conbackImg.Bounds().Dx() != reqW || r.conbackImg.Bounds().Dy() != reqH {
			pic := r.drawMgr.Pic("gfx/conback.lmp")
			if pic == nil {
				pic = r.drawMgr.Pic("conback")
			}
			if pic != nil {
				r.conbackImg = scalePicRGBA(pic, r.drawMgr.Palette(), reqW, reqH)
			}
		}
	}
	if r.conbackImg != nil && visibleH > 0 {
		srcY := int(conH - visibleH)
		if srcY < 0 {
			srcY = 0
		}
		visH := int(visibleH)
		if srcY+visH > r.conbackImg.Bounds().Dy() {
			visH = r.conbackImg.Bounds().Dy() - srcY
		}
		if visH > 0 {
			visImg := image.NewRGBA(image.Rect(0, 0, int(screenW), visH))
			srcOffset := srcY * r.conbackImg.Stride
			dstOffset := 0
			rowBytes := int(screenW) * 4
			if rowBytes > r.conbackImg.Stride {
				rowBytes = r.conbackImg.Stride
			}
			for y := 0; y < visH; y++ {
				copy(visImg.Pix[dstOffset:dstOffset+rowBytes], r.conbackImg.Pix[srcOffset:srcOffset+rowBytes])
				srcOffset += r.conbackImg.Stride
				dstOffset += visImg.Stride
			}
			canvas.DrawImage(visImg, geometry.Pt(0, 0))
		}
	}

	visibleRows := int(visibleH)/8 - 1
	if visibleRows < 1 {
		visibleRows = 1
	}

	snap := r.con.SnapshotFull(visibleRows)

	// Scroll indicator row
	if snap.BackScroll > 0 {
		r.drawText(canvas, 8, 0, strings.Repeat("^", charsWide-2), false)
	}

	// Scrollback lines
	y := int(visibleH) - 8 - len(snap.Lines)*8
	for _, line := range snap.Lines {
		if y >= 0 && y < int(visibleH)-8 {
			r.drawText(canvas, 8, y, line, false)
		}
		y += 8
	}

	// Input line + blinking cursor
	inputY := int(visibleH) - 8
	if inputY >= 0 {
		// Render candidate matches list above the prompt if multiple candidates exist
		if len(r.matches) > 1 {
			matchY := inputY - 8
			for i := len(r.matches) - 1; i >= 0; i-- {
				if matchY < 0 {
					break
				}
				r.drawText(canvas, 16, matchY, r.matches[i], true)
				matchY -= 8
			}
		}

		prompt := "]" + snap.InputLine
		r.drawText(canvas, 8, inputY, prompt, false)
		cursorX := 8 + (snap.CursorPos+1)*8
		r.drawCursor(canvas, cursorX, inputY)

		// Title / version string in bottom-right
		if len(snap.Title) > 0 {
			titleX := int(screenW) - len(snap.Title)*8 - 8
			if titleX > cursorX+16 {
				r.drawText(canvas, titleX, inputY, snap.Title, true)
			}
		}
	}
}

// drawNotify renders floating notification lines at the top of the screen when console is closed.
func (r *ConsoleRoot) drawNotify(canvas widget.Canvas, screenW float32) {
	if r.con == nil {
		return
	}
	notifies := r.con.SnapshotNotify()
	if len(notifies) == 0 {
		return
	}

	y := 0
	for _, line := range notifies {
		if line.Alpha <= 0 {
			continue
		}
		r.drawText(canvas, 8, y, line.Text, false)
		y += 8
	}
}

// drawText renders a line of conchars text at (x, y).
func (r *ConsoleRoot) drawText(canvas widget.Canvas, x, y int, text string, white bool) {
	if r == nil || r.atlas == nil || canvas == nil {
		return
	}
	for i, ch := range []byte(text) {
		if ch == 0 || ch == ' ' {
			continue
		}
		code := int(ch)
		if white {
			code += 128
		}
		if code < 0 || code > 255 {
			code = '?'
		}
		if img := r.atlas.GlyphImage(byte(code)); img != nil {
			canvas.DrawImage(img, geometry.Pt(float32(x+i*8), float32(y)))
		}
	}
}

// drawCursor renders the blinking cursor character.
func (r *ConsoleRoot) drawCursor(canvas widget.Canvas, x, y int) {
	if r == nil || r.atlas == nil || canvas == nil {
		return
	}
	frame := 10 + int((time.Now().UnixNano()/int64(250*time.Millisecond))&1)
	if img := r.atlas.GlyphImage(byte(frame)); img != nil {
		canvas.DrawImage(img, geometry.Pt(float32(x), float32(y)))
	}
}

// Event routes key and character events when the console is active.
func (r *ConsoleRoot) Event(ctx widget.Context, e event.Event) bool {
	if r == nil || r.con == nil {
		return false
	}

	// Only process events when the console dropdown is open or opening
	if r.slideFraction <= 0 && !r.forcedUp {
		return false
	}

	ke, ok := e.(*event.KeyEvent)
	if !ok || ke.KeyType != event.KeyPress {
		return false
	}

	ctrl := ke.IsCtrl()

	if ke.Key != event.KeyTab {
		r.matches = nil
	}

	markDirty := func() {
		r.SetNeedsRedraw(true)
		r.InvalidateScene()
		if ctx != nil {
			ctx.Invalidate()
		}
	}

	switch ke.Key {
	case event.KeyGrave:
		if r.onToggle != nil {
			r.onToggle()
		}
		markDirty()
		return true
	case event.KeyEscape:
		if !r.forcedUp && r.onToggle != nil {
			r.onToggle()
			markDirty()
			return true
		}
	case event.KeyEnter:
		cmd := r.con.CommitInput()
		if len(cmd) > 0 && r.onCommand != nil {
			r.onCommand(cmd)
		}
		markDirty()
		return true
	case event.KeyTab:
		forward := !ke.Modifiers().IsShift()
		r.Complete(forward)
		markDirty()
		return true
	case event.KeyUp:
		r.con.PreviousHistory()
		markDirty()
		return true
	case event.KeyDown:
		r.con.NextHistory()
		markDirty()
		return true
	case event.KeyPageUp:
		r.con.Scroll(5)
		markDirty()
		return true
	case event.KeyPageDown:
		r.con.Scroll(-5)
		markDirty()
		return true
	case event.KeyBackspace:
		r.con.BackspaceInput()
		markDirty()
		return true
	case event.KeyDelete:
		r.con.DeleteInput()
		markDirty()
		return true
	case event.KeyLeft:
		r.con.MoveCursorLeft(ctrl)
		markDirty()
		return true
	case event.KeyRight:
		r.con.MoveCursorRight(ctrl)
		markDirty()
		return true
	case event.KeyHome:
		r.con.MoveCursorStart()
		markDirty()
		return true
	case event.KeyEnd:
		r.con.MoveCursorEnd()
		markDirty()
		return true
	}

	if ke.Rune >= 32 && ke.Rune != '`' && ke.Rune != '~' && !ctrl {
		r.con.AppendInputRune(ke.Rune)
		markDirty()
		return true
	}

	return false
}

// scalePicRGBA scales a palette-indexed QPic to (dstW x dstH) using nearest-neighbor sampling.
func scalePicRGBA(pic *qimage.QPic, palette []byte, dstW, dstH int) *image.RGBA {
	if pic == nil || dstW <= 0 || dstH <= 0 || pic.Width == 0 || pic.Height == 0 {
		return nil
	}
	if len(palette) < 768 {
		palette = draw.DefaultQuakePalette()
	}
	srcW := int(pic.Width)
	srcH := int(pic.Height)
	img := image.NewRGBA(image.Rect(0, 0, dstW, dstH))
	for y := 0; y < dstH; y++ {
		srcY := (y * srcH) / dstH
		for x := 0; x < dstW; x++ {
			srcX := (x * srcW) / dstW
			srcIdx := pic.Pixels[srcY*srcW+srcX]
			if srcIdx != 255 {
				off := int(srcIdx) * 3
				dstOff := (y*dstW + x) * 4
				img.Pix[dstOff+0] = palette[off]
				img.Pix[dstOff+1] = palette[off+1]
				img.Pix[dstOff+2] = palette[off+2]
				img.Pix[dstOff+3] = 255
			}
		}
	}
	return img
}
