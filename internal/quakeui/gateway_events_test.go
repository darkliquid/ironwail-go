package quakeui

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/input"
	"github.com/gogpu/gpucontext"
)

// fakeKeyDest is a mutable KeyDest reader for routing tests.
type fakeKeyDest struct {
	dest input.KeyDest
}

func (f *fakeKeyDest) KeyDest() input.KeyDest { return f.dest }

// TestGatewayImplementsEventSource asserts the gateway satisfies the full
// gpucontext.EventSource contract (ADR-0003) so app.WithEventSource accepts it.
func TestGatewayImplementsEventSource(t *testing.T) {
	g := NewGateway(nil)
	var _ gpucontext.EventSource = g
	var _ gpucontext.PointerEventSource = g
	var _ gpucontext.ScrollEventSource = g
}

// TestGatewayKeyRoutingConsole asserts keys are delivered to the ui tree when
// KeyDest is KeyConsole, and not when KeyGame (gameplay only, ADR-0003).
func TestGatewayKeyRoutingConsole(t *testing.T) {
	kd := &fakeKeyDest{dest: input.KeyConsole}
	g := NewGateway(kd)

	var gotKey gpucontext.Key
	var gotDown bool
	var keyCalls int
	g.OnKeyPress(func(k gpucontext.Key, _ gpucontext.Modifiers) {
		gotKey = k
		gotDown = true
		keyCalls++
	})
	g.OnKeyRelease(func(k gpucontext.Key, _ gpucontext.Modifiers) {
		gotKey = k
		gotDown = false
		keyCalls++
	})

	// KeyConsole: delivered to ui.
	g.FeedKeyPress(gpucontext.KeyEnter)
	if keyCalls != 1 || gotKey != gpucontext.KeyEnter || !gotDown {
		t.Fatalf("KeyConsole press not delivered: calls=%d key=%v down=%v", keyCalls, gotKey, gotDown)
	}
	g.FeedKeyRelease(gpucontext.KeyEnter)
	if keyCalls != 2 || gotDown {
		t.Fatalf("KeyConsole release not delivered: calls=%d down=%v", keyCalls, gotDown)
	}

	// KeyGame: NOT delivered (gameplay only).
	kd.dest = input.KeyGame
	g.FeedKeyPress(gpucontext.KeyEnter)
	if keyCalls != 2 {
		t.Fatalf("KeyGame press leaked to ui: calls=%d", keyCalls)
	}
}

// TestGatewayKeyRoutingMenu asserts keys are delivered when KeyDest is
// KeyMenu.
func TestGatewayKeyRoutingMenu(t *testing.T) {
	kd := &fakeKeyDest{dest: input.KeyMenu}
	g := NewGateway(kd)

	var calls int
	g.OnKeyPress(func(gpucontext.Key, gpucontext.Modifiers) { calls++ })

	g.FeedKeyPress(gpucontext.KeyEscape)
	if calls != 1 {
		t.Fatalf("KeyMenu press not delivered: calls=%d", calls)
	}
}

// TestGatewayTextInputRouting asserts char/text input is delivered only when
// a text-capable surface (console/menu) is active.
func TestGatewayTextInputRouting(t *testing.T) {
	kd := &fakeKeyDest{dest: input.KeyConsole}
	g := NewGateway(kd)

	var got string
	g.OnTextInput(func(s string) { got = s })

	g.FeedTextInput("hello")
	if got != "hello" {
		t.Fatalf("console text input not delivered: got=%q", got)
	}

	kd.dest = input.KeyGame
	got = ""
	g.FeedTextInput("world")
	if got != "" {
		t.Fatalf("game text input leaked to ui: got=%q", got)
	}
}

