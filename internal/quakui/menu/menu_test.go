package menu

import (
	"image"
	"testing"
	"time"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/draw"
	qimage "github.com/darkliquid/ironwail-go/internal/image"
	"github.com/darkliquid/ironwail-go/internal/input"
	legacymenu "github.com/darkliquid/ironwail-go/internal/menu"
	"github.com/gogpu/ui/event"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

type drawImageCall struct {
	img image.Image
	at  geometry.Point
}

type testCanvas struct {
	images           []drawImageCall
	transforms       []geometry.Point
	currentTransform geometry.Point
}

func (c *testCanvas) Clear(color widget.Color)                                             {}
func (c *testCanvas) DrawRect(r geometry.Rect, color widget.Color)                         {}
func (c *testCanvas) FillRectDirect(r geometry.Rect, color widget.Color)                   {}
func (c *testCanvas) StrokeRect(r geometry.Rect, color widget.Color, strokeWidth float32) {}
func (c *testCanvas) DrawRoundRect(r geometry.Rect, color widget.Color, radius float32)   {}
func (c *testCanvas) StrokeRoundRect(r geometry.Rect, color widget.Color, radius float32, strokeWidth float32) {
}
func (c *testCanvas) DrawCircle(center geometry.Point, radius float32, color widget.Color)  {}
func (c *testCanvas) StrokeCircle(center geometry.Point, radius float32, color widget.Color, strokeWidth float32) {
}
func (c *testCanvas) StrokeArc(center geometry.Point, radius float32, startAngle, sweepAngle float64, color widget.Color, strokeWidth float32) {
}
func (c *testCanvas) DrawLine(from, to geometry.Point, color widget.Color, strokeWidth float32) {}
func (c *testCanvas) DrawText(text string, bounds geometry.Rect, fontSize float32, color widget.Color, bold bool, align widget.TextAlign) {
}
func (c *testCanvas) MeasureText(text string, fontSize float32, bold bool) float32 {
	return float32(len(text) * 8)
}
func (c *testCanvas) DrawImage(img image.Image, at geometry.Point) {
	c.images = append(c.images, drawImageCall{img: img, at: at.Add(c.currentTransform)})
}
func (c *testCanvas) PushClip(r geometry.Rect)                     {}
func (c *testCanvas) PushClipRoundRect(r geometry.Rect, radius float32) {}
func (c *testCanvas) PopClip()                                     {}
func (c *testCanvas) PushTransform(offset geometry.Point) {
	c.transforms = append(c.transforms, offset)
	c.currentTransform = c.currentTransform.Add(offset)
}
func (c *testCanvas) PopTransform() {
	if len(c.transforms) > 0 {
		last := c.transforms[len(c.transforms)-1]
		c.transforms = c.transforms[:len(c.transforms)-1]
		c.currentTransform = geometry.Pt(c.currentTransform.X-last.X, c.currentTransform.Y-last.Y)
	}
}
func (c *testCanvas) TransformOffset() geometry.Point              { return c.currentTransform }
func (c *testCanvas) ScreenOriginBase() geometry.Point             { return geometry.Pt(0, 0) }
func (c *testCanvas) ClipBounds() geometry.Rect                    { return geometry.Rect{} }
func (c *testCanvas) ReplayScene(cache widget.SceneCache)          {}

type testContext struct {
	invalidated bool
	animFrame   bool
	winSize     geometry.Size
}

func (ctx *testContext) RequestFocus(w widget.Widget)              {}
func (ctx *testContext) ReleaseFocus(w widget.Widget)              {}
func (ctx *testContext) IsFocused(w widget.Widget) bool            { return false }
func (ctx *testContext) FocusedWidget() widget.Widget              { return nil }
func (ctx *testContext) Now() time.Time                            { return time.Now() }
func (ctx *testContext) DeltaTime() time.Duration                  { return 0 }
func (ctx *testContext) Invalidate()                               { ctx.invalidated = true }
func (ctx *testContext) InvalidateRect(r geometry.Rect)            { ctx.invalidated = true }
func (ctx *testContext) ScheduleAnimationFrame()                   { ctx.animFrame = true }
func (ctx *testContext) Cursor() widget.CursorType                 { return widget.CursorDefault }
func (ctx *testContext) SetCursor(cursor widget.CursorType)        {}
func (ctx *testContext) Scale() float32                            { return 1.0 }
func (ctx *testContext) ThemeProvider() widget.ThemeProvider       { return nil }
func (ctx *testContext) OverlayManager() widget.OverlayManager     { return nil }
func (ctx *testContext) WindowSize() geometry.Size {
	if ctx.winSize.Width > 0 && ctx.winSize.Height > 0 {
		return ctx.winSize
	}
	return geometry.Sz(320, 200)
}
func (ctx *testContext) Scheduler() widget.SchedulerRef            { return nil }

func testConchars() []byte {
	data := make([]byte, 128*128)
	for y := 0; y < 128; y++ {
		for x := 0; x < 128; x++ {
			cx := x % 8
			cy := y % 8
			if cx == 0 || cy == 0 || cx == 7 || cy == 7 {
				data[y*128+x] = 254
			}
		}
	}
	return data
}

type testDrawManager struct {
	pics    map[string]*qimage.QPic
	palette []byte
}

func newTestDrawManager() *testDrawManager {
	return &testDrawManager{
		pics:    make(map[string]*qimage.QPic),
		palette: draw.DefaultQuakePalette(),
	}
}

func (m *testDrawManager) Pic(name string) *qimage.QPic {
	return m.pics[name]
}

func (m *testDrawManager) Palette() []byte {
	return m.palette
}

func (m *testDrawManager) addSolidPic(name string, w, h int, colorIdx byte) {
	pix := make([]byte, w*h)
	for i := range pix {
		pix[i] = colorIdx
	}
	m.pics[name] = &qimage.QPic{
		Width:  uint32(w),
		Height: uint32(h),
		Pixels: pix,
	}
}

func setupTestMenu(drawMgr legacymenu.DrawManager) (*legacymenu.Manager, *MenuRoot) {
	cvars := cvar.NewCVarSystem()
	mgr := legacymenu.NewManager(drawMgr, nil, cvars)
	mgr.ShowState(legacymenu.MenuMain)
	root := NewMenuRoot(mgr, nil, testConchars(), draw.DefaultQuakePalette())
	return mgr, root
}

func TestMenuRoot_ConstructionAndLayout(t *testing.T) {
	mgr, root := setupTestMenu(nil)
	if !root.IsVisible() {
		t.Fatal("expected root to be visible")
	}
	if !root.IsEnabled() {
		t.Fatal("expected root to be enabled")
	}

	ctx := &testContext{}
	sz := root.Layout(ctx, geometry.Loose(geometry.Sz(1280, 720)))
	if sz.Width != 1280 || sz.Height != 720 {
		t.Fatalf("layout size = %v, want 1280x720", sz)
	}
	if b := root.Bounds(); b.Width() != 1280 || b.Height() != 720 {
		t.Fatalf("bounds = %v, want 1280x720", b)
	}
	_ = mgr
}

func TestMenuRoot_DrawMain_TextFallback(t *testing.T) {
	mgr, root := setupTestMenu(nil)
	mgr.ShowState(legacymenu.MenuMain)

	ctx := &testContext{}
	canvas := &testCanvas{}
	root.Draw(ctx, canvas)

	if len(canvas.images) == 0 {
		t.Fatal("expected draw calls for conchars text, got 0")
	}

	// Verify cursor is drawn at (54, 32) for cursor 0.
	foundCursor := false
	for _, call := range canvas.images {
		if call.at.X == 54 && call.at.Y == 32 {
			foundCursor = true
			break
		}
	}
	if !foundCursor {
		t.Fatal("cursor not drawn at expected position (54, 32)")
	}

	// Verify "SINGLE PLAYER" glyphs start at (84, 32).
	foundText := false
	for _, call := range canvas.images {
		if call.at.X == 84 && call.at.Y == 32 {
			foundText = true
			break
		}
	}
	if !foundText {
		t.Fatal("SINGLE PLAYER text not drawn at expected start position (84, 32)")
	}
}

func TestMenuRoot_DrawMain_WithPics(t *testing.T) {
	drawMgr := newTestDrawManager()
	drawMgr.addSolidPic("gfx/qplaque.lmp", 32, 144, 4)
	drawMgr.addSolidPic("gfx/ttl_main.lmp", 160, 24, 4)
	drawMgr.addSolidPic("gfx/mainmenu.lmp", 100, 100, 4)
	drawMgr.addSolidPic("gfx/menudot1.lmp", 16, 16, 4)

	mgr := legacymenu.NewManager(drawMgr, nil, cvar.NewCVarSystem())
	mgr.ShowState(legacymenu.MenuMain)

	dm := draw.NewManager()
	dm.InitSyntheticFallback()

	root := NewMenuRoot(mgr, dm, testConchars(), draw.DefaultQuakePalette())
	ctx := &testContext{}
	canvas := &testCanvas{}
	root.Draw(ctx, canvas)

	if len(canvas.images) == 0 {
		t.Fatal("expected draw calls for pics/menu, got 0")
	}
}

func TestMenuRoot_DrawMain_WithMods(t *testing.T) {
	mgr, root := setupTestMenu(nil)
	mgr.SetModsProvider(func() []legacymenu.ModInfo {
		return []legacymenu.ModInfo{{Name: "hipnotic"}}
	})
	mgr.ShowState(legacymenu.MenuMain)

	ctx := &testContext{}
	canvas := &testCanvas{}
	root.Draw(ctx, canvas)

	// With mods, MODS item should be drawn at y=92, HELP at y=112, QUIT at y=132.
	foundMods := false
	foundHelp := false
	foundQuit := false
	for _, call := range canvas.images {
		if call.at.X == 84 && call.at.Y == 92 {
			foundMods = true
		}
		if call.at.X == 84 && call.at.Y == 112 {
			foundHelp = true
		}
		if call.at.X == 84 && call.at.Y == 132 {
			foundQuit = true
		}
	}
	if !foundMods || !foundHelp || !foundQuit {
		t.Fatalf("expected MODS(92)=%v, HELP(112)=%v, QUIT(132)=%v", foundMods, foundHelp, foundQuit)
	}
}

func TestMenuRoot_DrawSinglePlayer(t *testing.T) {
	mgr, root := setupTestMenu(nil)
	mgr.ShowState(legacymenu.MenuSinglePlayer)

	ctx := &testContext{}
	canvas := &testCanvas{}
	root.Draw(ctx, canvas)

	foundNewGame := false
	foundLoad := false
	foundSave := false
	for _, call := range canvas.images {
		if call.at.X == 84 && call.at.Y == 32 {
			foundNewGame = true
		}
		if call.at.X == 84 && call.at.Y == 52 {
			foundLoad = true
		}
		if call.at.X == 84 && call.at.Y == 72 {
			foundSave = true
		}
	}
	if !foundNewGame || !foundLoad || !foundSave {
		t.Fatalf("expected NEW GAME=%v, LOAD=%v, SAVE=%v", foundNewGame, foundLoad, foundSave)
	}
}

func TestMenuRoot_DrawOptions(t *testing.T) {
	mgr, root := setupTestMenu(nil)
	mgr.ShowState(legacymenu.MenuOptions)

	ctx := &testContext{}
	canvas := &testCanvas{}
	root.Draw(ctx, canvas)

	foundControls := false
	foundVideo := false
	foundAudio := false
	foundVSync := false
	foundBack := false
	for _, call := range canvas.images {
		if call.at.X == 84 && call.at.Y == 32 {
			foundControls = true
		}
		if call.at.X == 84 && call.at.Y == 52 {
			foundVideo = true
		}
		if call.at.X == 84 && call.at.Y == 72 {
			foundAudio = true
		}
		if call.at.X == 84 && call.at.Y == 92 {
			foundVSync = true
		}
		if call.at.X == 84 && call.at.Y == 112 {
			foundBack = true
		}
	}
	if !foundControls || !foundVideo || !foundAudio || !foundVSync || !foundBack {
		t.Fatalf("expected options labels, got controls=%v video=%v audio=%v vsync=%v back=%v",
			foundControls, foundVideo, foundAudio, foundVSync, foundBack)
	}
}

func TestMenuRoot_DrawInvisible(t *testing.T) {
	_, root := setupTestMenu(nil)
	root.SetVisible(false)

	ctx := &testContext{}
	canvas := &testCanvas{}
	root.Draw(ctx, canvas)

	if len(canvas.images) != 0 {
		t.Fatalf("expected 0 draw calls when invisible, got %d", len(canvas.images))
	}
}

func TestMenuRoot_Event_KeyboardNavigation(t *testing.T) {
	mgr, root := setupTestMenu(nil)
	mgr.ShowState(legacymenu.MenuMain)

	ctx := &testContext{}

	if c := mgr.CursorFor(legacymenu.MenuMain); c != 0 {
		t.Fatalf("initial cursor = %d, want 0", c)
	}

	// Down arrow moves cursor to 1.
	handled := root.Event(ctx, event.NewKeyEvent(event.KeyPress, event.KeyDown, 0, 0))
	if !handled {
		t.Fatal("expected Down arrow event to be handled")
	}
	if !ctx.invalidated {
		t.Fatal("expected Invalidate() call on key event")
	}
	if c := mgr.CursorFor(legacymenu.MenuMain); c != 1 {
		t.Fatalf("cursor after Down = %d, want 1", c)
	}

	// Up arrow moves cursor back to 0.
	ctx.invalidated = false
	handled = root.Event(ctx, event.NewKeyEvent(event.KeyPress, event.KeyUp, 0, 0))
	if !handled {
		t.Fatal("expected Up arrow event to be handled")
	}
	if c := mgr.CursorFor(legacymenu.MenuMain); c != 0 {
		t.Fatalf("cursor after Up = %d, want 0", c)
	}

	// Enter on slot 0 (Single Player) transitions to MenuSinglePlayer.
	handled = root.Event(ctx, event.NewKeyEvent(event.KeyPress, event.KeyEnter, 0, 0))
	if !handled {
		t.Fatal("expected Enter event to be handled")
	}
	if st := mgr.State(); st != legacymenu.MenuSinglePlayer {
		t.Fatalf("state after Enter = %v, want MenuSinglePlayer", st)
	}

	// Escape on Single Player returns to MenuMain.
	handled = root.Event(ctx, event.NewKeyEvent(event.KeyPress, event.KeyEscape, 0, 0))
	if !handled {
		t.Fatal("expected Escape event to be handled")
	}
	if st := mgr.State(); st != legacymenu.MenuMain {
		t.Fatalf("state after Escape = %v, want MenuMain", st)
	}

	// KeyRelease should be ignored.
	handled = root.Event(ctx, event.NewKeyEvent(event.KeyRelease, event.KeyDown, 0, 0))
	if handled {
		t.Fatal("expected KeyRelease to not be handled")
	}
}

func TestMenuRoot_Event_CharRouting(t *testing.T) {
	mgr, root := setupTestMenu(nil)
	mgr.ShowState(legacymenu.MenuSetup)

	ctx := &testContext{}

	// On MenuSetup, typing 't' should route to M_Char and append to hostname buffer.
	initialHost := mgr.TextBuffer("hostname")
	handled := root.Event(ctx, event.NewKeyEvent(event.KeyPress, event.KeyUnknown, 't', 0))
	if !handled {
		t.Fatal("expected char event to be handled")
	}
	if host := mgr.TextBuffer("hostname"); host != initialHost+"t" {
		t.Fatalf("hostname = %q, want %q", host, initialHost+"t")
	}

	// On MenuQuit, typing 'y' should route to M_Key('y') and confirm quit.
	mgr.ShowQuitPrompt()
	handled = root.Event(ctx, event.NewKeyEvent(event.KeyPress, event.KeyUnknown, 'y', 0))
	if !handled {
		t.Fatal("expected 'y' on quit to be handled")
	}
	if mgr.IsActive() {
		t.Fatal("expected menu to be hidden after confirming quit with 'y'")
	}
}

func TestSubPic(t *testing.T) {
	// 10x10 test image.
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))

	// Normal crop.
	sub := subPic(img, 2, 2, 4, 4)
	if sub == nil || sub.Bounds().Dx() != 4 || sub.Bounds().Dy() != 4 {
		t.Fatalf("subPic(2,2,4,4) = %v", sub)
	}

	// Out of bounds crop should be clamped.
	sub = subPic(img, 8, 8, 10, 10)
	if sub == nil || sub.Bounds().Dx() != 2 || sub.Bounds().Dy() != 2 {
		t.Fatalf("subPic clamped = %v, want 2x2", sub)
	}

	// Zero / negative size.
	if subPic(img, 10, 10, 1, 1) != nil {
		t.Fatal("expected nil for out-of-bounds start")
	}
	if subPic(img, 0, 0, 0, 0) != nil {
		t.Fatal("expected nil for 0x0 size")
	}
	if subPic(nil, 0, 0, 5, 5) != nil {
		t.Fatal("expected nil for nil image")
	}
}

