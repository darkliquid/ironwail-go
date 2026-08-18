package console

import (
	"testing"
	"time"

	"github.com/darkliquid/ironwail-go/internal/console"
	"github.com/darkliquid/ironwail-go/internal/quakeui/widgets"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// testConchars returns a minimal 128x128 conchars atlas.
func testConchars() []byte {
	data := make([]byte, 128*128)
	for i := range data {
		data[i] = byte(i / 64)
	}
	return data
}

// testConsole builds a console with a few lines of scrollback and an input
// line, mirroring the legacy draw's data reads.
func testConsole(t *testing.T) *console.Console {
	t.Helper()
	c := console.NewConsole(console.DefaultTextSize)
	if err := c.Init(0); err != nil {
		t.Fatalf("console init: %v", err)
	}
	c.Printf("line one\n")
	c.Printf("line two\n")
	c.Printf("line three\n")
	c.SetInputLine("r_waterwarp")
	return c
}

// TestConsoleWidgetRows asserts the widget exposes the visible scrollback
// rows from the console buffer (virtualized: only the visible window).
func TestConsoleWidgetRows(t *testing.T) {
	c := testConsole(t)
	wt := widgets.NewQuakeText(testConchars(), nil)
	cw := NewConsoleWidget(c, wt)

	rows := cw.Rows()
	if len(rows) < 3 {
		t.Fatalf("rows = %d, want >= 3", len(rows))
	}
	// The most recent line is the last printed line.
	if rows[len(rows)-1] != "line three" {
		t.Fatalf("last row = %q, want %q", rows[len(rows)-1], "line three")
	}
}

// TestConsoleWidgetInputLine asserts the widget exposes the input line and
// prompt.
func TestConsoleWidgetInputLine(t *testing.T) {
	c := testConsole(t)
	wt := widgets.NewQuakeText(testConchars(), nil)
	cw := NewConsoleWidget(c, wt)

	if cw.InputLine() != "r_waterwarp" {
		t.Fatalf("InputLine() = %q, want r_waterwarp", cw.InputLine())
	}
	if cw.Prompt() != ']' {
		t.Fatalf("Prompt() = %q, want ']'", cw.Prompt())
	}
}

// TestConsoleWidgetScrollIndicator asserts the scroll indicator appears when
// backScroll is non-zero.
func TestConsoleWidgetScrollIndicator(t *testing.T) {
	c := testConsole(t)
	wt := widgets.NewQuakeText(testConchars(), nil)
	cw := NewConsoleWidget(c, wt)

	if cw.Scrolled() {
		t.Fatal("Scrolled() = true before backscroll")
	}

	c.SetBackScroll(2)
	if !cw.Scrolled() {
		t.Fatal("Scrolled() = false after backscroll 2")
	}
}

// TestConsoleWidgetNotifyAlpha asserts notify lines expose a fading alpha
// from con_notifytime (3s default) to 0.
func TestConsoleWidgetNotifyAlpha(t *testing.T) {
	c := testConsole(t)
	wt := widgets.NewQuakeText(testConchars(), nil)
	cw := NewConsoleWidget(c, wt)

	// Fresh notify line: alpha near 1.
	alpha := cw.NotifyAlpha(time.Now())
	if alpha <= 0.5 || alpha > 1.0 {
		t.Fatalf("NotifyAlpha(now) = %v, want ~1", alpha)
	}
	// Expired notify line: alpha 0.
	alpha = cw.NotifyAlpha(time.Now().Add(-10 * time.Second))
	if alpha != 0 {
		t.Fatalf("NotifyAlpha(expired) = %v, want 0", alpha)
	}
}

// TestConsoleWidgetLayout asserts the widget lays out to a positive size.
func TestConsoleWidgetLayout(t *testing.T) {
	c := testConsole(t)
	wt := widgets.NewQuakeText(testConchars(), nil)
	cw := NewConsoleWidget(c, wt)

	ctx := widget.NewContext()
	size := cw.Layout(ctx, geometry.Expand())
	if size.Width <= 0 || size.Height <= 0 {
		t.Fatalf("Layout size = %v, want positive", size)
	}
}
