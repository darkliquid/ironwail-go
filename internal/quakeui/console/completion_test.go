package console

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/quakeui/widgets"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// testCompleter returns a completer that matches "r_waterwarp" and
// "r_gamma" for the "r_" prefix.
func testCompleter() CompleteFunc {
	return func(input string, forward bool) (string, []string) {
		if input == "r_" {
			return "r_", []string{"r_waterwarp", "r_gamma"}
		}
		return input, nil
	}
}

// TestCompletionBridgeMatches asserts Tab yields a match list.
func TestCompletionBridgeMatches(t *testing.T) {
	wt := widgets.NewQuakeText(testConchars(), nil)
	cb := NewCompletionBridge(testCompleter(), wt)

	matches := cb.Complete("r_", true)
	if len(matches) != 2 {
		t.Fatalf("matches = %d, want 2", len(matches))
	}
	if matches[0] != "r_waterwarp" || matches[1] != "r_gamma" {
		t.Fatalf("matches = %v, want [r_waterwarp r_gamma]", matches)
	}
}

// TestCompletionBridgeNoMatches asserts an unmatched input yields no list.
func TestCompletionBridgeNoMatches(t *testing.T) {
	wt := widgets.NewQuakeText(testConchars(), nil)
	cb := NewCompletionBridge(testCompleter(), wt)

	matches := cb.Complete("zzz", true)
	if len(matches) != 0 {
		t.Fatalf("matches = %d, want 0", len(matches))
	}
}

// TestCompletionBridgeCycle asserts forward/back cycling advances the match
// index.
func TestCompletionBridgeCycle(t *testing.T) {
	wt := widgets.NewQuakeText(testConchars(), nil)
	cb := NewCompletionBridge(testCompleter(), wt)

	cb.Complete("r_", true)
	if cb.Current() != "r_waterwarp" {
		t.Fatalf("Current() after first = %q, want r_waterwarp", cb.Current())
	}

	cb.Cycle(true)
	if cb.Current() != "r_gamma" {
		t.Fatalf("Current() after forward cycle = %q, want r_gamma", cb.Current())
	}

	cb.Cycle(true)
	if cb.Current() != "r_waterwarp" {
		t.Fatalf("Current() after wrap = %q, want r_waterwarp", cb.Current())
	}

	cb.Cycle(false)
	if cb.Current() != "r_gamma" {
		t.Fatalf("Current() after backward cycle = %q, want r_gamma", cb.Current())
	}
}

// TestCompletionBridgeLayout asserts the widget lays out to a positive size.
func TestCompletionBridgeLayout(t *testing.T) {
	wt := widgets.NewQuakeText(testConchars(), nil)
	cb := NewCompletionBridge(testCompleter(), wt)

	ctx := widget.NewContext()
	size := cb.Layout(ctx, geometry.Expand())
	if size.Width <= 0 || size.Height <= 0 {
		t.Fatalf("Layout size = %v, want positive", size)
	}
}
