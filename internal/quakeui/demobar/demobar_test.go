package demobar

import (
	"image"
	"testing"

	"github.com/gogpu/gg"
	"github.com/gogpu/ui/geometry"
	"github.com/gogpu/ui/render"
	"github.com/gogpu/ui/widget"
)

// stubProvider returns a fixed DemoBarState.
type stubProvider struct {
	st DemoBarState
}

func (p *stubProvider) DemoBarState() DemoBarState { return p.st }

// renderBar renders the bar into a CPU gg canvas and returns Pix (RGBA).
func renderBar(t *testing.T, st DemoBarState) []byte {
	t.Helper()
	conchars := make([]byte, 128*128)
	for i := range conchars {
		conchars[i] = byte(i%255 + 1)
	}
	palette := make([]byte, 768)
	for i := range palette {
		palette[i] = 255
	}
	bar := NewDemoBarRoot(&stubProvider{st: st}, conchars, palette)

	dc := gg.NewContext(320, 200)
	dc.Clear()
	canvas := render.NewCanvas(dc, 320, 200)
	ctx := widget.NewContext()
	ctx.SetWindowSize(geometry.Sz(320, 200))

	bar.Layout(ctx, geometry.Tight(geometry.Sz(320, 200)))
	bar.Draw(ctx, canvas)

	img := dc.Image()
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba.Pix
	}
	return nil
}

func TestDemoBarRendersWhenPlaybackActive(t *testing.T) {
	st := DemoBarState{
		Playback:   true,
		Show:       true,
		Speed:      1,
		BaseSpeed:  1,
		Progress:   0.5,
		Name:       "ancient",
		ClientTime: 65.0,
	}
	pix := renderBar(t, st)
	if len(pix) == 0 {
		t.Fatal("renderBar produced no pixels")
	}
	var nonZero int
	for i := 3; i < len(pix); i += 4 {
		if pix[i] > 0 {
			nonZero++
		}
	}
	if nonZero == 0 {
		t.Fatal("demo bar rendered nothing — no glyph pixels drawn")
	}
}

func TestDemoBarHiddenWhenNotPlaying(t *testing.T) {
	bar := NewDemoBarRoot(&stubProvider{st: DemoBarState{}}, make([]byte, 128*128), make([]byte, 768))
	if bar.IsVisible() {
		t.Fatal("demo bar visible without playback")
	}
}

func TestDemoBarEventFallsThrough(t *testing.T) {
	bar := NewDemoBarRoot(&stubProvider{st: DemoBarState{Playback: true, Show: true}}, make([]byte, 128*128), make([]byte, 768))
	ctx := widget.NewContext()
	if bar.Event(ctx, nil) {
		t.Fatal("demo bar consumed an event — it must be display-only (ADR-0015)")
	}
}

func TestFormatDemoBaseSpeed(t *testing.T) {
	cases := []struct {
		speed float32
		want  string
	}{
		{0, ""},
		{1, "1x"},
		{2, "2x"},
		{0.5, "1/2x"},
	}
	for _, tc := range cases {
		if got := formatDemoBaseSpeed(tc.speed); got != tc.want {
			t.Fatalf("formatDemoBaseSpeed(%v) = %q, want %q", tc.speed, got, tc.want)
		}
	}
}

func TestProgressClamped(t *testing.T) {
	bar := NewDemoBarRoot(&stubProvider{st: DemoBarState{Playback: true, Show: true, Progress: 2.0}}, make([]byte, 128*128), make([]byte, 768))
	if !bar.IsVisible() {
		t.Fatal("bar not visible with clamped progress > 1")
	}
}