func TestKeyEventToEngine(t *testing.T) {
	if keyEventToEngine(nil) != -1 {
		t.Fatal("nil event should return -1")
	}
	if keyEventToEngine(event.NewKeyEvent(event.KeyRelease, event.KeyDown, 0, 0)) != -1 {
		t.Fatal("KeyRelease should return -1")
	}
	if keyEventToEngine(event.NewKeyEvent(event.KeyPress, event.KeyUp, 0, 0)) != input.KUpArrow {
		t.Fatal("KeyUp should map to KUpArrow")
	}
	if keyEventToEngine(event.NewKeyEvent(event.KeyPress, event.KeyDown, 0, 0)) != input.KDownArrow {
		t.Fatal("KeyDown should map to KDownArrow")
	}
	if keyEventToEngine(event.NewKeyEvent(event.KeyPress, event.KeySpace, 0, 0)) != input.KSpace {
		t.Fatal("KeySpace should map to KSpace")
	}
	if keyEventToEngine(event.NewKeyEvent(event.KeyPress, event.KeyEnter, 0, 0)) != input.KEnter {
		t.Fatal("KeyEnter should map to KEnter")
	}
	if keyEventToEngine(event.NewKeyEvent(event.KeyPress, event.KeyEscape, 0, 0)) != input.KEscape {
		t.Fatal("KeyEscape should map to KEscape")
	}
	if keyEventToEngine(event.NewKeyEvent(event.KeyPress, event.KeyUnknown, 'q', 0)) != int('q') {
		t.Fatal("printable rune 'q' should map to int('q')")
	}
}

