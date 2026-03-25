package console

import (
	"strings"
	"testing"
	"time"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/image"
)

func registerConsoleNotifyTestCvars() {
	cvar.Register("con_notifytime", "3", cvar.FlagArchive, "test console notify lifetime")
	cvar.Register("con_notifycenter", "0", cvar.FlagArchive, "test console notify centering")
	cvar.Register("con_notifyfade", "0", cvar.FlagArchive, "test console notify fade enable")
	cvar.Register("con_notifyfadetime", "0.5", cvar.FlagArchive, "test console notify fade duration")
}

type mockRenderContext struct {
	characters []struct{ x, y, num int }
	pics       []struct {
		x, y int
		pic  *image.QPic
	}
	fills []struct {
		x, y, w, h int
		color      byte
	}
}

func (m *mockRenderContext) Clear(r, g, b, a float32)        {}
func (m *mockRenderContext) DrawTriangle(r, g, b, a float32) {}
func (m *mockRenderContext) SurfaceView() interface{}        { return nil }
func (m *mockRenderContext) Gamma() float32                  { return 1 }
func (m *mockRenderContext) DrawPic(x, y int, pic *image.QPic) {
	m.pics = append(m.pics, struct {
		x, y int
		pic  *image.QPic
	}{x, y, pic})
}
func (m *mockRenderContext) DrawMenuPic(x, y int, pic *image.QPic) {
}
func (m *mockRenderContext) DrawFill(x, y, w, h int, color byte) {
	m.fills = append(m.fills, struct {
		x, y, w, h int
		color      byte
	}{x, y, w, h, color})
}
func (m *mockRenderContext) DrawFillAlpha(x, y, w, h int, color byte, alpha float32) {
	m.DrawFill(x, y, w, h, color)
}
func (m *mockRenderContext) DrawCharacter(x, y int, num int) {
	m.characters = append(m.characters, struct{ x, y, num int }{x, y, num})
}
func (m *mockRenderContext) DrawMenuCharacter(x, y int, num int) {
	m.DrawCharacter(x, y, num)
}

// TestConsoleDrawRendersConsoleLinesAndPrompt tests the console's rendering logic.
// It ensures the console correctly displays logged text lines, the command prompt (]), and the current input line.
// Where in C: Con_DrawConsole in console.c
func TestConsoleDrawRendersConsoleLinesAndPrompt(t *testing.T) {
	originalNow := consoleNow
	consoleNow = func() time.Time { return time.Unix(0, 0) }
	t.Cleanup(func() {
		consoleNow = originalNow
	})

	c := NewConsole(DefaultTextSize)
	if err := c.Init(DefaultLineWidth); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	c.Printf("line1\nline2")
	c.AppendInputRune('s')
	c.AppendInputRune('v')

	mock := &mockRenderContext{}
	c.Draw(mock, 80, 80, true, nil)

	if len(mock.fills) == 0 {
		t.Fatalf("Draw() did not draw console background")
	}

	linesByY := charactersByRow(mock.characters)
	if !containsRowSubstring(linesByY, "line1") {
		t.Fatalf("console draw did not include line1, rows: %#v", linesByY)
	}
	if !containsRowSubstring(linesByY, "line2") {
		t.Fatalf("console draw did not include line2, rows: %#v", linesByY)
	}
	if got := linesByY[32]; !strings.HasPrefix(got, "]sv") {
		t.Fatalf("prompt row = %q, want prefix %q", got, "]sv")
	}
}

func TestConsoleDrawRendersBlinkCursorAndClipsPrompt(t *testing.T) {
	originalNow := consoleNow
	consoleNow = func() time.Time { return time.Unix(0, int64(time.Second/4)) }
	t.Cleanup(func() {
		consoleNow = originalNow
	})

	c := NewConsole(DefaultTextSize)
	if err := c.Init(DefaultLineWidth); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	c.SetInputLine("longcommand")

	mock := &mockRenderContext{}
	c.Draw(mock, 80, 80, true, nil)

	var row []struct{ x, y, num int }
	for _, ch := range mock.characters {
		if ch.y == 32 {
			row = append(row, ch)
		}
	}
	if len(row) == 0 {
		t.Fatal("expected prompt row characters")
	}
	last := row[len(row)-1]
	if last.x != 64 || last.num != 11 {
		t.Fatalf("cursor draw = (%d,%d), want (%d,%d)", last.x, last.num, 64, 11)
	}
	if got := string(rune(row[0].num)); got != "]" {
		t.Fatalf("prompt prefix = %q, want %q", got, "]")
	}
	if got := string(rune(row[1].num)) + string(rune(row[2].num)); got != "om" {
		t.Fatalf("clipped tail = %q, want %q", got, "om")
	}
}

// TestConsoleDrawNotifyHonorsNotifyLifetime tests the "notify" (overlay) console display.
// It ensures that messages shown at the top of the screen during gameplay expire and disappear after a set time.
// Where in C: Con_DrawNotify and con_notifylines in console.c
func TestConsoleDrawNotifyHonorsNotifyLifetime(t *testing.T) {
	registerConsoleNotifyTestCvars()
	cvar.Set("con_notifytime", "3")
	c := NewConsole(DefaultTextSize)
	if err := c.Init(DefaultLineWidth); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	c.Printf("old\nnew")
	c.notifyTimes[(c.current-1)%NumNotifyTimes] = time.Now().Add(-consoleNotifyTTL() - time.Second)
	c.notifyTimes[c.current%NumNotifyTimes] = time.Now()

	mock := &mockRenderContext{}
	c.Draw(mock, 80, 40, false, nil)

	linesByY := charactersByRow(mock.characters)
	if containsRowSubstring(linesByY, "old") {
		t.Fatalf("notify draw should skip expired line, rows: %#v", linesByY)
	}
	if !containsRowSubstring(linesByY, "new") {
		t.Fatalf("notify draw did not include fresh line, rows: %#v", linesByY)
	}
}

