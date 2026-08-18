package quakeui

import (
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/gogpu/gpucontext"
)

// KeyDestReader reports the engine's current input routing destination. The
// gateway routes events into the ui tree only when a ui surface is active
// (KeyConsole/KeyMenu), and suppresses them during gameplay (KeyGame) so the
// engine's latching/mouse-look behavior is untouched (ADR-0003).
type KeyDestReader interface {
	KeyDest() input.KeyDest
}

// Gateway is a gpucontext.EventSource shim that feeds the gogpu/ui app from
// the engine's authoritative input pipeline (ADR-0003). It is the only
// registration on the ui app's EventSource; the engine keeps its own backend
// registration for the legacy path (ui_backend=0). On path 1 the game forwards
// engine input events into the gateway via the Feed* methods, and the gateway
// routes them to the ui tree per KeyDest, suppressing gameplay-only input.
//
// The gateway also implements PointerEventSource and ScrollEventSource so the
// ui app's event bridge uses the unified pointer/scroll pipeline.
type Gateway struct {
	kd KeyDestReader

	onKeyPress   func(key gpucontext.Key, mods gpucontext.Modifiers)
	onKeyRelease func(key gpucontext.Key, mods gpucontext.Modifiers)
	onTextInput  func(text string)
	onMouseMove  func(x, y float64)
	onMousePress func(button gpucontext.MouseButton, x, y float64)
	onMouseRel   func(button gpucontext.MouseButton, x, y float64)
	onScroll     func(dx, dy float64)
	onScrollEv   func(ev gpucontext.ScrollEvent)
	onResize     func(width, height int)
	onFocus      func(focused bool)
	onPointer    func(ev gpucontext.PointerEvent)
}

// NewGateway constructs the input gateway. The KeyDestReader may be nil (the
// gateway then treats every destination as ui-active, used by headless tests
// that feed events directly).
func NewGateway(kd KeyDestReader) *Gateway {
	return &Gateway{kd: kd}
}

// uiActive reports whether the current KeyDest routes events to a ui surface.
// KeyConsole and KeyMenu are ui surfaces; KeyGame and KeyMessage are not.
// A nil reader (headless tests) is always ui-active.
func (g *Gateway) uiActive() bool {
	if g == nil || g.kd == nil {
		return true
	}
	switch g.kd.KeyDest() {
	case input.KeyConsole, input.KeyMenu:
		return true
	default:
		return false
	}
}

// textActive reports whether character input should reach the ui tree.
// Console and menu both accept text entry.
func (g *Gateway) textActive() bool {
	return g.uiActive()
}

// FeedKeyPress delivers an engine key press to the ui tree when a ui surface
// is active.
func (g *Gateway) FeedKeyPress(key gpucontext.Key) {
	if !g.uiActive() || g.onKeyPress == nil {
		return
	}
	g.onKeyPress(key, g.currentModifiers())
}

// FeedEngineKeyEvent translates an engine input.KeyEvent and delivers it to
// the ui tree when a ui surface is active. This is the raw-sink entry point
// (ADR-0003): the engine's input.System.OnRawKey calls it for every key event
// before routing, and the gateway routes by KeyDest.
func (g *Gateway) FeedEngineKeyEvent(ev input.KeyEvent) {
	if !g.uiActive() {
		return
	}
	key := EngineKeyToGPU(ev.Key)
	if key == gpucontext.KeyUnknown {
		return
	}
	if ev.Down {
		g.FeedKeyPress(key)
	} else {
		g.FeedKeyRelease(key)
	}
}

// FeedEngineCharEvent delivers engine character input to the ui tree when a
// text-capable surface is active. This is the raw-sink entry point for text.
func (g *Gateway) FeedEngineCharEvent(ch rune) {
	if !g.textActive() {
		return
	}
	g.FeedTextInput(string(ch))
}

// FeedKeyRelease delivers an engine key release to the ui tree when a ui
// surface is active.
func (g *Gateway) FeedKeyRelease(key gpucontext.Key) {
	if !g.uiActive() || g.onKeyRelease == nil {
		return
	}
	g.onKeyRelease(key, g.currentModifiers())
}

