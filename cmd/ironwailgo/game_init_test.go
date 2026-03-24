package main

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/renderer"
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

func TestCurrentRuntimeFOVUsesCanonicalFOVCVar(t *testing.T) {
	if cvar.Get("fov") == nil {
		cvar.Register("fov", "90", cvar.FlagArchive, "")
	}

	cvar.Set("fov", "110")
	t.Cleanup(func() {
		cvar.Set("fov", "90")
	})

	if got := currentRuntimeFOV(); got != 110 {
		t.Fatalf("currentRuntimeFOV() = %v, want 110", got)
	}
}

func TestCurrentRuntimeViewSizeUsesCanonicalViewsizeCVar(t *testing.T) {
	registerMirroredArchiveCvars("viewsize", "scr_viewsize", "100", "")

	cvar.Set("scr_viewsize", "100")
	cvar.Set("viewsize", "130")
	t.Cleanup(func() {
		cvar.Set("viewsize", "100")
		cvar.Set("scr_viewsize", "100")
	})

	if got := currentRuntimeViewSize(); got != 130 {
		t.Fatalf("currentRuntimeViewSize() = %v, want 130", got)
	}
	if got := cvar.FloatValue("scr_viewsize"); got != 130 {
		t.Fatalf("legacy scr_viewsize alias = %v, want 130", got)
	}
}

func TestCurrentRuntimeZoomFOVUsesCanonicalZoomFOVCVar(t *testing.T) {
	if cvar.Get("zoom_fov") == nil {
		cvar.Register("zoom_fov", "30", cvar.FlagArchive, "")
	}

	cvar.Set("zoom_fov", "55")
	t.Cleanup(func() {
		cvar.Set("zoom_fov", "30")
	})

	if got := currentRuntimeZoomFOV(); got != 55 {
		t.Fatalf("currentRuntimeZoomFOV() = %v, want 55", got)
	}
}

func TestCurrentRuntimeFOVAdaptUsesCanonicalFOVAdaptCVar(t *testing.T) {
	if cvar.Get("fov_adapt") == nil {
		cvar.Register("fov_adapt", "1", cvar.FlagArchive, "")
	}

	cvar.Set("fov_adapt", "0")
	t.Cleanup(func() {
		cvar.Set("fov_adapt", "1")
	})

	if got := currentRuntimeFOVAdapt(); got {
		t.Fatal("currentRuntimeFOVAdapt() = true, want false")
	}
}

func TestCurrentShowTurtlePrefersCanonicalShowturtleCVar(t *testing.T) {
	registerMirroredArchiveCvars("showturtle", "scr_showturtle", "0", "")

	cvar.Set("scr_showturtle", "0")
	cvar.Set("showturtle", "1")
	t.Cleanup(func() {
		cvar.Set("showturtle", "0")
		cvar.Set("scr_showturtle", "0")
	})

	if got := currentShowTurtle(); !got {
		t.Fatal("currentShowTurtle() = false, want true")
	}
	if got := cvar.BoolValue("scr_showturtle"); !got {
		t.Fatal("legacy scr_showturtle alias did not mirror canonical showturtle")
	}
}

func TestRegisterColorShiftPercentCvarsRegistersDefaults(t *testing.T) {
	t.Parallel()

	registry := cvar.NewCVarSystem()
	registerColorShiftPercentCvars(registry.Register)

	tests := []struct {
		name string
	}{
		{name: "gl_cshiftpercent"},
		{name: "gl_cshiftpercent_contents"},
		{name: "gl_cshiftpercent_damage"},
		{name: "gl_cshiftpercent_bonus"},
		{name: "gl_cshiftpercent_powerup"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			cv := registry.Get(tt.name)
			if cv == nil {
				t.Fatalf("%s should be registered", tt.name)
			}
			if cv.String != "100" {
				t.Fatalf("%s default = %q, want 100", tt.name, cv.String)
			}
			if cv.Flags&cvar.FlagArchive == 0 {
				t.Fatalf("%s should be archived", tt.name)
			}
		})
	}
}

func TestRendererRDynamicCVarName(t *testing.T) {
	if renderer.CvarRDynamic != "r_dynamic" {
		t.Fatalf("renderer.CvarRDynamic = %q, want %q", renderer.CvarRDynamic, "r_dynamic")
	}
}
