package host

import (
	"strings"
	"testing"
)

type stubWindowSetter struct {
	calls []string
}

func (s *stubWindowSetter) Init() error   { return nil }
func (s *stubWindowSetter) UpdateScreen() {}
func (s *stubWindowSetter) Shutdown()     {}
func (s *stubWindowSetter) SetWindowTitle(t string) {
	s.calls = append(s.calls, t)
}

func TestComputeWindowTitleDefaults(t *testing.T) {
	if got := computeWindowTitle("", "", 1); got != DefaultWindowTitle {
		t.Fatalf("no level should yield default: %q", got)
	}
	if got := computeWindowTitle("", "e1m1", 1); !strings.Contains(got, "e1m1") {
		t.Fatalf("map-only title missing map: %q", got)
	}
	got := computeWindowTitle("The Slipgate Complex", "e1m1", 3)
	if !strings.Contains(got, "The Slipgate Complex") || !strings.Contains(got, "e1m1") || !strings.Contains(got, "skill 3") {
		t.Fatalf("missing fields: %q", got)
	}
}

func TestUpdateWindowTitleRateLimits(t *testing.T) {
	h := NewHost()
	setter := &stubWindowSetter{}
	subs := &Subsystems{Renderer: setter}

	// First call always fires.
	h.updateWindowTitle(subs, 1.0)
	if len(setter.calls) != 1 {
		t.Fatalf("first call should emit title, got %d", len(setter.calls))
	}
	// Immediate follow-up within interval should be suppressed even
	// if state changes (rate-limit protects the compositor).
	h.currentSkill = 2
	h.updateWindowTitle(subs, 0.001)
	if len(setter.calls) != 1 {
		t.Fatalf("rate limiter allowed a second call too soon: %d", len(setter.calls))
	}
	// After the interval elapses, a changed skill triggers a new title.
	h.updateWindowTitle(subs, updateTitleInterval+0.001)
	if len(setter.calls) != 2 {
		t.Fatalf("expected second emission after interval: %d", len(setter.calls))
	}
	// Same state again should not re-emit even after interval.
	h.updateWindowTitle(subs, updateTitleInterval+0.001)
	if len(setter.calls) != 2 {
		t.Fatalf("idempotent re-emit: %d", len(setter.calls))
	}
}

func TestUpdateWindowTitleSkipsWithoutSetter(t *testing.T) {
	h := NewHost()
	// Renderer that doesn't implement WindowTitleSetter should not
	// cause a panic or error.
	subs := &Subsystems{Renderer: &nullRenderer{}}
	h.updateWindowTitle(subs, 1.0)
	h.updateWindowTitle(nil, 1.0)
}

type nullRenderer struct{}

func (nullRenderer) Init() error   { return nil }
func (nullRenderer) UpdateScreen() {}
func (nullRenderer) Shutdown()     {}
