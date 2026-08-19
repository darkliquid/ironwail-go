package quakui

import (
	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/gogpu/ui/event"
)

// MapEngineKey translates an engine key code (input.Key*) into the gogpu/ui
// event.Key domain. It returns KeyUnknown for keys with no ui counterpart.
//
// This is the isolation-boundary side of the engine↔ui key translation. The
// ui never sees the engine's numeric key codes; the KeyForwarder (M1.5 /
// ADR-0007) routes translated ui keys into the widget tree.
func MapEngineKey(k int) event.Key {
	switch k {
	case input.KBackspace:
		return event.KeyBackspace
	case input.KTab:
		return event.KeyTab
	case input.KEnter:
		return event.KeyEnter
	case input.KEscape:
		return event.KeyEscape
	case input.KSpace:
		return event.KeySpace
	case input.KUpArrow:
		return event.KeyUp
	case input.KDownArrow:
		return event.KeyDown
	case input.KLeftArrow:
		return event.KeyLeft
	case input.KRightArrow:
		return event.KeyRight
	case input.KDel:
		return event.KeyDelete
	case input.KIns:
		return event.KeyInsert
	case input.KHome:
		return event.KeyHome
	case input.KEnd:
		return event.KeyEnd
	case input.KPgUp:
		return event.KeyPageUp
	case input.KPgDn:
		return event.KeyPageDown
	case input.KCapsLock:
		return event.KeyCapsLock
	case input.KScrollLock:
		return event.KeyScrollLock
	case input.KPrintScreen:
		return event.KeyPrintScreen
	case input.KF1, input.KF2, input.KF3, input.KF4, input.KF5, input.KF6,
		input.KF7, input.KF8, input.KF9, input.KF10, input.KF11, input.KF12:
		return event.Key(int(event.KeyF1) + (k - input.KF1))
	case input.KShift:
		return event.KeyLeftShift
	case input.KCtrl:
		return event.KeyLeftCtrl
	case input.KAlt:
		return event.KeyLeftAlt
	case input.KCommand:
		return event.KeyLeftSuper
	default:
		// Printable ASCII (letters, digits, punctuation) has no distinct
		// event.Key in the ui domain for character input; character events
		// reach the ui via Rune. Navigation/modifier keys above are mapped.
		// Mouse buttons (KMouseBegin+) and unhandled special keys translate
		// to KeyUnknown; the shim drops them rather than misroute.
		return event.KeyUnknown
	}
}

// Modifiers translates the engine modifier state into gogpu/ui event.Modifiers.
func Modifiers(m input.ModifierState) event.Modifiers {
	var mods event.Modifiers
	if m.Shift {
		mods |= event.ModShift
	}
	if m.Ctrl {
		mods |= event.ModCtrl
	}
	if m.Alt {
		mods |= event.ModAlt
	}
	return mods
}

// HandleEvents is the minimal ui surface the KeyForwarder needs: a single
// push-events-into-the-tree method. quakui uses *app.App in production and a
// recording stub in tests, keeping the shim dependency narrow (ADR-0009).
type HandleEvents interface {
	HandleEvent(e event.Event)
}

// KeyForwarder is the M1.5 input shim (ADR-0007). It is the sole conduit that
// translates already-routed engine key/char events into gogpu/ui events and
// pushes them into the ui app's widget tree via app.HandleEvent. The shim does
// NOT re-implement KeyDest routing — the engine decides whether a given key
// reaches the ui (menu/console capture) or stays in the game (game/HUD-only
// fallthrough). This mirrors how the ui's own keyboard bridge translates
// platform events (app/event_bridge.go), without holding a second EventSource
// registration (research 0007 §1 single-slot conflict).
type KeyForwarder struct {
	ui HandleEvents
}

// NewKeyForwarder builds a KeyForwarder that pushes translated events into the
// given handle-events sink (typically the uiApp built in Run).
func NewKeyForwarder(ui HandleEvents) *KeyForwarder {
	return &KeyForwarder{ui: ui}
}

// ForwardKey routes an already-KeyDest-decided engine key event into the ui.
// It translates the engine key code and stamps the current engine modifier
// state. Character input is delivered separately via ForwardChar/ForwardText
// (matching how the ui's keyboard bridge delivers text via OnTextInput).
func (f *KeyForwarder) ForwardKey(ev input.KeyEvent, mods input.ModifierState) {
	if f == nil || f.ui == nil {
		return
	}
	ktype := event.KeyRelease
	if ev.Down {
		ktype = event.KeyPress
	}
	uiMods := Modifiers(mods)
	f.ui.HandleEvent(event.NewKeyEvent(ktype, MapEngineKey(ev.Key), 0, uiMods))
}

// ForwardChar routes an engine character event (rune) into the ui as a text
// input KeyPress. Used for character composition (letters/digits/punctuation)
// which the engine delivers separately from physical keys.
func (f *KeyForwarder) ForwardChar(r rune, mods input.ModifierState) {
	if f == nil || f.ui == nil || r == 0 {
		return
	}
	f.ui.HandleEvent(event.NewKeyEvent(event.KeyPress, event.KeyUnknown, r, Modifiers(mods)))
}

// ForwardText routes a run of characters (e.g. an OS-composed IME string) as
// individual ui KeyPress events. Mirrors the ui bridge's OnTextInput loop.
func (f *KeyForwarder) ForwardText(text string, mods input.ModifierState) {
	if f == nil || f.ui == nil {
		return
	}
	uiMods := Modifiers(mods)
	for _, r := range text {
		f.ui.HandleEvent(event.NewKeyEvent(event.KeyPress, event.KeyUnknown, r, uiMods))
	}
}