func TestMenuRoot_Draw_ContinuousAnimation(t *testing.T) {
	_, root := setupTestMenu(nil)
	ctx := &testContext{}
	canvas := &testCanvas{}

	root.Draw(ctx, canvas)
	if !ctx.animFrame {
		t.Fatal("expected MenuRoot.Draw to schedule continuous animation frame")
	}
}

func TestMenuRoot_Draw_CenteringAtWindowSizes(t *testing.T) {
	_, root := setupTestMenu(nil)

	// Test at 1280x720 window: centering offset should be (1280-320)/2 = 480, (720-200)/2 = 260.
	ctx1280 := &testContext{winSize: geometry.Sz(1280, 720)}
	canvas1280 := &testCanvas{}
	root.Draw(ctx1280, canvas1280)

	foundCenteredCursor := false
	for _, call := range canvas1280.images {
		// Cursor 0 was at (54, 32). With offset (480, 260), it should land at (534, 292).
		if call.at.X == 54+480 && call.at.Y == 32+260 {
			foundCenteredCursor = true
			break
		}
	}
	if !foundCenteredCursor {
		t.Fatalf("expected cursor at (534, 292) for 1280x720 window, got calls: %+v", canvas1280.images[:min(3, len(canvas1280.images))])
	}

	// Test at 320x200 window: centering offset is (0, 0).
	ctx320 := &testContext{winSize: geometry.Sz(320, 200)}
	canvas320 := &testCanvas{}
	root.Draw(ctx320, canvas320)

	foundBaseCursor := false
	for _, call := range canvas320.images {
		if call.at.X == 54 && call.at.Y == 32 {
			foundBaseCursor = true
			break
		}
	}
	if !foundBaseCursor {
		t.Fatal("expected cursor at (54, 32) for 320x200 window")
	}
}

