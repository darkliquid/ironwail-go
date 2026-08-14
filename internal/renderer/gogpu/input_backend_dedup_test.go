package gogpu

import (
	"testing"

	iinput "github.com/darkliquid/ironwail-go/internal/input"
	gg "github.com/gogpu/gogpu"
	ginput "github.com/gogpu/gogpu/input"
)

// TestPollEventsDoesNotDoubleDeliverKeyEdges guards the regression where one
// physical key press was delivered twice to the engine input System: once via
// the EventSource callback path and once via the raw pressed-state polling
// path in PollEvents (the gogpu App feeds both from a single platform event).
// Double delivery advanced menu pages twice per click and corrupted button
// state in-game.
func TestPollEventsDoesNotDoubleDeliverKeyEdges(t *testing.T) {
	app := gg.NewApp(gg.Config{})
	sys := iinput.NewSystem(nil)
	backend := &InputBackend{app: app, sys: sys}

	var events []iinput.KeyEvent
	sys.OnKey = func(ev iinput.KeyEvent) { events = append(events, ev) }

	if err := backend.Init(); err != nil {
		t.Fatalf("backend Init: %v", err)
	}

	// Simulate the callback path already delivered events (the gogpu App
	// dispatches to EventSource callbacks AND to the raw Input() state from
	// the same platform event).
	backend.markCallbackSeen()

	// Seed the raw pressed state as if the same key is physically down.
	backend.app.Input().Keyboard().SetKey(ginput.KeyW, true)
	backend.PollEvents()

	if len(events) != 0 {
		t.Fatalf("callback-active PollEvents delivered %d raw edge events, want 0 (no double delivery)", len(events))
	}
}
// TestMousePressMarksCallbackActive guards the menu regression where a mouse
// click was the first input: the mouse EventSource callbacks did NOT mark the
// callback path active, so the raw polling path delivered the same click a
// second time (once per platform event), advancing the menu two pages per
// physical click (e.g. "Single Player" instantly starting a new game).
func TestMousePressMarksCallbackActive(t *testing.T) {
	app := gg.NewApp(gg.Config{})
	sys := iinput.NewSystem(nil)
	backend := &InputBackend{app: app, sys: sys}

	var events []iinput.KeyEvent
	sys.OnKey = func(ev iinput.KeyEvent) { events = append(events, ev) }

	if err := backend.Init(); err != nil {
		t.Fatalf("backend Init: %v", err)
	}

	// Simulate a callback-mode mouse press (what the real EventSource does).
	backend.markCallbackSeen()

	// Polling must NOT re-deliver the mouse-button edge.
	backend.app.Input().Mouse().SetButton(ginput.MouseButtonLeft, true)
	backend.PollEvents()

	if len(events) != 0 {
		t.Fatalf("callback-active PollEvents delivered %d raw mouse events, want 0 (no double click delivery)", len(events))
	}
}

// TestCallbackPathIsAuthoritativeForGameplay verifies that once the gogpu
// EventSource callbacks are active, the raw-polling path is fully suppressed
// so a single physical press cannot be double-delivered as two callbacks (this
// is what corrupted in-game button latches: a duplicate `+forward` made the
// KButton Down[] bookkeeping drift from the physical key state).
func TestCallbackPathIsAuthoritativeForGameplay(t *testing.T) {
	app := gg.NewApp(gg.Config{})
	sys := iinput.NewSystem(nil)
	backend := &InputBackend{app: app, sys: sys}

	var events []iinput.KeyEvent
	sys.OnKey = func(ev iinput.KeyEvent) { events = append(events, ev) }
	sys.SetKeyDest(iinput.KeyGame)

	if err := backend.Init(); err != nil {
		t.Fatalf("backend Init: %v", err)
	}
	backend.markCallbackSeen()

	// A single physical press: the app sets the raw pressed state; the
	// callback path would have delivered the Down. PollEvents must NOT add a
	// second Down from the raw state.
	backend.app.Input().Keyboard().SetKey(ginput.KeyW, true)
	backend.PollEvents()

	// Then release.
	backend.app.Input().Keyboard().SetKey(ginput.KeyW, false)
	backend.PollEvents()

	if len(events) != 0 {
		t.Fatalf("callback-active backend delivered %d events from raw polling, want 0 (double-delivery corrupts button bookkeeping)", len(events))
	}
}

// TestFirstInputMouseMoveArmsCallbackPath guards the in-game first-input case:
// the FIRST event after grabbing the pointer is usually a mouse move (not a
// key press), which only marks "seen". The polling path must still be fully
// suppressed from that point, otherwise the first key press in-game is
// delivered twice (once via callback, once via raw polling armed by the
// mouse-move) and the KButton bookkeeping drifts — the "movement only works
// once until escape" symptom.
func TestFirstInputMouseMoveArmsCallbackPath(t *testing.T) {
	app := gg.NewApp(gg.Config{})
	sys := iinput.NewSystem(nil)
	backend := &InputBackend{app: app, sys: sys}

	var events []iinput.KeyEvent
	sys.OnKey = func(ev iinput.KeyEvent) { events = append(events, ev) }
	sys.SetKeyDest(iinput.KeyGame)

	if err := backend.Init(); err != nil {
		t.Fatalf("backend Init: %v", err)
	}

	// First input: a mouse move (marks seen only, not input-OK).
	backend.markCallbackSeen()
	if !backend.hasCallbackSeen() {
		t.Fatal("markCallbackSeen must arm the seen flag")
	}

	// Raw polling now sees a held key (the app set it when the physical key
	// went down in the same frame as the mouse move). It must NOT be delivered.
	backend.app.Input().Keyboard().SetKey(ginput.KeyW, true)
	backend.PollEvents()
	backend.app.Input().Keyboard().SetKey(ginput.KeyW, false)
	backend.PollEvents()

	if len(events) != 0 {
		t.Fatalf("after first mouse-move callback, raw polling delivered %d events, want 0 (double delivery of first key press)", len(events))
	}
}
func TestMouseMoveNotDoubleCounted(t *testing.T) {
	app := gg.NewApp(gg.Config{})
	sys := iinput.NewSystem(nil)
	backend := &InputBackend{app: app, sys: sys}

	if err := backend.Init(); err != nil {
		t.Fatalf("backend Init: %v", err)
	}
	backend.markCallbackSeen()

	// Simulate one physical move: the app fed the OnPointer callback (which
	// accumulated 50/25 into the backend) AND its raw Input() state (position
	// moved 100,100 -> 150,125, so Delta() = 50/25 after Update()).
	backend.accumMouseDX += 50
	backend.accumMouseDY += 25
	backend.app.Input().Mouse().SetPosition(100, 100)
	backend.app.Input().Update()
	backend.app.Input().Mouse().SetPosition(150, 125)

	// PollEvents (callback-active path) must NOT add the raw delta again.
	backend.PollEvents()

	dx, dy := backend.MouseDelta()
	if dx != 50 || dy != 25 {
		t.Fatalf("mouse delta after one move = (%d, %d), want (50, 25) — move double-counted", dx, dy)
	}
}
