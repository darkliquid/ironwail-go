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
	if trigger.AreaPrev != nil {
		t.Fatalf("trigger should be unlinked after direct QC solid mutation")
	}

	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"touchlinks callback begin self=",
		"touchlinks callback end self=",
		"self_link=unlinked",
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

func TestTouchLinksSyncsThirdPartyPusherChangesBackFromQCVM(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 8)
	s.Areanodes = make([]AreaNode, AreaNodes)
	s.ClearWorld()
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	var doorNum int
	const callbackBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		vm.SetEFloat(doorNum, qc.EntFieldNextThink, 0.5)
		vm.SetEInt(doorNum, qc.EntFieldThink, 7)
		vm.SetEFloat(doorNum, qc.EntFieldLTime, 0.25)
		vm.SetEVector(doorNum, qc.EntFieldOrigin, [3]float32{64, 0, 0})
		vm.SetEVector(doorNum, qc.EntFieldAbsMin, [3]float32{48, -16, -16})
		vm.SetEVector(doorNum, qc.EntFieldAbsMax, [3]float32{80, 16, 16})
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("touch_callback_mutates_door"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(callbackBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(callbackBuiltinOfs, -1)

	mover := s.AllocEdict()
	trigger := s.AllocEdict()
	door := s.AllocEdict()
	if mover == nil || trigger == nil || door == nil {
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

	door.Vars.MoveType = float32(MoveTypePush)
	door.Vars.Solid = float32(SolidBSP)
	door.Vars.Mins = [3]float32{-16, -16, -16}
	door.Vars.Maxs = [3]float32{16, 16, 16}
	s.LinkEdict(door, false)
	doorNum = s.NumForEdict(door)

	s.touchLinks(mover)

	if got := door.Vars.NextThink; got != 0.5 {
		t.Fatalf("door nextthink = %v, want 0.5", got)
	}
	if got := door.Vars.Think; got != 7 {
		t.Fatalf("door think = %v, want 7", got)
	}
	if got := door.Vars.LTime; got != 0.25 {
		t.Fatalf("door ltime = %v, want 0.25", got)
	}
	if got := door.Vars.Origin; got != [3]float32{64, 0, 0} {
		t.Fatalf("door origin = %v, want [64 0 0]", got)
	}
	if door.AreaPrev == nil || door.AreaNext == nil {
		t.Fatalf("door unexpectedly unlinked after third-party QC sync: prev=%p next=%p", door.AreaPrev, door.AreaNext)
	}
}

func TestTouchLinksSyncsOwnerPusherStateIntoQCBeforeCallback(t *testing.T) {
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
		owner := vm.EInt(self, qc.EntFieldOwner)
		if owner == 0 {
			return
		}
		if got := vm.EFloat(int(owner), qc.EntFieldNextThink); got != 0.5 {
			return
		}
		vm.SetEVector(int(owner), qc.EntFieldOrigin, [3]float32{128, 0, 0})
		vm.SetEVector(int(owner), qc.EntFieldAbsMin, [3]float32{112, -16, -16})
		vm.SetEVector(int(owner), qc.EntFieldAbsMax, [3]float32{144, 16, 16})
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("touch_callback_reads_owner"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(callbackBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(callbackBuiltinOfs, -1)

	mover := s.AllocEdict()
	trigger := s.AllocEdict()
	door := s.AllocEdict()
	if mover == nil || trigger == nil || door == nil {
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

	door.Vars.MoveType = float32(MoveTypePush)
	door.Vars.Solid = float32(SolidBSP)
	door.Vars.Mins = [3]float32{-16, -16, -16}
	door.Vars.Maxs = [3]float32{16, 16, 16}
	door.Vars.NextThink = 0.5
	s.LinkEdict(door, false)

	trigger.Vars.Owner = int32(s.NumForEdict(door))

	s.touchLinks(mover)

	if got := door.Vars.Origin; got != [3]float32{128, 0, 0} {
		t.Fatalf("door origin = %v, want [128 0 0]", got)
	}
}

func TestTouchLinksDoesNotClobberUnchangedPusherFromStaleQCVM(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 8)
	s.Areanodes = make([]AreaNode, AreaNodes)
	s.ClearWorld()
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("noop_touch"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{{Op: uint16(qc.OPDone)}}

	mover := s.AllocEdict()
	trigger := s.AllocEdict()
	door := s.AllocEdict()
	if mover == nil || trigger == nil || door == nil {
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

	door.Vars.MoveType = float32(MoveTypePush)
	door.Vars.Solid = float32(SolidBSP)
	door.Vars.Mins = [3]float32{-16, -16, -16}
	door.Vars.Maxs = [3]float32{16, 16, 16}
	door.Vars.Origin = [3]float32{64, 0, 0}
	door.Vars.LTime = 48.84
	door.Vars.NextThink = 51.238
	s.LinkEdict(door, false)

	doorNum := s.NumForEdict(door)
	vm.SetEVector(doorNum, qc.EntFieldOrigin, [3]float32{})
	vm.SetEFloat(doorNum, qc.EntFieldLTime, 0.1)
	vm.SetEFloat(doorNum, qc.EntFieldNextThink, 1.02)

	s.touchLinks(mover)

	if got := door.Vars.Origin; got != [3]float32{64, 0, 0} {
		t.Fatalf("door origin clobbered from stale QC state: got %v", got)
	}
	if got := door.Vars.LTime; got != 48.84 {
		t.Fatalf("door ltime clobbered from stale QC state: got %v", got)
	}
	if got := door.Vars.NextThink; got != 51.238 {
		t.Fatalf("door nextthink clobbered from stale QC state: got %v", got)
	}
}

func TestTouchLinksRestoresQCExecutionContextAfterCallback(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 8)
	s.Areanodes = make([]AreaNode, AreaNodes)
	s.ClearWorld()
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("touch_context_test"), FirstStatement: 0},
		{Name: vm.AllocString("outer_qc_func"), FirstStatement: 1},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPDone)},
		{Op: uint16(qc.OPDone)},
	}

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

	vm.SetGInt(qc.OFSSelf, 77)
	vm.SetGInt(qc.OFSOther, 88)
	vm.XFunction = &vm.Functions[2]
	vm.XFunctionIndex = 2

	s.touchLinks(mover)

	if got := vm.GInt(qc.OFSSelf); got != 77 {
		t.Fatalf("self after touchLinks = %d, want 77", got)
	}
	if got := vm.GInt(qc.OFSOther); got != 88 {
		t.Fatalf("other after touchLinks = %d, want 88", got)
	}
	if vm.XFunction != &vm.Functions[2] || vm.XFunctionIndex != 2 {
		t.Fatalf("qc context not restored: xfunction=%p idx=%d", vm.XFunction, vm.XFunctionIndex)
	}
}

func TestTouchLinksDeduplicatesTriggerCallbacksWithinPhysicsFrame(t *testing.T) {
	s := NewServer()
	vm := newServerTestVM(s, 8)
	s.Areanodes = make([]AreaNode, AreaNodes)
	s.ClearWorld()
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	callbacks := 0
	const callbackBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		callbacks++
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("touch_callback_count"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(callbackBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(callbackBuiltinOfs, -1)

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
	s.touchLinks(mover)

	if callbacks != 2 {
		t.Fatalf("callbacks after repeated same-frame touches = %d, want 2", callbacks)
	}

	s.touchLinks(mover)

	if callbacks != 3 {
		t.Fatalf("callbacks after third touch = %d, want 3", callbacks)
	}
}