func TestMenuRoot_IsRepaintBoundary(t *testing.T) {
	_, root := setupTestMenu(nil)
	if !root.IsRepaintBoundary() {
		t.Fatal("expected MenuRoot to have IsRepaintBoundary() == true for dedicated PictureLayer")
	}
}

func TestMenuRoot_IsVisibleDynamic(t *testing.T) {
	mgr, root := setupTestMenu(nil)

	// Inactive menu
	mgr.HideMenu()
	if root.IsVisible() {
		t.Fatal("expected IsVisible() == false when mgr is inactive")
	}
	if root.WidgetBase.IsVisible() {
		t.Fatal("expected WidgetBase.IsVisible() == false after transition to inactive")
	}
	if !root.IsSceneDirty() {
		t.Fatal("expected IsSceneDirty() == true after transition to inactive")
	}
	root.ClearSceneDirty()

	// Active menu
	mgr.ShowState(legacymenu.MenuMain)
	if !root.IsVisible() {
		t.Fatal("expected IsVisible() == true when mgr is active")
	}
	if !root.WidgetBase.IsVisible() {
		t.Fatal("expected WidgetBase.IsVisible() == true after transition to active")
	}
	if !root.IsSceneDirty() {
		t.Fatal("expected IsSceneDirty() == true after transition to active")
	}
	root.ClearSceneDirty()

	// Hide again (e.g. game started)
	mgr.HideMenu()
	if root.IsVisible() {
		t.Fatal("expected IsVisible() == false after HideMenu()")
	}
	if root.WidgetBase.IsVisible() {
		t.Fatal("expected WidgetBase.IsVisible() == false after HideMenu()")
	}
	if !root.IsSceneDirty() {
		t.Fatal("expected IsSceneDirty() == true after HideMenu()")
	}
}