// FeedTextInput delivers engine character input to the ui tree when a
// text-capable surface is active.
func (g *Gateway) FeedTextInput(text string) {
	if !g.textActive() || g.onTextInput == nil {
		return
	}
	g.onTextInput(text)
}

// FeedMouseMove delivers an engine mouse move to the ui tree when a ui
// surface is active.
func (g *Gateway) FeedMouseMove(x, y float64) {
	if !g.uiActive() || g.onMouseMove == nil {
		return
	}
	g.onMouseMove(x, y)
}

// FeedMousePress delivers an engine mouse press to the ui tree when a ui
// surface is active.
func (g *Gateway) FeedMousePress(button gpucontext.MouseButton, x, y float64) {
	if !g.uiActive() || g.onMousePress == nil {
		return
	}
	g.onMousePress(button, x, y)
}

// FeedMouseRelease delivers an engine mouse release to the ui tree when a ui
// surface is active.
func (g *Gateway) FeedMouseRelease(button gpucontext.MouseButton, x, y float64) {
	if !g.uiActive() || g.onMouseRel == nil {
		return
	}
	g.onMouseRel(button, x, y)
}

// FeedScroll delivers an engine scroll wheel event to the ui tree when a ui
// surface is active.
func (g *Gateway) FeedScroll(dx, dy float64) {
	if !g.uiActive() || g.onScroll == nil {
		return
	}
	g.onScroll(dx, dy)
}

// FeedScrollEvent delivers a detailed scroll event to the ui tree when a ui
// surface is active.
func (g *Gateway) FeedScrollEvent(ev gpucontext.ScrollEvent) {
	if !g.uiActive() || g.onScrollEv == nil {
		return
	}
	g.onScrollEv(ev)
}

// FeedPointer delivers a unified pointer event to the ui tree when a ui
// surface is active.
func (g *Gateway) FeedPointer(ev gpucontext.PointerEvent) {
	if !g.uiActive() || g.onPointer == nil {
		return
	}
	g.onPointer(ev)
}

// currentModifiers returns the engine modifier state as gpucontext modifiers.
func (g *Gateway) currentModifiers() gpucontext.Modifiers {
	var mods gpucontext.Modifiers
	if g.kd == nil {
		return mods
	}
	if sys, ok := g.kd.(*input.System); ok {
		ms := sys.ModifierState()
		if ms.Shift {
			mods |= gpucontext.ModShift
		}
		if ms.Ctrl {
			mods |= gpucontext.ModControl
		}
		if ms.Alt {
			mods |= gpucontext.ModAlt
		}
	}
	return mods
}

// --- gpucontext.EventSource ---

// OnKeyPress registers the key press callback.
func (g *Gateway) OnKeyPress(fn func(key gpucontext.Key, mods gpucontext.Modifiers)) {
	g.onKeyPress = fn
}

// OnKeyRelease registers the key release callback.
func (g *Gateway) OnKeyRelease(fn func(key gpucontext.Key, mods gpucontext.Modifiers)) {
	g.onKeyRelease = fn
}

// OnTextInput registers the text input callback.
func (g *Gateway) OnTextInput(fn func(text string)) {
	g.onTextInput = fn
}

// OnMouseMove registers the mouse move callback.
func (g *Gateway) OnMouseMove(fn func(x, y float64)) {
	g.onMouseMove = fn
}

// OnMousePress registers the mouse press callback.
func (g *Gateway) OnMousePress(fn func(button gpucontext.MouseButton, x, y float64)) {
	g.onMousePress = fn
}

// OnMouseRelease registers the mouse release callback.
func (g *Gateway) OnMouseRelease(fn func(button gpucontext.MouseButton, x, y float64)) {
	g.onMouseRel = fn
}

// OnScroll registers the scroll callback.
func (g *Gateway) OnScroll(fn func(dx, dy float64)) {
	g.onScroll = fn
}

