// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/qc"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
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
	ent.SetSolid(s, float32(SolidBBox))

	lines := make([]string, 0, 4)
	s.DebugTelemetry = NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{
			Enabled:      true,
			EventMask:    debugEventMaskTrigger,
			EntityFilter: debugEntityFilter{All: true},
			SummaryMode:  0,
		}
	}, func(line string) {
		lines = append(lines, line)
	})
	oldEnable := debugTelemetryEnableCVar
	debugTelemetryEnableCVar = s.CVar.Register("sv_debug_telemetry_test_touchlinks", "1", cvar.FlagNone, "")
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

		newOrigin := qtypes.Vec3{X: 128, Y: 0, Z: 0}
		otherMins := vm.EVector(other, qc.EntFieldMins)
		otherMaxs := vm.EVector(other, qc.EntFieldMaxs)
		vm.SetEVector(other, qc.EntFieldOrigin, newOrigin)
		vm.SetEVector(other, qc.EntFieldAbsMin, newOrigin.Add(otherMins))
		vm.SetEVector(other, qc.EntFieldAbsMax, newOrigin.Add(otherMaxs))
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
			EntityFilter: debugEntityFilter{All: true},
			SummaryMode:  0,
		}
	}, func(line string) {
		lines = append(lines, line)
	})
	oldEnable := debugTelemetryEnableCVar
	debugTelemetryEnableCVar = s.CVar.Register("sv_debug_telemetry_test_touchlinks_sync", "1", cvar.FlagNone, "")
	t.Cleanup(func() {
		debugTelemetryEnableCVar = oldEnable
	})

	mover := s.AllocEdict()
	trigger := s.AllocEdict()
	if mover == nil || trigger == nil {
		t.Fatal("failed to allocate test edicts")
	}

	mover.SetOrigin(s, qtypes.Vec3{})
	mover.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -16})
	mover.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 16})
	mover.SetSolid(s, float32(SolidBBox))
	s.LinkEdict(mover, false)

	trigger.SetOrigin(s, qtypes.Vec3{})
	trigger.SetMins(s, qtypes.Vec3{X: -8, Y: -8, Z: -8})
	trigger.SetMaxs(s, qtypes.Vec3{X: 8, Y: 8, Z: 8})
	trigger.SetSolid(s, float32(SolidTrigger))
	trigger.SetTouch(s, 1)
	s.LinkEdict(trigger, false)

	s.touchLinks(mover)

	if got := mover.Origin(s); got != (qtypes.Vec3{X: 128, Y: 0, Z: 0}) {
		t.Fatalf("mover origin = %v", got)
	}
	if got := mover.AbsMin(s); got != (qtypes.Vec3{X: 112, Y: -16, Z: -16}) {
		t.Fatalf("mover absmin = %v", got)
	}
	if got := mover.AbsMax(s); got != (qtypes.Vec3{X: 144, Y: 16, Z: 16}) {
		t.Fatalf("mover absmax = %v", got)
	}
	if got := trigger.Solid(s); got != float32(SolidNot) {
		t.Fatalf("trigger solid = %v, want %v", got, float32(SolidNot))
	}

	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"touchlinks callback begin self=",
		"touchlinks callback end self=",
		"self_link=linked",
		"other_vel=(",
		"other_punch=(",
		"other_flags=",
		"other_origin=(0.0 0.0 0.0)",
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
		vm.SetEVector(doorNum, qc.EntFieldOrigin, qtypes.Vec3{X: 64, Y: 0, Z: 0})
		vm.SetEVector(doorNum, qc.EntFieldAbsMin, qtypes.Vec3{X: 48, Y: -16, Z: -16})
		vm.SetEVector(doorNum, qc.EntFieldAbsMax, qtypes.Vec3{X: 80, Y: 16, Z: 16})
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

	mover.SetOrigin(s, qtypes.Vec3{})
	mover.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -16})
	mover.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 16})
	mover.SetSolid(s, float32(SolidBBox))
	s.LinkEdict(mover, false)

	trigger.SetOrigin(s, qtypes.Vec3{})
	trigger.SetMins(s, qtypes.Vec3{X: -8, Y: -8, Z: -8})
	trigger.SetMaxs(s, qtypes.Vec3{X: 8, Y: 8, Z: 8})
	trigger.SetSolid(s, float32(SolidTrigger))
	trigger.SetTouch(s, 1)
	s.LinkEdict(trigger, false)

	door.SetMoveType(s, float32(MoveTypePush))
	door.SetSolid(s, float32(SolidBSP))
	door.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -16})
	door.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 16})
	s.LinkEdict(door, false)
	doorNum = s.NumForEdict(door)

	s.touchLinks(mover)

	if got := door.NextThink(s); got != 0.5 {
		t.Fatalf("door nextthink = %v, want 0.5", got)
	}
	if got := door.Think(s); got != 7 {
		t.Fatalf("door think = %v, want 7", got)
	}
	if got := door.LTime(s); got != 0.25 {
		t.Fatalf("door ltime = %v, want 0.25", got)
	}
	if got := door.Origin(s); got != (qtypes.Vec3{X: 64, Y: 0, Z: 0}) {
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
		vm.SetEVector(int(owner), qc.EntFieldOrigin, qtypes.Vec3{X: 128, Y: 0, Z: 0})
		vm.SetEVector(int(owner), qc.EntFieldAbsMin, qtypes.Vec3{X: 112, Y: -16, Z: -16})
		vm.SetEVector(int(owner), qc.EntFieldAbsMax, qtypes.Vec3{X: 144, Y: 16, Z: 16})
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

	mover.SetOrigin(s, qtypes.Vec3{})
	mover.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -16})
	mover.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 16})
	mover.SetSolid(s, float32(SolidBBox))
	s.LinkEdict(mover, false)

	trigger.SetOrigin(s, qtypes.Vec3{})
	trigger.SetMins(s, qtypes.Vec3{X: -8, Y: -8, Z: -8})
	trigger.SetMaxs(s, qtypes.Vec3{X: 8, Y: 8, Z: 8})
	trigger.SetSolid(s, float32(SolidTrigger))
	trigger.SetTouch(s, 1)
	s.LinkEdict(trigger, false)

	door.SetMoveType(s, float32(MoveTypePush))
	door.SetSolid(s, float32(SolidBSP))
	door.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -16})
	door.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 16})
	door.SetNextThink(s, 0.5)
	s.LinkEdict(door, false)

	trigger.SetOwner(s, int32(s.NumForEdict(door)))

	s.touchLinks(mover)

	if got := door.Origin(s); got != (qtypes.Vec3{X: 128, Y: 0, Z: 0}) {
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

	mover.SetOrigin(s, qtypes.Vec3{})
	mover.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -16})
	mover.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 16})
	mover.SetSolid(s, float32(SolidBBox))
	s.LinkEdict(mover, false)

	trigger.SetOrigin(s, qtypes.Vec3{})
	trigger.SetMins(s, qtypes.Vec3{X: -8, Y: -8, Z: -8})
	trigger.SetMaxs(s, qtypes.Vec3{X: 8, Y: 8, Z: 8})
	trigger.SetSolid(s, float32(SolidTrigger))
	trigger.SetTouch(s, 1)
	s.LinkEdict(trigger, false)

	door.SetMoveType(s, float32(MoveTypePush))
	door.SetSolid(s, float32(SolidBSP))
	door.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -16})
	door.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 16})
	door.SetOrigin(s, qtypes.Vec3{X: 64, Y: 0, Z: 0})
	door.SetLTime(s, 48.84)
	door.SetNextThink(s, 51.238)
	s.touchLinks(mover)

	if got := door.Origin(s); got != (qtypes.Vec3{X: 64, Y: 0, Z: 0}) {
		t.Fatalf("door origin clobbered during touchLinks: got %v", got)
	}
	if got := door.LTime(s); got != 48.84 {
		t.Fatalf("door ltime clobbered during touchLinks: got %v", got)
	}
	if got := door.NextThink(s); got != 51.238 {
		t.Fatalf("door nextthink clobbered during touchLinks: got %v", got)
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

	mover.SetOrigin(s, qtypes.Vec3{})
	mover.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -16})
	mover.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 16})
	mover.SetSolid(s, float32(SolidBBox))
	s.LinkEdict(mover, false)

	trigger.SetOrigin(s, qtypes.Vec3{})
	trigger.SetMins(s, qtypes.Vec3{X: -8, Y: -8, Z: -8})
	trigger.SetMaxs(s, qtypes.Vec3{X: 8, Y: 8, Z: 8})
	trigger.SetSolid(s, float32(SolidTrigger))
	trigger.SetTouch(s, 1)
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

	mover.SetOrigin(s, qtypes.Vec3{})
	mover.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -16})
	mover.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 16})
	mover.SetSolid(s, float32(SolidBBox))
	s.LinkEdict(mover, false)

	trigger.SetOrigin(s, qtypes.Vec3{})
	trigger.SetMins(s, qtypes.Vec3{X: -8, Y: -8, Z: -8})
	trigger.SetMaxs(s, qtypes.Vec3{X: 8, Y: 8, Z: 8})
	trigger.SetSolid(s, float32(SolidTrigger))
	trigger.SetTouch(s, 1)
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
