// package input internal/input/types.go — see types.go for the package overview.
// polling.go implements the decoupled-input polling adapter (ADR-0012).
package input

import (
	"log/slog"
)

// PollableInput is the gogpu-side polled input state the engine reads each
// frame. It exposes Ebiten-style state: press/release edge tracking
// (JustPressed/JustReleased), held state (Pressed), and accumulated mouse
// movement (Delta). gogpu feeds this state from every platform event via
// app.Input() regardless of EventSource ownership (§4.2, ADR-0012).
type PollableInput interface {
	// Poll returns the frame's accumulated input state.
	Poll() PolledInput
}

// PolledInput is a frame snapshot of the polled input source: keyboard
// held state, mouse held state, and accumulated mouse delta.
type PolledInput struct {
	// KeyPressed is the set of engine key codes currently held.
	KeyPressed map[int]bool
	// MousePressed is the set of engine mouse-button key codes currently held.
	MousePressed map[int]bool
	// Delta is the accumulated relative mouse movement in pixels since the
	// previous frame (positive = right/down).
	DeltaX, DeltaY float32
}

// PollingAdapter translates a polled input source into the engine's key-event
// stream. It is the ADR-0012 gameplay path: the engine polls app.Input() each
// frame instead of receiving platform callbacks, eliminating the
// callback/polling double-delivery class of bugs from the v3 input backend.
//
// Edge semantics are identical to the callback backend: a "press" edge fires
// exactly once on the transition to held, a "release" edge exactly once on
// the transition to released, and only while the engine is in KeyGame (or
// message) destination. The adapter only issues *events*; the engine's
// routing (System.HandleKeyEvent → KeyDest router) remains the policy point.
type PollingAdapter struct {
	// display is the polled input source.
	display PollableInput

	// prev holds the previous frame's key/mouse held state so edges are
	// derived by comparison (identical to the callback backend's edge logic).
	prevKeys   map[int]bool
	prevMouse  map[int]bool
}

// NewPollingAdapter builds an adapter that reads from display each frame.
func NewPollingAdapter(display PollableInput) *PollingAdapter {
	return &PollingAdapter{
		display:   display,
		prevKeys:  make(map[int]bool),
		prevMouse: make(map[int]bool),
	}
}

// Poll reads the next frame of polled input and returns the set of key events
// to feed into the engine (System.HandleKeyEvent). It synthesizes press and
// release edges from held-state transitions, matching the callback backend's
// edge semantics: a held key yields one press edge on descent and one release
// edge on ascent, per key code and mouse button. Mouse deltas are accumulated
// per frame and exposed via the engine's mouse-look path.
func (a *PollingAdapter) Poll() []KeyEvent {
	if a == nil || a.display == nil {
		return nil
	}

	in := a.display.Poll()

	var events []KeyEvent

	// Press edge: held now and not held last frame. The source's level state
	// (KeyPressed) is authoritative; a key absent from it is not held.
	for key, isHeld := range in.KeyPressed {
		if isHeld && !a.prevKeys[key] {
			events = append(events, KeyEvent{Key: key, Down: true, Device: DeviceKeyboard})
		}
	}
	// Release edge: was held last frame and not held now — covers both keys
	// the source explicitly released and keys that merely vanished from the
	// held set (identical to the callback backend's ascent semantics).
	for key, wasHeld := range a.prevKeys {
		if wasHeld && !in.KeyPressed[key] {
			events = append(events, KeyEvent{Key: key, Down: false, Device: DeviceKeyboard})
		}
	}

	// Mouse buttons: same edge logic against the previous frame.
	for btn, isHeld := range in.MousePressed {
		if isHeld && !a.prevMouse[btn] {
			events = append(events, KeyEvent{Key: btn, Down: true, Device: DeviceMouse})
		}
	}
	for btn, wasHeld := range a.prevMouse {
		if wasHeld && !in.MousePressed[btn] {
			events = append(events, KeyEvent{Key: btn, Down: false, Device: DeviceMouse})
		}
	}

	// Persist current held state for next frame's edge derivation. Keys that
	// are no longer held are dropped so a later re-press is a fresh edge.
	for key := range a.prevKeys {
		if !in.KeyPressed[key] {
			delete(a.prevKeys, key)
		}
	}
	if len(in.KeyPressed) > 0 {
		a.prevKeys = in.KeyPressed
	}
	for btn := range a.prevMouse {
		if !in.MousePressed[btn] {
			delete(a.prevMouse, btn)
		}
	}
	if len(in.MousePressed) > 0 {
		a.prevMouse = in.MousePressed
	}

	if len(events) > 0 {
		slog.Debug("input polling adapter",
			"events", len(events),
			"keys_held", len(in.KeyPressed),
			"mouse_btn_held", len(in.MousePressed),
			"mouse_dx", in.DeltaX,
			"mouse_dy", in.DeltaY,
		)
	}
	return events
}
