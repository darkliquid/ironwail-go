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
