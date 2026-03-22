package main

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
)

func TestShouldWarnAboutGoGPUX11Keyboard(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name                  string
		goos                  string
		requestedInputBackend string
		waylandDisplay        string
		x11Display            string
		want                  bool
	}{
		{
			name:       "warns on linux x11 without override",
			goos:       "linux",
			x11Display: ":0",
			want:       true,
		},
		{
			name:           "skips when wayland is present",
			goos:           "linux",
			waylandDisplay: "wayland-0",
			x11Display:     ":0",
		},
		{
			name:                  "skips when sdl3 override requested",
			goos:                  "linux",
			requestedInputBackend: "sDl3",
			x11Display:            ":0",
		},
		{
			name:       "skips without x11 display",
			goos:       "linux",
			x11Display: "",
		},
		{
			name:       "skips on non-linux",
			goos:       "darwin",
			x11Display: ":0",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := shouldWarnAboutGoGPUX11Keyboard(tt.goos, tt.requestedInputBackend, tt.waylandDisplay, tt.x11Display)
			if got != tt.want {
				t.Fatalf("shouldWarnAboutGoGPUX11Keyboard(%q, %q, %q, %q) = %v, want %v", tt.goos, tt.requestedInputBackend, tt.waylandDisplay, tt.x11Display, got, tt.want)
			}
		})
	}
}

func TestGoGPUX11KeyboardHint(t *testing.T) {
	t.Parallel()

	if got := gogpuX11KeyboardHint(true); got != "set IW_INPUT_BACKEND=sdl3 for event-driven keyboard input on X11" {
		t.Fatalf("gogpuX11KeyboardHint(true) = %q", got)
	}

	if got := gogpuX11KeyboardHint(false); got != "rebuild with `mise run build-gogpu-sdl3` or run under Wayland for event-driven keyboard input" {
		t.Fatalf("gogpuX11KeyboardHint(false) = %q", got)
	}
}

func TestCurrentZoomSpeedUsesCanonicalZoomSpeedCVar(t *testing.T) {
	if cvar.Get("zoom_speed") == nil {
		cvar.Register("zoom_speed", "8", cvar.FlagArchive, "")
	}

	cvar.Set("zoom_speed", "12")
	t.Cleanup(func() {
		cvar.Set("zoom_speed", "8")
	})

	if got := currentZoomSpeed(); got != 12 {
		t.Fatalf("currentZoomSpeed() = %v, want 12", got)
	}
}