func TestConsoleDrawNotifyUsesCVarLifetime(t *testing.T) {
	registerConsoleNotifyTestCvars()
	cvar.Set("con_notifytime", "1")

	c := NewConsole(DefaultTextSize)
	if err := c.Init(DefaultLineWidth); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	c.Printf("linger")
	c.notifyTimes[c.current%NumNotifyTimes] = time.Now().Add(-1500 * time.Millisecond)

	mock := &mockRenderContext{}
	c.Draw(mock, 80, 40, false, nil)

	linesByY := charactersByRow(mock.characters)
	if containsRowSubstring(linesByY, "linger") {
		t.Fatalf("notify draw should respect con_notifytime, rows: %#v", linesByY)
	}
}

func TestConsoleDrawNotifyCanCenterLines(t *testing.T) {
	originalNow := consoleNow
	consoleNow = func() time.Time { return time.Unix(10, 0) }
	t.Cleanup(func() {
		consoleNow = originalNow
	})

	registerConsoleNotifyTestCvars()
	cvar.Set("con_notifycenter", "1")
	t.Cleanup(func() {
		cvar.Set("con_notifycenter", "0")
	})

	c := NewConsole(DefaultTextSize)
	if err := c.Init(DefaultLineWidth); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	c.Printf("abc")
	c.notifyTimes[c.current%NumNotifyTimes] = consoleNow()

	mock := &mockRenderContext{}
	c.Draw(mock, 80, 40, false, nil)

	if len(mock.characters) != 3 {
		t.Fatalf("centered notify chars = %d, want 3", len(mock.characters))
	}
	if first := mock.characters[0]; first.x != 28 || first.y != 16 {
		t.Fatalf("first centered notify char = (%d,%d), want (28,16)", first.x, first.y)
	}
}

func TestConsoleDrawNotifyFadeStipplesLateLines(t *testing.T) {
	originalNow := consoleNow
	consoleNow = func() time.Time { return time.Unix(10, 0) }
	t.Cleanup(func() {
		consoleNow = originalNow
	})

	registerConsoleNotifyTestCvars()
	cvar.Set("con_notifyfade", "1")
	cvar.Set("con_notifyfadetime", "1")
	t.Cleanup(func() {
		cvar.Set("con_notifyfade", "0")
		cvar.Set("con_notifyfadetime", "0.5")
	})

	c := NewConsole(DefaultTextSize)
	if err := c.Init(DefaultLineWidth); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	c.Printf("fading")
	c.notifyTimes[c.current%NumNotifyTimes] = consoleNow().Add(-consoleNotifyTTL() - 750*time.Millisecond)

	mock := &mockRenderContext{}
	c.Draw(mock, 80, 40, false, nil)

	if got := len(mock.characters); got == 0 || got >= len("fading") {
		t.Fatalf("late notify char count = %d, want partial stipple between 1 and %d", got, len("fading")-1)
	}
}

func TestConsoleDrawResizesWithMargins(t *testing.T) {
	c := NewConsole(DefaultTextSize)
	if err := c.Init(DefaultLineWidth); err != nil {
		t.Fatalf("Init failed: %v", err)
	}
	mock := &mockRenderContext{}
	c.Draw(mock, 80, 80, true, nil)
	if got := c.LineWidth(); got != 8 {
		t.Fatalf("line width = %d, want 8 (charsWide 10 minus 2 margins)", got)
	}
}

// TestConsoleDrawUsesBackgroundPicWhenProvided tests the console background image rendering.
// It allowing the engine to use a custom image (e.g., conback.lmp) for the console background instead of a solid color.
// Where in C: Con_DrawConsole in console.c
func TestConsoleDrawUsesBackgroundPicWhenProvided(t *testing.T) {
	c := NewConsole(DefaultTextSize)
	if err := c.Init(DefaultLineWidth); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	c.Printf("hello")
	bg := &image.QPic{Width: 320, Height: 200, Pixels: make([]byte, 320*200)}
	mock := &mockRenderContext{}
	c.Draw(mock, 640, 480, true, bg)

	if len(mock.pics) != 1 {
		t.Fatalf("Draw() background pics = %d, want 1", len(mock.pics))
	}
	if got := mock.pics[0].pic.Width; got != 640 {
		t.Fatalf("background width = %d, want 640", got)
	}
	if got := mock.pics[0].pic.Height; got != 240 {
		t.Fatalf("background height = %d, want 240", got)
	}
	if got := len(mock.pics[0].pic.Pixels); got != 640*240 {
		t.Fatalf("background pixel count = %d, want %d", got, 640*240)
	}
	if len(mock.fills) != 0 {
		t.Fatalf("Draw() should prefer background pic over solid fill, got %d fills", len(mock.fills))
	}

	linesByY := charactersByRow(mock.characters)
	if !containsRowSubstring(linesByY, "hello") {
		t.Fatalf("console draw did not include text, rows: %#v", linesByY)
	}
}

func charactersByRow(chars []struct{ x, y, num int }) map[int]string {
	rows := make(map[int][]rune)
	for _, ch := range chars {
		rows[ch.y] = append(rows[ch.y], rune(ch.num))
	}

	result := make(map[int]string, len(rows))
	for y, row := range rows {
		result[y] = string(row)
	}
	return result
}

func containsRowSubstring(rows map[int]string, want string) bool {
	for _, row := range rows {
		if strings.Contains(row, want) {
			return true
		}
	}
	return false
}