// OnResize registers the resize callback.
func (g *Gateway) OnResize(fn func(width, height int)) {
	g.onResize = fn
}

// OnFocus registers the focus callback.
func (g *Gateway) OnFocus(fn func(focused bool)) {
	g.onFocus = fn
}

// OnIMECompositionStart is a no-op (the engine has no IME composition bridge).
func (g *Gateway) OnIMECompositionStart(fn func()) {}

// OnIMECompositionUpdate is a no-op (the engine has no IME composition bridge).
func (g *Gateway) OnIMECompositionUpdate(fn func(state gpucontext.IMEState)) {}

// OnIMECompositionEnd is a no-op (the engine has no IME composition bridge).
func (g *Gateway) OnIMECompositionEnd(fn func(committed string)) {}

// --- gpucontext.PointerEventSource ---

// OnPointer registers the unified pointer callback.
func (g *Gateway) OnPointer(fn func(ev gpucontext.PointerEvent)) {
	g.onPointer = fn
}

// --- gpucontext.ScrollEventSource ---

// OnScrollEvent registers the detailed scroll callback.
func (g *Gateway) OnScrollEvent(fn func(ev gpucontext.ScrollEvent)) {
	g.onScrollEv = fn
}

var (
	_ gpucontext.EventSource        = (*Gateway)(nil)
	_ gpucontext.PointerEventSource = (*Gateway)(nil)
	_ gpucontext.ScrollEventSource   = (*Gateway)(nil)
)

// EngineKeyToGPU maps an engine key code to a gpucontext.Key. Printable ASCII
// (a-z, 0-9, punctuation) map to their gpucontext letter/digit keys; the
// Quake special keys map to gpucontext navigation keys. Unmapped codes return
// KeyUnknown.
func EngineKeyToGPU(engineKey int) gpucontext.Key {
	switch {
	case 'a' <= engineKey && engineKey <= 'z':
		return gpucontext.Key(engineKey - 'a' + 1) // KeyA..KeyZ
	case '0' <= engineKey && engineKey <= '9':
		return gpucontext.Key(engineKey - '0' + 33) // Key0..Key9
	}
	switch engineKey {
	case input.KEnter:
		return gpucontext.KeyEnter
	case input.KEscape:
		return gpucontext.KeyEscape
	case input.KTab:
		return gpucontext.KeyTab
	case input.KSpace:
		return gpucontext.KeySpace
	case input.KBackspace:
		return gpucontext.KeyBackspace
	case input.KUpArrow:
		return gpucontext.KeyUp
	case input.KDownArrow:
		return gpucontext.KeyDown
	case input.KLeftArrow:
		return gpucontext.KeyLeft
	case input.KRightArrow:
		return gpucontext.KeyRight
	case input.KHome:
		return gpucontext.KeyHome
	case input.KEnd:
		return gpucontext.KeyEnd
	case input.KPgUp:
		return gpucontext.KeyPageUp
	case input.KPgDn:
		return gpucontext.KeyPageDown
	case input.KIns:
		return gpucontext.KeyInsert
	case input.KDel:
		return gpucontext.KeyDelete
	case input.KShift:
		return gpucontext.KeyLeftShift
	case input.KCtrl:
		return gpucontext.KeyLeftControl
	case input.KAlt:
		return gpucontext.KeyLeftAlt
	case input.KF1:
		return gpucontext.KeyF1
	case input.KF2:
		return gpucontext.KeyF2
	case input.KF3:
		return gpucontext.KeyF3
	case input.KF4:
		return gpucontext.KeyF4
	case input.KF5:
		return gpucontext.KeyF5
	case input.KF6:
		return gpucontext.KeyF6
	case input.KF7:
		return gpucontext.KeyF7
	case input.KF8:
		return gpucontext.KeyF8
	case input.KF9:
		return gpucontext.KeyF9
	case input.KF10:
		return gpucontext.KeyF10
	case input.KF11:
		return gpucontext.KeyF11
	case input.KF12:
		return gpucontext.KeyF12
	}
	return gpucontext.KeyUnknown
}
