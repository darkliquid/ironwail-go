package console

import (
	"strings"
	"testing"
	"time"

	"github.com/ironwail/ironwail-go/internal/image"
)

type mockRenderContext struct {
	characters []struct{ x, y, num int }
	fills      []struct {
		x, y, w, h int
		color      byte
	}
}

func (m *mockRenderContext) Clear(r, g, b, a float32)          {}
func (m *mockRenderContext) DrawTriangle(r, g, b, a float32)   {}
func (m *mockRenderContext) SurfaceView() interface{}          { return nil }
func (m *mockRenderContext) Gamma() float32                    { return 1 }
func (m *mockRenderContext) DrawPic(x, y int, pic *image.QPic) {}
func (m *mockRenderContext) DrawMenuPic(x, y int, pic *image.QPic) {
}
func (m *mockRenderContext) DrawFill(x, y, w, h int, color byte) {
	m.fills = append(m.fills, struct {
		x, y, w, h int
		color      byte
	}{x, y, w, h, color})
}
func (m *mockRenderContext) DrawCharacter(x, y int, num int) {
	m.characters = append(m.characters, struct{ x, y, num int }{x, y, num})
}
func (m *mockRenderContext) DrawMenuCharacter(x, y int, num int) {
	m.DrawCharacter(x, y, num)
}

func TestConsoleDrawRendersConsoleLinesAndPrompt(t *testing.T) {
	c := NewConsole(DefaultTextSize)
	if err := c.Init(DefaultLineWidth); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	c.Printf("line1\nline2")
	c.AppendInputRune('s')
	c.AppendInputRune('v')

	mock := &mockRenderContext{}
	c.Draw(mock, 80, 80, true)

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
	if got := strings.TrimSpace(linesByY[32]); got != "]sv" {
		t.Fatalf("prompt row = %q, want %q", got, "]sv")
	}
}

func TestConsoleDrawNotifyHonorsNotifyLifetime(t *testing.T) {
	c := NewConsole(DefaultTextSize)
	if err := c.Init(DefaultLineWidth); err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	c.Printf("old\nnew")
	c.notifyTimes[(c.current-1)%NumNotifyTimes] = time.Now().Add(-consoleNotifyTTL - time.Second)
	c.notifyTimes[c.current%NumNotifyTimes] = time.Now()

	mock := &mockRenderContext{}
	c.Draw(mock, 80, 40, false)

	linesByY := charactersByRow(mock.characters)
	if containsRowSubstring(linesByY, "old") {
		t.Fatalf("notify draw should skip expired line, rows: %#v", linesByY)
	}
	if !containsRowSubstring(linesByY, "new") {
		t.Fatalf("notify draw did not include fresh line, rows: %#v", linesByY)
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
