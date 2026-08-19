package quakui

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
)

// TestUIBackendFailOpenBeforeRegistration asserts the gate reads as legacy
// (fail-open, ADR-0001) when the ui_backend cvar does not exist yet.
func TestUIBackendFailOpenBeforeRegistration(t *testing.T) {
	cvs := cvar.NewCVarSystem()

	if got := UIBackend(cvs); got != 0 {
		t.Fatalf("UIBackend before registration = %d, want 0 (fail-open)", got)
	}
	if IsGogpuUIPath(cvs) {
		t.Fatal("IsGogpuUIPath = true before registration, want false (fail-open)")
	}
}

// TestUIBackendDefaultsToLegacy asserts the registered default (0) selects
// the legacy path (ADR-0001).
func TestUIBackendDefaultsToLegacy(t *testing.T) {
	cvs := cvar.NewCVarSystem()
	cvs.Register(CvarUIBackend, "0", 0, "UI render backend (0=legacy, 1=gogpu/ui)")

	if got := UIBackend(cvs); got != 0 {
		t.Fatalf("UIBackend = %d, want 0", got)
	}
	if IsGogpuUIPath(cvs) {
		t.Fatal("IsGogpuUIPath = true with default 0, want false")
	}
}

// TestUIBackendToggleSwitchesPath asserts a mid-session ui_backend toggle
// flips the selected path (spec §1.4 #4).
func TestUIBackendToggleSwitchesPath(t *testing.T) {
	cvs := cvar.NewCVarSystem()
	cvs.Register(CvarUIBackend, "0", 0, "UI render backend (0=legacy, 1=gogpu/ui)")

	cvs.Set(CvarUIBackend, "1")
	if !IsGogpuUIPath(cvs) {
		t.Fatal("IsGogpuUIPath = false with 1, want true")
	}
	if got := UIBackend(cvs); got != 1 {
		t.Fatalf("UIBackend = %d, want 1", got)
	}

	cvs.Set(CvarUIBackend, "0")
	if IsGogpuUIPath(cvs) {
		t.Fatal("IsGogpuUIPath = true after toggle back to 0, want false")
	}
}
