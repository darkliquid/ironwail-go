package hud

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/hud"
	"github.com/darkliquid/ironwail-go/internal/quakeui/widgets"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/widget"
)

// testCVars returns a cvar system with the HUD cvars registered.
func testCVars(t *testing.T) *cvar.CVarSystem {
	t.Helper()
	cvs := cvar.NewCVarSystem()
	cvs.Register("crosshair", "1", 0, "crosshair style")
	cvs.Register("scr_centerprintbg", "0", 0, "centerprint bg mode")
	cvs.Register("scr_printspeed", "8", 0, "typewriter speed")
	cvs.Register("viewsize", "100", 0, "view size")
	return cvs
}

// TestCrosshairGlyph asserts the crosshair glyph maps from the crosshair cvar
// (0 off, 1 '+', >1 dot char 15, <0 custom char).
func TestCrosshairGlyph(t *testing.T) {
	cvs := testCVars(t)
	wt := widgets.NewQuakeText(testConchars(), nil)

	// 0 = off.
	cvs.Set("crosshair", "0")
	cw := NewCrosshairWidget(cvs, wt)
	if cw.Glyph() != 0 {
		t.Fatalf("Glyph() with crosshair 0 = %d, want 0 (off)", cw.Glyph())
	}

	// 1 = '+'.
	cvs.Set("crosshair", "1")
	cw = NewCrosshairWidget(cvs, wt)
	if cw.Glyph() != int('+') {
		t.Fatalf("Glyph() with crosshair 1 = %d, want %d ('+')", cw.Glyph(), int('+'))
	}

	// >1 = dot (char 15).
	cvs.Set("crosshair", "2")
	cw = NewCrosshairWidget(cvs, wt)
	if cw.Glyph() != 15 {
		t.Fatalf("Glyph() with crosshair 2 = %d, want 15 (dot)", cw.Glyph())
	}

	// <0 = custom char.
	cvs.Set("crosshair", "-3")
	cw = NewCrosshairWidget(cvs, wt)
	if cw.Glyph() != 3 {
		t.Fatalf("Glyph() with crosshair -3 = %d, want 3", cw.Glyph())
	}
}

// TestCrosshairHidden asserts the crosshair is hidden during intermission and
// at viewsize >= 130.
func TestCrosshairHidden(t *testing.T) {
	cvs := testCVars(t)
	wt := widgets.NewQuakeText(testConchars(), nil)
	cw := NewCrosshairWidget(cvs, wt)

	// Visible normally.
	if cw.Hidden(hud.State{}) {
		t.Fatal("Hidden() = true for normal state, want false")
	}

	// Hidden during intermission.
	if !cw.Hidden(hud.State{Intermission: 1}) {
		t.Fatal("Hidden() = false during intermission, want true")
	}

	// Hidden at viewsize >= 130.
	cvs.Set("viewsize", "130")
	if !cw.Hidden(hud.State{}) {
		t.Fatal("Hidden() = false at viewsize 130, want true")
	}
}

// TestCenterprintTypewriter asserts the typewriter reveal at scr_printspeed.
func TestCenterprintTypewriter(t *testing.T) {
	cvs := testCVars(t)
	wt := widgets.NewQuakeText(testConchars(), nil)
	cw := NewCenterprintWidget(cvs, wt)

	// At time 0 (start), no chars revealed.
	st := hud.State{Intermission: 2, CenterPrint: "HELLO", CenterPrintAt: 0, Time: 0}
	if got := cw.RevealedText(st); got != "" {
		t.Fatalf("RevealedText at t=0 = %q, want empty", got)
	}

	// At 8 chars/sec, after 1s all 5 chars revealed.
	st.Time = 1.0
	if got := cw.RevealedText(st); got != "HELLO" {
		t.Fatalf("RevealedText at t=1 = %q, want HELLO", got)
	}

	// After 0.5s, 4 chars revealed.
	st.Time = 0.5
	if got := cw.RevealedText(st); got != "HELL" {
		t.Fatalf("RevealedText at t=0.5 = %q, want HELL", got)
	}
}

// TestCenterprintBgMode asserts the background mode is read from
// scr_centerprintbg.
func TestCenterprintBgMode(t *testing.T) {
	cvs := testCVars(t)
	wt := widgets.NewQuakeText(testConchars(), nil)
	cw := NewCenterprintWidget(cvs, wt)

	if cw.BgMode() != 0 {
		t.Fatalf("BgMode() = %d, want 0", cw.BgMode())
	}
	cvs.Set("scr_centerprintbg", "2")
	if cw.BgMode() != 2 {
		t.Fatalf("BgMode() = %d, want 2", cw.BgMode())
	}
}

// TestHUDWidgetsLayout asserts both widgets lay out to a positive size.
func TestHUDWidgetsLayout(t *testing.T) {
	cvs := testCVars(t)
	wt := widgets.NewQuakeText(testConchars(), nil)

	ctx := widget.NewContext()
	for _, w := range []widget.Widget{
		NewCrosshairWidget(cvs, wt),
		NewCenterprintWidget(cvs, wt),
	} {
		size := w.Layout(ctx, geometry.Expand())
		if size.Width <= 0 || size.Height <= 0 {
			t.Fatalf("Layout size = %v, want positive", size)
		}
	}
}
