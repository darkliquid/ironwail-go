package server

import (
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/qc"
)

func TestTouchLinksTelemetryAndReentrantSkip(t *testing.T) {
	s := NewServer()
	s.QCVM = qc.NewVM()
	s.Areanodes = make([]AreaNode, AreaNodes)
	s.ClearWorld()

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to alloc entity")
	}
	ent.Vars.Solid = float32(SolidBBox)

	lines := make([]string, 0, 4)
	s.DebugTelemetry = NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{
			Enabled:      true,
			EventMask:    debugEventMaskTrigger,
			EntityFilter: debugEntityFilter{all: true},
			SummaryMode:  0,
		}
	}, func(line string) {
		lines = append(lines, line)
	})
	oldEnable := debugTelemetryEnableCVar
	debugTelemetryEnableCVar = cvar.Register("sv_debug_telemetry_test_touchlinks", "1", cvar.FlagNone, "")
	t.Cleanup(func() {
		debugTelemetryEnableCVar = oldEnable
	})

	s.touchLinks(ent)

	inTouchLinks = true
	s.touchLinks(ent)
	inTouchLinks = false

	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"touchlinks begin",
		"touchlinks candidates=0",
		"touchlinks end",
		"touchlinks reentrant-skip",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in telemetry:\n%s", want, joined)
		}
	}
}
