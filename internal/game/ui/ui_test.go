package ui_test

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/game/ui"

)

func TestGUIDimensions(t *testing.T) {
	w, h := ui.GUIDimensions(1920, 1080, 1920, 1080, 1.0)
	if w != 1920 || h != 1080 {
		t.Fatalf("GUIDimensions = (%d, %d), want (1920, 1080)", w, h)
	}

	w, h = ui.GUIDimensions(0, 0, 1280, 720, 1.0)
	if w != 1280 || h != 720 {
		t.Fatalf("GUIDimensions fallback = (%d, %d), want (1280, 720)", w, h)
	}
}

func TestConsoleDimensionsClamping(t *testing.T) {
	w, h := ui.ConsoleDimensions(1920, 1080, 0, 0)
	if w < 320 || w > 1920 {
		t.Fatalf("ConsoleDimensions width = %d, want in [320, 1920]", w)
	}
	if h <= 0 {
		t.Fatalf("ConsoleDimensions height = %d, want > 0", h)
	}

	w, h = ui.ConsoleDimensions(200, 150, 0, 0)
	if w != 200 || h != 150 {
		t.Fatalf("ConsoleDimensions small screen = (%d, %d), want (200, 150)", w, h)
	}
}

func TestStepConsoleSlide(t *testing.T) {
	frac, anim := ui.StepConsoleSlide(0.0, 2.0, 0.25, 1.0)
	if frac != 0.5 || !anim {
		t.Fatalf("StepConsoleSlide = (%f, %v), want (0.5, true)", frac, anim)
	}

	frac, anim = ui.StepConsoleSlide(0.9, 2.0, 0.1, 1.0)
	if frac != 1.0 || anim {
		t.Fatalf("StepConsoleSlide finish = (%f, %v), want (1.0, false)", frac, anim)
	}
}

func TestClampF64(t *testing.T) {
	if got := ui.ClampF64(5, 0, 10); got != 5 {
		t.Fatalf("ClampF64 mid = %v, want 5", got)
	}
	if got := ui.ClampF64(-1, 0, 10); got != 0 {
		t.Fatalf("ClampF64 low = %v, want 0", got)
	}
	if got := ui.ClampF64(11, 0, 10); got != 10 {
		t.Fatalf("ClampF64 high = %v, want 10", got)
	}
}

func TestDemoName(t *testing.T) {
	tests := []struct{ in, want string }{
		{"demos/demo1", "demo1"},
		{"demo1.dem", "demo1"},
		{"demos/t1x", "t1x"},
		{"demos/", "demos"},
	}
	for _, tc := range tests {
		if got := ui.DemoName(tc.in); got != tc.want {
			t.Errorf("DemoName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
	long := ui.DemoName("demos/" + string(make([]byte, 40)))
	if len(long) != 30 {
		t.Fatalf("DemoName long = %d chars, want 30", len(long))
	}
}

func TestFormatDemoBaseSpeed(t *testing.T) {
	if got := ui.FormatDemoBaseSpeed(0); got != "" {
		t.Fatalf("speed 0 = %q, want empty", got)
	}
	if got := ui.FormatDemoBaseSpeed(2); got != "2x" {
		t.Fatalf("speed 2 = %q, want 2x", got)
	}
	if got := ui.FormatDemoBaseSpeed(0.5); got != "1/2x" {
		t.Fatalf("speed 0.5 = %q, want 1/2x", got)
	}
}