// TestGatewayMouseRouting asserts mouse events are delivered when a ui
// surface is active and suppressed during gameplay.
func TestGatewayMouseRouting(t *testing.T) {
	kd := &fakeKeyDest{dest: input.KeyMenu}
	g := NewGateway(kd)

	var pressCalls int
	var moveCalls int
	g.OnMousePress(func(gpucontext.MouseButton, float64, float64) { pressCalls++ })
	g.OnMouseMove(func(float64, float64) { moveCalls++ })

	g.FeedMousePress(gpucontext.MouseButtonLeft, 10, 20)
	if pressCalls != 1 {
		t.Fatalf("menu mouse press not delivered: calls=%d", pressCalls)
	}
	g.FeedMouseMove(30, 40)
	if moveCalls != 1 {
		t.Fatalf("menu mouse move not delivered: calls=%d", moveCalls)
	}

	kd.dest = input.KeyGame
	g.FeedMousePress(gpucontext.MouseButtonLeft, 10, 20)
	if pressCalls != 1 {
		t.Fatalf("game mouse press leaked to ui: calls=%d", pressCalls)
	}
}

// TestGatewayScrollRouting asserts scroll events are delivered to the ui tree
// when a scrollable surface is active.
func TestGatewayScrollRouting(t *testing.T) {
	kd := &fakeKeyDest{dest: input.KeyConsole}
	g := NewGateway(kd)

	var gotDX, gotDY float64
	var calls int
	g.OnScroll(func(dx, dy float64) {
		gotDX, gotDY = dx, dy
		calls++
	})

	g.FeedScroll(0, -3)
	if calls != 1 || gotDX != 0 || gotDY != -3 {
		t.Fatalf("console scroll not delivered: calls=%d dx=%v dy=%v", calls, gotDX, gotDY)
	}

	kd.dest = input.KeyGame
	g.FeedScroll(0, -3)
	if calls != 1 {
		t.Fatalf("game scroll leaked to ui: calls=%d", calls)
	}
}

// TestGatewayKeyTranslation asserts engine key codes map to gpucontext keys.
func TestGatewayKeyTranslation(t *testing.T) {
	cases := []struct {
		engine int
		want   gpucontext.Key
	}{
		{input.KEnter, gpucontext.KeyEnter},
		{input.KEscape, gpucontext.KeyEscape},
		{input.KTab, gpucontext.KeyTab},
		{input.KSpace, gpucontext.KeySpace},
		{input.KBackspace, gpucontext.KeyBackspace},
		{input.KUpArrow, gpucontext.KeyUp},
		{input.KDownArrow, gpucontext.KeyDown},
		{input.KLeftArrow, gpucontext.KeyLeft},
		{input.KRightArrow, gpucontext.KeyRight},
		{input.KHome, gpucontext.KeyHome},
		{input.KEnd, gpucontext.KeyEnd},
		{input.KPgUp, gpucontext.KeyPageUp},
		{input.KPgDn, gpucontext.KeyPageDown},
		{input.KIns, gpucontext.KeyInsert},
		{input.KDel, gpucontext.KeyDelete},
		{int('a'), gpucontext.KeyA},
		{int('w'), gpucontext.KeyW},
		{int('0'), gpucontext.Key0},
		{int('9'), gpucontext.Key9},
	}
	for _, c := range cases {
		got := EngineKeyToGPU(c.engine)
		if got != c.want {
			t.Errorf("EngineKeyToGPU(%d) = %v, want %v", c.engine, got, c.want)
		}
	}
}

// TestGatewayKeyTranslationUnknown asserts unmapped engine codes become
// KeyUnknown.
func TestGatewayKeyTranslationUnknown(t *testing.T) {
	if got := EngineKeyToGPU(input.KCommand); got != gpucontext.KeyUnknown {
		t.Fatalf("KCommand mapped to %v, want KeyUnknown", got)
	}
	if got := EngineKeyToGPU(input.KMWheelUp); got != gpucontext.KeyUnknown {
		t.Fatalf("KMWheelUp mapped to %v, want KeyUnknown", got)
	}
}
