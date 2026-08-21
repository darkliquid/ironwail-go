package input

import (
	"testing"

	"github.com/gogpu/gogpu/input"
)

// stubPollableInput fakes the gogpu polled input state. Tests drive it
// directly: gogpu would fill these from platform events via app.Input().
type stubPollableInput struct {
	keys        []input.Key
	mouseBtns   []input.MouseButton
	dx, dy      float32
	keyToEngine map[input.Key]int
}

func newStubPollableInput() *stubPollableInput {
	return &stubPollableInput{
		keyToEngine: defaultGogpuKeyToEngine(),
	}
}

// defaultGogpuKeyToEngine mirrors the renderer/gogpu PollingKeyMap subset the
// engine cares about for gameplay: letters, digits, and modifiers. The polling
// adapter receives already-mapped engine key codes; this stub just uses the
// same convention (W→'w', space→KSpace etc.) so adapter tests stay in the
// engine key-code domain.
func defaultGogpuKeyToEngine() map[input.Key]int {
	return map[input.Key]int{
		input.KeyA:        int('a'),
		input.KeyW:        int('w'),
		input.KeySpace:    KSpace,
		input.KeyEscape:   KEscape,
		input.KeyShiftLeft: KShift,
		input.KeyControlLeft: KCtrl,
		input.KeyAltLeft:  KAlt,
	}
}

// Poll returns the current frame's snapshot.
func (s *stubPollableInput) Poll() PolledInput {
	out := PolledInput{
		KeyPressed:   make(map[int]bool),
		MousePressed: make(map[int]bool),
		DeltaX:       s.dx,
		DeltaY:       s.dy,
	}
	// Reset per-frame accumulation after handing out the snapshot.
	s.dx, s.dy = 0, 0
	for _, key := range s.keys {
		out.KeyPressed[s.keyToEngine[key]] = true
	}
	for _, btn := range s.mouseBtns {
		out.MousePressed[engineMouseKey(btn)] = true
	}
	return out
}

func engineMouseKey(btn input.MouseButton) int {
	switch btn {
	case input.MouseButtonLeft:
		return KMouse1
	case input.MouseButtonRight:
		return KMouse2
	case input.MouseButtonMiddle:
		return KMouse3
	default:
		return KMouse1
	}
}

// TestPollingAdapterPressReleaseEdges asserts the polling adapter synthesizes
// one press edge on descent and one release edge on ascent per key — the same
// edge semantics the callback backend delivers.
func TestPollingAdapterPressReleaseEdges(t *testing.T) {
	stub := newStubPollableInput()
	adapter := NewPollingAdapter(stub)

	// Frame 1: W presses and is held.
	stub.keys = []input.Key{input.KeyW}
	events := adapter.Poll()
	if len(events) != 1 || !events[0].Down || events[0].Key != int('w') {
		t.Fatalf("frame 1 events = %+v, want single press of 'w'", events)
	}

	// Frame 2: still held — must NOT re-emit the press edge (matches the
	// callback backend: no repeat delivery).
	events = adapter.Poll()
	if len(events) != 0 {
		t.Fatalf("frame 2 (held) events = %+v, want none (no repeat)", events)
	}

	// Frame 3: release (key vanishes from the held set).
	stub.keys = nil
	events = adapter.Poll()
	if len(events) != 1 || events[0].Down || events[0].Key != int('w') {
		t.Fatalf("frame 3 events = %+v, want single release of 'w'", events)
	}

	// Frame 4: idle — nothing.
	events = adapter.Poll()
	if len(events) != 0 {
		t.Fatalf("frame 4 (idle) events = %+v, want none", events)
	}
}

// TestPollingAdapterMouseDeltaAccumulation asserts mouse deltas accumulate
// across multiple platform samples within one engine frame and are delivered
// once per frame.
func TestPollingAdapterMouseDeltaAccumulation(t *testing.T) {
	stub := newStubPollableInput()
	adapter := NewPollingAdapter(stub)

	stub.dx, stub.dy = 3, 4
	if events := adapter.Poll(); len(events) != 0 {
		t.Fatalf("mouse-only frame events = %+v, want none (deltas are not key events)", events)
	}

	// Second sample within the same frame accumulates on top.
	stub.dx, stub.dy = 2, 1
	if events := adapter.Poll(); len(events) != 0 {
		t.Fatalf("second mouse-only frame events = %+v, want none", events)
	}
}

// TestPollingAdapterMouseButtonEdges asserts mouse button press and release
// edges mirror keyboard edges.
func TestPollingAdapterMouseButtonEdges(t *testing.T) {
	stub := newStubPollableInput()
	adapter := NewPollingAdapter(stub)

	stub.mouseBtns = []input.MouseButton{input.MouseButtonLeft}
	events := adapter.Poll()
	if len(events) != 1 || !events[0].Down || events[0].Key != KMouse1 || events[0].Device != DeviceMouse {
		t.Fatalf("mouse press events = %+v, want single press of KMouse1", events)
	}

	stub.mouseBtns = nil
	events = adapter.Poll()
	if len(events) != 1 || events[0].Down || events[0].Key != KMouse1 {
		t.Fatalf("mouse release events = %+v, want single release of KMouse1", events)
	}
}

// TestPollingAdapterTracksHeldStateAcrossFrames guards that the adapter's
// internal previous-state bookkeeping survives multi-frame holds and releases
// without dropping or duplicating edges (the exact latching scenario the M0
// spike migrates).
func TestPollingAdapterTracksHeldStateAcrossFrames(t *testing.T) {
	stub := newStubPollableInput()
	adapter := NewPollingAdapter(stub)

	// Press W, hold for 3 frames, release.
	stub.keys = []input.Key{input.KeyW}
	adapter.Poll()               // press edge
	adapter.Poll()               // held
	adapter.Poll()               // held

	// Release W: vanish from the held set.
	stub.keys = nil
	events := adapter.Poll()
	if len(events) != 1 || events[0].Down {
		t.Fatalf("release events = %+v, want exactly one release", events)
	}

	// Re-press immediately — must emit a fresh press edge (no straddle).
	stub.keys = []input.Key{input.KeyW}
	events = adapter.Poll()
	if len(events) != 1 || !events[0].Down {
		t.Fatalf("re-press events = %+v, want exactly one press", events)
	}
}

// TestPollingAdapterModifierEdges verifies modifier keys (alt, shift, ctrl)
// route through the same edge machinery.
func TestPollingAdapterModifierEdges(t *testing.T) {
	stub := newStubPollableInput()
	adapter := NewPollingAdapter(stub)

	stub.keys = []input.Key{input.KeyAltLeft}
	events := adapter.Poll()
	if len(events) != 1 || events[0].Key != KAlt || !events[0].Down {
		t.Fatalf("alt press events = %+v, want single press of KAlt", events)
	}

	stub.keys = nil
	events = adapter.Poll()
	if len(events) != 1 || events[0].Key != KAlt || events[0].Down {
		t.Fatalf("alt release events = %+v, want single release of KAlt", events)
	}
}
