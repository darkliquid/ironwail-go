package console

import (
	"image"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/draw"
	"github.com/darkliquid/ironwail-go/internal/quakui/gfx"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

type testContext struct {
	widget.Context
	winSize   geometry.Size
	animFrame bool
}

func (c *testContext) WindowSize() geometry.Size {
	if c.winSize.Width <= 0 || c.winSize.Height <= 0 {
		return geometry.Sz(640, 480)
	}
	return c.winSize
}

func (c *testContext) ScheduleAnimationFrame() {
	c.animFrame = true
}

func (c *testContext) Invalidate() {}

type imageDrawCall struct {
	at geometry.Point
}

type testCanvas struct {
	widget.Canvas
	images []imageDrawCall
	rects  []geometry.Rect
}

func (c *testCanvas) PushTransform(offset geometry.Point) {}
func (c *testCanvas) PopTransform()                       {}
func (c *testCanvas) TransformOffset() geometry.Point     { return geometry.Pt(0, 0) }
func (c *testCanvas) DrawImage(img image.Image, at geometry.Point) {
	c.images = append(c.images, imageDrawCall{at: at})
}
func (c *testCanvas) DrawRect(bounds geometry.Rect, col widget.Color) {
	c.rects = append(c.rects, bounds)
}

func setupTestConsole() (*console.Console, *ConsoleRoot) {
	con := &console.Console{
		Title: "Test Console",
	}
	_ = con.Init(1024)

	rawConchars := make([]byte, 128*128)
	for i := range rawConchars {
		rawConchars[i] = 1
	}
	atlas := gfx.NewConcharsAtlas(rawConchars, draw.DefaultQuakePalette())
	root := NewConsoleRoot(con, nil, atlas)
	return con, root
}

func TestConsoleRoot_Layout(t *testing.T) {
	_, root := setupTestConsole()
	ctx := &testContext{winSize: geometry.Sz(640, 480)}
	sz := root.Layout(ctx, geometry.Loose(geometry.Sz(640, 480)))
	if sz.Width != 640 || sz.Height != 480 {
		t.Fatalf("Layout size = %+v, want (640, 480)", sz)
	}
}

func TestConsoleRoot_Draw_Dropdown(t *testing.T) {
	con, root := setupTestConsole()
	con.Printf("Console line 1\n")
	con.Printf("Console line 2\n")
	root.SetSlideFraction(1.0)

	ctx := &testContext{winSize: geometry.Sz(640, 480)}
	canvas := &testCanvas{}
	root.Draw(ctx, canvas)

	if len(canvas.rects) == 0 {
		t.Fatal("expected background rectangle drawn")
	}
	if len(canvas.images) == 0 {
		t.Fatal("expected text glyph images drawn")
	}
}

func TestConsoleRoot_Draw_Notify(t *testing.T) {
	con, root := setupTestConsole()
	con.Printf("Notify message\n")
	root.SetSlideFraction(0)

	ctx := &testContext{winSize: geometry.Sz(640, 480)}
	canvas := &testCanvas{}
	root.Draw(ctx, canvas)

	if len(canvas.images) == 0 {
		t.Fatal("expected notify text glyphs drawn")
	}
}

func TestConsoleRoot_Event_Input(t *testing.T) {
	con, root := setupTestConsole()
	root.SetSlideFraction(1.0)
	ctx := &testContext{}

	var executedCmd string
	root.SetOnCommand(func(cmd string) {
		executedCmd = cmd
	})

	// Type "help"
	for _, ch := range "help" {
		root.Event(ctx, &event.KeyEvent{
			KeyType: event.KeyPress,
			Rune:    ch,
		})
	}
	if con.InputLine() != "help" {
		t.Fatalf("InputLine = %q, want %q", con.InputLine(), "help")
	}

	// Press Backspace
	root.Event(ctx, &event.KeyEvent{
		KeyType: event.KeyPress,
		Key:     event.KeyBackspace,
	})
	if con.InputLine() != "hel" {
		t.Fatalf("InputLine after backspace = %q, want %q", con.InputLine(), "hel")
	}

	// Press Enter
	root.Event(ctx, &event.KeyEvent{
		KeyType: event.KeyPress,
		Key:     event.KeyEnter,
	})
	if executedCmd != "hel" {
		t.Fatalf("executedCmd = %q, want %q", executedCmd, "hel")
	}
	if con.InputLine() != "" {
		t.Fatalf("InputLine after enter = %q, want empty", con.InputLine())
	}
}

func TestConsoleRoot_ToggleAndEscape(t *testing.T) {
	_, root := setupTestConsole()
	root.SetSlideFraction(1.0)
	ctx := &testContext{}

	toggled := false
	root.SetOnToggle(func() {
		toggled = true
	})

	root.Event(ctx, &event.KeyEvent{
		KeyType: event.KeyPress,
		Key:     event.KeyEscape,
	})
	if !toggled {
		t.Fatal("expected escape to invoke onToggle callback")
	}
}

func TestConsoleRoot_IsRepaintBoundary(t *testing.T) {
	_, root := setupTestConsole()
	if root.IsRepaintBoundary() {
		t.Fatal("expected ConsoleRoot to not be a separate RepaintBoundary (renders in main scene pass)")
	}
}

func TestConsoleRoot_TabCompletion(t *testing.T) {
	con, root := setupTestConsole()
	root.SetSlideFraction(1.0)
	ctx := &testContext{}

	console.SetGlobalCommandProvider(func(partial string) []string {
		if partial == "ma" {
			return []string{"map"}
		}
		return nil
	})

	con.SetInputLine("ma")
	root.Event(ctx, &event.KeyEvent{
		KeyType: event.KeyPress,
		Key:     event.KeyTab,
	})

	if con.InputLine() != "map" {
		t.Fatalf("InputLine after Tab completion = %q, want 'map'", con.InputLine())
	}
}

func TestConsoleRoot_TabCompletionMultipleMatches(t *testing.T) {
	con, root := setupTestConsole()
	root.SetSlideFraction(1.0)
	ctx := &testContext{winSize: geometry.Sz(640, 480)}

	console.SetGlobalCommandProvider(func(partial string) []string {
		if partial == "m" {
			return []string{"map", "messagemode", "messagemode2"}
		}
		return nil
	})

	con.SetInputLine("m")
	root.Event(ctx, &event.KeyEvent{
		KeyType: event.KeyPress,
		Key:     event.KeyTab,
	})

	if len(root.Matches()) != 3 {
		t.Fatalf("Matches() length = %d, want 3", len(root.Matches()))
	}

	canvas := &testCanvas{}
	root.Draw(ctx, canvas)
	if len(canvas.images) == 0 {
		t.Fatal("expected match list images drawn")
	}

	// Pressing another key clears matches
	root.Event(ctx, &event.KeyEvent{
		KeyType: event.KeyPress,
		Rune:    'a',
	})
	if len(root.Matches()) != 0 {
		t.Fatalf("Matches() after typing = %+v, want empty", root.Matches())
	}
}

func TestConsoleRoot_IsVisible(t *testing.T) {
	_, root := setupTestConsole()

	// Initially slideFraction=0, no notify -> not active
	if root.IsVisible() {
		t.Fatal("expected IsVisible() == false initially when slideFraction=0")
	}

	root.SetSlideFraction(0.5)
	if !root.IsVisible() {
		t.Fatal("expected IsVisible() == true when slideFraction > 0")
	}

	root.SetSlideFraction(0.0)
	if root.IsVisible() {
		t.Fatal("expected IsVisible() == false after sliding down to 0")
	}

	root.SetForcedUp(true)
	if !root.IsVisible() {
		t.Fatal("expected IsVisible() == true when forcedUp=true")
	}
}
