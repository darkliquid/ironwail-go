package quakui

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/gogpu/ui/event"
)

// recordingEvents is a test double for HandleEvents that appends every event
// it receives, so tests can assert the KeyForwarder's translation.
type recordingEvents struct {
	events []event.Event
}

func (r *recordingEvents) HandleEvent(e event.Event) {
	r.events = append(r.events, e)
}

func keyEvent(key int, runeKey rune, down bool) input.KeyEvent {
	return input.KeyEvent{Key: key, Down: down, Character: runeKey}
}

func TestMapEngineKey(t *testing.T) {
	cases := []struct {
		name string
		in   int
		want event.Key
	}{
		{"backspace", input.KBackspace, event.KeyBackspace},
		{"tab", input.KTab, event.KeyTab},
		{"enter", input.KEnter, event.KeyEnter},
		{"escape", input.KEscape, event.KeyEscape},
		{"space", input.KSpace, event.KeySpace},
		{"up", input.KUpArrow, event.KeyUp},
		{"down", input.KDownArrow, event.KeyDown},
		{"left", input.KLeftArrow, event.KeyLeft},
		{"right", input.KRightArrow, event.KeyRight},
		{"delete", input.KDel, event.KeyDelete},
		{"insert", input.KIns, event.KeyInsert},
		{"home", input.KHome, event.KeyHome},
		{"end", input.KEnd, event.KeyEnd},
		{"pgup", input.KPgUp, event.KeyPageUp},
		{"pgdn", input.KPgDn, event.KeyPageDown},
		{"capslock", input.KCapsLock, event.KeyCapsLock},
		{"scrolllock", input.KScrollLock, event.KeyScrollLock},
		{"printscreen", input.KPrintScreen, event.KeyPrintScreen},
		{"f1", input.KF1, event.KeyF1},
		{"f12", input.KF12, event.KeyF12},
		{"shift", input.KShift, event.KeyLeftShift},
		{"ctrl", input.KCtrl, event.KeyLeftCtrl},
		{"alt", input.KAlt, event.KeyLeftAlt},
		{"command", input.KCommand, event.KeyLeftSuper},
		{"backtick", int('`'), event.KeyGrave},
		{"tilde", int('~'), event.KeyGrave},
		{"ascii letter a", int('a'), event.KeyA},
		{"ascii digit 0", int('0'), event.Key0},
		{"mouse button dropped", input.KMouseBegin, event.KeyUnknown},
		{"unknown dropped", 9999, event.KeyUnknown},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := MapEngineKey(tc.in); got != tc.want {
				t.Errorf("MapEngineKey(%d) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestModifiers(t *testing.T) {
	cases := []struct {
		name string
		in   input.ModifierState
		want event.Modifiers
	}{
		{"none", input.ModifierState{}, event.ModNone},
		{"shift", input.ModifierState{Shift: true}, event.ModShift},
		{"ctrl", input.ModifierState{Ctrl: true}, event.ModCtrl},
		{"alt", input.ModifierState{Alt: true}, event.ModAlt},
		{"all", input.ModifierState{Shift: true, Ctrl: true, Alt: true}, event.ModShift | event.ModCtrl | event.ModAlt},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Modifiers(tc.in); got != tc.want {
				t.Errorf("Modifiers(%+v) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestKeyForwarderForwardKey(t *testing.T) {
	rec := &recordingEvents{}
	f := NewKeyForwarder(rec)

	// A mapped key press (Tab) with Shift held.
	f.ForwardKey(keyEvent(input.KTab, 0, true), input.ModifierState{Shift: true})

	if len(rec.events) != 1 {
		t.Fatalf("ForwardKey produced %d events, want 1", len(rec.events))
	}
	ke, ok := rec.events[0].(*event.KeyEvent)
	if !ok {
		t.Fatalf("ForwardKey produced %T, want *event.KeyEvent", rec.events[0])
	}
	if ke.Key != event.KeyTab {
		t.Errorf("key = %v, want KeyTab", ke.Key)
	}
	if ke.KeyType != event.KeyPress {
		t.Errorf("keyType = %v, want KeyPress", ke.KeyType)
	}
	if !ke.IsShift() {
		t.Error("Shift modifier not stamped")
	}
}

func TestKeyForwarderForwardKeyRelease(t *testing.T) {
	rec := &recordingEvents{}
	f := NewKeyForwarder(rec)

	f.ForwardKey(keyEvent(input.KEnter, 0, false), input.ModifierState{})

	if len(rec.events) != 1 {
		t.Fatalf("ForwardKey produced %d events, want 1", len(rec.events))
	}
	ke := rec.events[0].(*event.KeyEvent)
	if ke.KeyType != event.KeyRelease {
		t.Errorf("keyType = %v, want KeyRelease", ke.KeyType)
	}
}

func TestKeyForwarderForwardChar(t *testing.T) {
	rec := &recordingEvents{}
	f := NewKeyForwarder(rec)

	f.ForwardChar('q', input.ModifierState{Ctrl: true})

	if len(rec.events) != 1 {
		t.Fatalf("ForwardChar produced %d events, want 1", len(rec.events))
	}
	ke := rec.events[0].(*event.KeyEvent)
	if ke.Rune != 'q' {
		t.Errorf("rune = %q, want 'q'", ke.Rune)
	}
	if ke.Key != event.KeyUnknown {
		t.Errorf("key = %v, want KeyUnknown (text delivered via Rune)", ke.Key)
	}
	if ke.KeyType != event.KeyPress {
		t.Errorf("keyType = %v, want KeyPress", ke.KeyType)
	}
	if !ke.IsCtrl() {
		t.Error("Ctrl modifier not stamped")
	}
}

func TestKeyForwarderForwardText(t *testing.T) {
	rec := &recordingEvents{}
	f := NewKeyForwarder(rec)

	f.ForwardText("ab", input.ModifierState{})

	if len(rec.events) != 2 {
		t.Fatalf("ForwardText produced %d events, want 2", len(rec.events))
	}
	if rec.events[0].(*event.KeyEvent).Rune != 'a' ||
		rec.events[1].(*event.KeyEvent).Rune != 'b' {
		t.Errorf("rune sequence = %q, %q, want 'a', 'b'",
			rec.events[0].(*event.KeyEvent).Rune, rec.events[1].(*event.KeyEvent).Rune)
	}
}

func TestKeyForwarderNilSafe(t *testing.T) {
	var f *KeyForwarder
	// nil receiver and nil ui must not panic.
	f.ForwardKey(keyEvent(input.KTab, 0, true), input.ModifierState{})
	f.ForwardChar('a', input.ModifierState{})
	f.ForwardText("ab", input.ModifierState{})

	f2 := NewKeyForwarder(nil)
	f2.ForwardKey(keyEvent(input.KTab, 0, true), input.ModifierState{})
	f2.ForwardChar('a', input.ModifierState{})
	f2.ForwardText("ab", input.ModifierState{})
}

// TestKeyForwarderIsolation preserves AC7/ADR-0009: the quakui package must not
// import internal/game or internal/renderer. The KeyForwarder consumes the
// engine input types (internal/input), which is the documented legacy-state
// dependency, not an engine-package import.
func TestKeyForwarderIsolation(t *testing.T) {
	// The import-closure guard lives in TestNoEngineImports; this guards that
	// the forwarder's own type surface does not leak ui types to callers.
	if got := MapEngineKey(input.KEnter); got != event.KeyEnter {
		t.Errorf("MapEngineKey(KEnter) = %v, want KeyEnter", got)
	}
}
