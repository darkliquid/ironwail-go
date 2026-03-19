package server

import (
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/qc"
)

func TestTouchLinksTelemetry(t *testing.T) {
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

	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"touchlinks begin",
		"touchlinks candidates=0",
		"touchlinks end",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in telemetry:\n%s", want, joined)
		}
	}
}

func TestTouchLinksSyncsQCChangesBackToGoEdicts(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 8)
	s.Areanodes = make([]AreaNode, AreaNodes)
	s.ClearWorld()
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	const callbackBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		self := int(vm.GInt(qc.OFSSelf))
		other := int(vm.GInt(qc.OFSOther))

		vm.SetEFloat(self, qc.EntFieldSolid, float32(SolidNot))

		newOrigin := [3]float32{128, 0, 0}
		otherMins := vm.EVector(other, qc.EntFieldMins)
		otherMaxs := vm.EVector(other, qc.EntFieldMaxs)
		vm.SetEVector(other, qc.EntFieldOrigin, newOrigin)
		vm.SetEVector(other, qc.EntFieldAbsMin, [3]float32{
			newOrigin[0] + otherMins[0],
			newOrigin[1] + otherMins[1],
			newOrigin[2] + otherMins[2],
		})
		vm.SetEVector(other, qc.EntFieldAbsMax, [3]float32{
			newOrigin[0] + otherMaxs[0],
			newOrigin[1] + otherMaxs[1],
			newOrigin[2] + otherMaxs[2],
		})
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("touch_callback"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(callbackBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(callbackBuiltinOfs, -1)

	lines := make([]string, 0, 16)
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
	debugTelemetryEnableCVar = cvar.Register("sv_debug_telemetry_test_touchlinks_sync", "1", cvar.FlagNone, "")
	t.Cleanup(func() {
		debugTelemetryEnableCVar = oldEnable
	})

	mover := s.AllocEdict()
	trigger := s.AllocEdict()
	if mover == nil || trigger == nil {
		t.Fatal("failed to allocate test edicts")
	}

	mover.Vars.Origin = [3]float32{}
	mover.Vars.Mins = [3]float32{-16, -16, -16}
	mover.Vars.Maxs = [3]float32{16, 16, 16}
	mover.Vars.Solid = float32(SolidBBox)
	s.LinkEdict(mover, false)

	trigger.Vars.Origin = [3]float32{}
	trigger.Vars.Mins = [3]float32{-8, -8, -8}
	trigger.Vars.Maxs = [3]float32{8, 8, 8}
	trigger.Vars.Solid = float32(SolidTrigger)
	trigger.Vars.Touch = 1
	s.LinkEdict(trigger, false)

	s.touchLinks(mover)

	if got := mover.Vars.Origin; got != [3]float32{128, 0, 0} {
		t.Fatalf("mover origin = %v", got)
	}
	if got := mover.Vars.AbsMin; got != [3]float32{112, -16, -16} {
		t.Fatalf("mover absmin = %v", got)
	}
	if got := mover.Vars.AbsMax; got != [3]float32{144, 16, 16} {
		t.Fatalf("mover absmax = %v", got)
	}
	if got := trigger.Vars.Solid; got != float32(SolidNot) {
		t.Fatalf("trigger solid = %v, want %v", got, float32(SolidNot))
	}
	if trigger.AreaPrev == nil {
		t.Fatalf("trigger unexpectedly unlinked after direct QC solid mutation")
	}

	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"touchlinks callback begin self=",
		"touchlinks callback end self=",
		"self_link=linked",
		"other_vel=(",
		"other_punch=(",
		"other_flags=",
		"other_origin=(128.0 0.0 0.0)",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in telemetry:\n%s", want, joined)
		}
	}
}
