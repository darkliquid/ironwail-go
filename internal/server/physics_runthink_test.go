package server

// RunThink, impact, and pusher sync tests split from physics_test.go.

import (
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/qc"
)

// TestRunThinkTelemetry tests telemetry for entity \"think\" functions.
// It monitoring the execution time and frequency of entity logic.
// Where in C: N/A
func TestRunThinkTelemetry(t *testing.T) {
	s := newPhysicsTestServer()

	ent := s.AllocEdict()
	ent.SetNextThink(s, 0.05)
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	lines := make([]string, 0, 2)
	s.DebugTelemetry = NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{
			Enabled:      true,
			EventMask:    debugEventMaskThink,
			EntityFilter: debugEntityFilter{all: true},
			SummaryMode:  0,
		}
	}, func(line string) {
		lines = append(lines, line)
	})
	oldEnable := debugTelemetryEnableCVar
	debugTelemetryEnableCVar = s.CVar.Register("sv_debug_telemetry_test_think", "1", cvar.FlagNone, "")
	t.Cleanup(func() {
		debugTelemetryEnableCVar = oldEnable
	})

	if ok := s.RunThink(ent); !ok {
		t.Fatal("RunThink unexpectedly returned false")
	}

	if len(lines) != 2 {
		t.Fatalf("got %d telemetry lines, want 2", len(lines))
	}
	if !strings.Contains(lines[0], "runthink begin") || !strings.Contains(lines[1], "runthink end") {
		t.Fatalf("unexpected telemetry lines: %#v", lines)
	}
}

// TestRunThinkPublishesQCTimeGlobal tests QuakeC global time synchronization.
// It ensuring that QuakeC scripts see the correct server time when their think functions are called.
// Where in C: SV_RunThink in sv_phys.c
func TestRunThinkPublishesQCTimeGlobal(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("test_think"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPDone)},
	}

	ent := s.AllocEdict()
	ent.SetNextThink(s, 0.05)
	ent.SetThink(s, 1)
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	if ok := s.RunThink(ent); !ok {
		t.Fatal("RunThink unexpectedly returned false")
	}
	if got := s.QCVM.GlobalFloat("time"); got != 0.05 {
		t.Fatalf("QC global time = %v, want 0.05", got)
	}
}

// TestRunThinkSyncsEdictStateBackFromQCVM tests entity state synchronization from QuakeC back to the engine.
// It allowing QuakeC to modify entity properties (like solid) and ensuring the engine's physics/collision state reflects those changes.
// Where in C: SV_RunThink in sv_phys.c
func TestRunThinkSyncsEdictStateBackFromQCVM(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	const mutateBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		self := int(vm.GInt(qc.OFSSelf))
		vm.SetEFloat(self, qc.EntFieldSolid, float32(SolidTrigger))
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("test_think"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(mutateBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(mutateBuiltinOfs, -1)

	ent := s.AllocEdict()
	ent.SetSolid(s, float32(SolidNot))
	ent.SetNextThink(s, 0.05)
	ent.SetThink(s, 1)
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	if ok := s.RunThink(ent); !ok {
		t.Fatal("RunThink unexpectedly returned false")
	}
	if got := ent.Solid(s); got != float32(SolidTrigger) {
		t.Fatalf("entity solid = %v, want %v after QC think", got, float32(SolidTrigger))
	}
}

// TestRunThinkSyncsThirdPartySchedulerFieldsFromQCVM tests cross-entity state synchronization.
// It ensuring that when one entity's think function modifies another entity's scheduling fields (like nextthink), the changes are correctly captured.
// Where in C: SV_RunThink in sv_phys.c
func TestRunThinkSyncsThirdPartySchedulerFieldsFromQCVM(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	var targetNum int
	const mutateBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		vm.SetEFloat(targetNum, qc.EntFieldFrame, 7)
		vm.SetEInt(targetNum, qc.EntFieldThink, 9)
		vm.SetEFloat(targetNum, qc.EntFieldNextThink, 1.25)
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("test_think_mutates_other_scheduler"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(mutateBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(mutateBuiltinOfs, -1)

	ent := s.AllocEdict()
	ent.SetNextThink(s, 0.05)
	ent.SetThink(s, 1)
	target := s.AllocEdict()
	s.Edicts = append(s.Edicts, ent, target)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts
	targetNum = s.NumForEdict(target)

	if ok := s.RunThink(ent); !ok {
		t.Fatal("RunThink unexpectedly returned false")
	}
	if got := target.Frame(s); got != 7 {
		t.Fatalf("target frame = %v, want 7", got)
	}
	if got := target.Think(s); got != 9 {
		t.Fatalf("target think = %v, want 9", got)
	}
	if got := target.NextThink(s); got != 1.25 {
		t.Fatalf("target nextthink = %v, want 1.25", got)
	}
}

// TestRunThinkSyncsThirdPartyCombatStateFromQCVM tests cross-entity combat state synchronization.
// It verifying that combat-related changes (health, enemy targets) made in QuakeC are reflected in the engine's entity state.
// Where in C: SV_RunThink in sv_phys.c
func TestRunThinkSyncsThirdPartyCombatStateFromQCVM(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	var targetNum int
	const mutateBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		vm.SetEFloat(targetNum, qc.EntFieldHealth, 12)
		vm.SetEInt(targetNum, qc.EntFieldEnemy, 1)
		vm.SetEFloat(targetNum, qc.EntFieldDeadFlag, 2)
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("test_think_mutates_other_combat_state"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(mutateBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(mutateBuiltinOfs, -1)

	ent := s.AllocEdict()
	ent.SetNextThink(s, 0.05)
	ent.SetThink(s, 1)
	target := s.AllocEdict()
	target.SetHealth(s, 100)
	s.Edicts = append(s.Edicts, ent, target)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts
	targetNum = s.NumForEdict(target)

	if ok := s.RunThink(ent); !ok {
		t.Fatal("RunThink unexpectedly returned false")
	}
	if got := target.Health(s); got != 12 {
		t.Fatalf("target health = %v, want 12", got)
	}
	if got := target.Enemy(s); got != 1 {
		t.Fatalf("target enemy = %v, want 1", got)
	}
	if got := target.DeadFlag(s); got != 2 {
		t.Fatalf("target deadflag = %v, want 2", got)
	}
}

// TestImpactSyncsMutatedTouchStateBackFromQCVM tests entity state synchronization after a \"touch\" event.
// It allowing QuakeC touch functions to modify entity states (e.g., picking up an item) and ensuring the engine is updated.
// Where in C: SV_Impact in sv_phys.c
func TestImpactSyncsMutatedTouchStateBackFromQCVM(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	const mutateBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		other := int(vm.GInt(qc.OFSOther))
		vm.SetEFloat(other, qc.EntFieldSolid, float32(SolidNot))
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("touch_mutates_other"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(mutateBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(mutateBuiltinOfs, -1)

	e1 := s.AllocEdict()
	e1.SetTouch(s, 1)
	e1.SetSolid(s, float32(SolidTrigger))
	e2 := s.AllocEdict()
	e2.SetSolid(s, float32(SolidBSP))
	s.Edicts = append(s.Edicts, e1, e2)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	s.Impact(e1, e2)

	if got := e2.Solid(s); got != float32(SolidNot) {
		t.Fatalf("other entity solid = %v, want %v after QC touch", got, float32(SolidNot))
	}
}

// TestImpactRestoresQCExecutionContextAfterTouch tests QuakeC VM state restoration.
// It ensuring that a touch callback (which might be triggered during another QuakeC function) correctly restores the VM state after execution.
// Where in C: SV_Impact in sv_phys.c
func TestImpactRestoresQCExecutionContextAfterTouch(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
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

	e1 := s.AllocEdict()
	e1.SetTouch(s, 1)
	e1.SetSolid(s, float32(SolidTrigger))
	e2 := s.AllocEdict()
	e2.SetSolid(s, float32(SolidBSP))
	s.Edicts = append(s.Edicts, e1, e2)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	vm.SetGInt(qc.OFSSelf, 77)
	vm.SetGInt(qc.OFSOther, 88)
	vm.XFunction = &vm.Functions[2]
	vm.XFunctionIndex = 2

	s.Impact(e1, e2)

	if got := vm.GInt(qc.OFSSelf); got != 77 {
		t.Fatalf("self after Impact = %d, want 77", got)
	}
	if got := vm.GInt(qc.OFSOther); got != 88 {
		t.Fatalf("other after Impact = %d, want 88", got)
	}
	if vm.XFunction != &vm.Functions[2] || vm.XFunctionIndex != 2 {
		t.Fatalf("qc context not restored: xfunction=%p idx=%d", vm.XFunction, vm.XFunctionIndex)
	}
}

// TestImpactDeduplicatesSameFrameTouchCallbacks tests touch callback deduplication.
// It preventing an entity from triggering multiple touch events in the same physics frame, which can cause logic errors (e.g., picking up an item twice).
// Where in C: SV_Impact in sv_phys.c
func TestImpactDeduplicatesSameFrameTouchCallbacks(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	callbacks := 0
	const countBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		callbacks++
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("door_touch"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(countBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(countBuiltinOfs, -1)

	e1 := s.AllocEdict()
	e1.SetTouch(s, 1)
	e1.SetSolid(s, float32(SolidBSP))
	e2 := s.AllocEdict()
	e2.SetSolid(s, float32(SolidSlideBox))
	s.Edicts = append(s.Edicts, e1, e2)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	s.Impact(e1, e2)
	s.Impact(e1, e2)

	if callbacks != 2 {
		t.Fatalf("same-frame impact callbacks = %d, want 2", callbacks)
	}

	s.Impact(e1, e2)

	if callbacks != 3 {
		t.Fatalf("next impact callbacks = %d, want 3", callbacks)
	}
}

// TestPhysicsPusherSyncsCurrentStateIntoQCBeforeThink tests state synchronization for pusher entities before their think function.
// It ensuring that moving platforms have their latest position and state available to QuakeC.
// Where in C: SV_Physics_Pusher in sv_phys.c
func TestPhysicsPusherSyncsCurrentStateIntoQCBeforeThink(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	const mutateBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		self := int(vm.GInt(qc.OFSSelf))
		vm.SetEFloat(self, qc.EntFieldSolid, float32(SolidTrigger))
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("pusher_think"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(mutateBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(mutateBuiltinOfs, -1)

	ent := &Edict{Num: 1, Vars: &EntVars{}}
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	ent.SetMoveType(s, float32(MoveTypePush))
	ent.SetLTime(s, 0)
	ent.SetNextThink(s, 0.05)
	ent.SetThink(s, 1)
	ent.SetSolid(s, float32(SolidNot))

	s.PhysicsPusher(ent)

	if got := ent.NextThink(s); got != 0 {
		t.Fatalf("nextthink = %v, want 0 after pusher think", got)
	}
	if got := ent.Solid(s); got != float32(SolidTrigger) {
		t.Fatalf("solid = %v, want %v after pusher think", got, float32(SolidTrigger))
	}
}

// TestPhysicsPusherSyncsThirdPartyPusherStateBackFromQCVM tests pusher state synchronization from QuakeC.
// It allowing QuakeC to control moving platforms and doors by modifying their velocity and think times.
// Where in C: SV_Physics_Pusher in sv_phys.c
func TestPhysicsPusherSyncsThirdPartyPusherStateBackFromQCVM(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	var targetNum int
	const mutateBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		vm.SetEVector(targetNum, qc.EntFieldVelocity, [3]float32{0, 100, 0})
		vm.SetEFloat(targetNum, qc.EntFieldNextThink, 0.5)
		vm.SetEInt(targetNum, qc.EntFieldThink, 7)
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("pusher_think_mutates_target"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(mutateBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(mutateBuiltinOfs, -1)

	e1 := &Edict{Num: 1, Vars: &EntVars{}}
	target := &Edict{Num: 2, Vars: &EntVars{}}
	s.Edicts = append(s.Edicts, e1, target)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	e1.SetMoveType(s, float32(MoveTypePush))
	e1.SetNextThink(s, 0.05)
	e1.SetThink(s, 1)
	target.SetMoveType(s, float32(MoveTypePush))
	targetNum = s.NumForEdict(target)

	s.PhysicsPusher(e1)

	if got := target.Velocity(s); got != [3]float32{0, 100, 0} {
		t.Fatalf("target velocity = %v, want [0 100 0]", got)
	}
	if got := target.NextThink(s); got != 0.5 {
		t.Fatalf("target nextthink = %v, want 0.5", got)
	}
	if got := target.Think(s); got != 7 {
		t.Fatalf("target think = %v, want 7", got)
	}
}

// TestPhysicsPusherSyncsNewTriggerSpawnedDuringThinkFromQCVM tests entity spawning during pusher execution.
// It ensuring that triggers or other entities spawned by a moving platform are correctly integrated into the physics world immediately.
// Where in C: SV_Physics_Pusher in sv_phys.c
func TestPhysicsPusherSyncsNewTriggerSpawnedDuringThinkFromQCVM(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.1
	s.ClearWorld()
	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)
	t.Cleanup(func() {
	})
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	var spawnedNum int
	const spawnBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		if fn := vm.Builtins[14]; fn == nil {
			t.Fatal("spawn builtin not registered")
		} else {
			fn(vm)
		}
		spawnedNum = int(vm.GInt(qc.OFSReturn))
		vm.SetEFloat(spawnedNum, qc.EntFieldSolid, float32(SolidTrigger))
		vm.SetEInt(spawnedNum, qc.EntFieldTouch, 99)
		vm.SetEVector(spawnedNum, qc.EntFieldOrigin, [3]float32{64, 0, 0})
		vm.SetEVector(spawnedNum, qc.EntFieldMins, [3]float32{-8, -8, -8})
		vm.SetEVector(spawnedNum, qc.EntFieldMaxs, [3]float32{8, 8, 8})
		vm.SetEVector(spawnedNum, qc.EntFieldSize, [3]float32{16, 16, 16})
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("pusher_think_spawns_trigger"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(spawnBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(spawnBuiltinOfs, -1)

	ent := &Edict{Num: 1, Vars: &EntVars{}}
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	ent.SetMoveType(s, float32(MoveTypePush))
	ent.SetNextThink(s, 0.05)
	ent.SetThink(s, 1)

	s.PhysicsPusher(ent)

	if spawnedNum <= 0 || spawnedNum >= s.NumEdicts {
		t.Fatalf("spawned edict num = %d, want valid new edict", spawnedNum)
	}
	spawned := s.EdictNum(spawnedNum)
	if spawned == nil {
		t.Fatal("spawned trigger missing")
	}
	if got := spawned.Solid(s); got != float32(SolidTrigger) {
		t.Fatalf("spawned solid = %v, want %v", got, float32(SolidTrigger))
	}
	if got := spawned.Touch(s); got != 99 {
		t.Fatalf("spawned touch = %v, want 99", got)
	}
	if spawned.AreaPrev == nil || spawned.AreaNext == nil {
		t.Fatalf("spawned trigger was not linked: prev=%p next=%p", spawned.AreaPrev, spawned.AreaNext)
	}
}

// TestPushMoveBlockedSyncsMutatedPusherFromQCVM tests blocked callback synchronization for pusher entities.
// It ensuring that pusher blocked callbacks can mutate pusher state in QC and have those mutations applied back to server state.
// Where in C: SV_PushMove in sv_phys.c
func TestPushMoveBlockedSyncsMutatedPusherFromQCVM(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.1
	s.WorldModel = CreateSyntheticWorldModel()
	if len(s.Edicts) == 0 || s.Edicts[0] == nil {
		t.Fatal("missing world edict")
	}
	s.Edicts[0].SetSolid(s, float32(SolidBSP))
	s.ClearWorld()

	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	const mutateBlockedBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		self := int(vm.GInt(qc.OFSSelf))
		vm.SetEVector(self, qc.EntFieldVelocity, [3]float32{0, 0, 200})
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("pusher_blocked_mutates_self"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(mutateBlockedBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(mutateBlockedBuiltinOfs, -1)

	pusher := s.AllocEdict()
	if pusher == nil {
		t.Fatal("failed to alloc pusher")
	}
	pusher.SetMoveType(s, float32(MoveTypePush))
	pusher.SetSolid(s, float32(SolidBSP))
	pusher.SetVelocity(s, [3]float32{0, 0, 64})
	pusher.SetOrigin(s, [3]float32{0, 0, 0})
	pusher.SetMins(s, [3]float32{-64, -64, -8})
	pusher.SetMaxs(s, [3]float32{64, 64, 8})
	pusher.SetBlocked(s, 1)
	s.LinkEdict(pusher, false)

	blocker := s.AllocEdict()
	if blocker == nil {
		t.Fatal("failed to alloc blocker")
	}
	blocker.SetMoveType(s, float32(MoveTypeWalk))
	blocker.SetSolid(s, float32(SolidSlideBox))
	blocker.SetOrigin(s, [3]float32{0, 0, 24})
	blocker.SetMins(s, [3]float32{-16, -16, -24})
	blocker.SetMaxs(s, [3]float32{16, 16, 32})
	s.LinkEdict(blocker, false)

	vm.NumEdicts = s.NumEdicts

	s.PushMove(pusher, s.FrameTime)

	if got := pusher.Velocity(s); got != [3]float32{0, 0, 200} {
		t.Fatalf("pusher velocity = %v, want [0 0 200] after blocked callback", got)
	}
}

// TestImpactDoesNotClobberExistingPusherStateFromStaleQCVM tests pusher state protection during touch events.
// It ensuring that a touch event (which uses the QuakeC VM) doesn't accidentally overwrite the state of unrelated moving platforms with stale data.
// Where in C: SV_Impact in sv_phys.c
func TestImpactDoesNotClobberExistingPusherStateFromStaleQCVM(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	const noopBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("touch_noop"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(noopBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(noopBuiltinOfs, -1)

	e1 := s.AllocEdict()
	e1.SetTouch(s, 1)
	e1.SetSolid(s, float32(SolidTrigger))
	e2 := s.AllocEdict()
	e2.SetSolid(s, float32(SolidBSP))
	pusher := s.AllocEdict()
	pusher.SetMoveType(s, float32(MoveTypePush))
	pusher.SetOrigin(s, [3]float32{32, 0, 0})
	pusher.SetLTime(s, 0.3)
	pusher.SetNextThink(s, 0.6)
	s.Edicts = append(s.Edicts, e1, e2, pusher)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	pusherNum := s.NumForEdict(pusher)
	vm.SetEVector(pusherNum, qc.EntFieldOrigin, [3]float32{})
	vm.SetEFloat(pusherNum, qc.EntFieldLTime, 0)
	vm.SetEFloat(pusherNum, qc.EntFieldNextThink, 0)

	s.Impact(e1, e2)

	if got := pusher.Origin(s); got != [3]float32{32, 0, 0} {
		t.Fatalf("pusher origin = %v, want [32 0 0]", got)
	}
	if got := pusher.LTime(s); got != 0.3 {
		t.Fatalf("pusher ltime = %v, want 0.3", got)
	}
	if got := pusher.NextThink(s); got != 0.6 {
		t.Fatalf("pusher nextthink = %v, want 0.6", got)
	}
}

// TestImpactSyncsPusherMutationsFromQCVM verifies that when a touch callback
// executed via Impact mutates a MOVETYPE_PUSH entity's fields (velocity,
// nextthink, think) in QCVM storage, those mutations are synced back to the
// Go server edict. Without this sync, a trigger that targets a pusher (e.g.,
// a button that activates a func_train) would have its effect silently lost
// when the touch dispatch path is through Impact (direct collision) rather
// than touchLinks (area trigger).
func TestImpactSyncsPusherMutationsFromQCVM(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	// The touch callback writes velocity, nextthink, and think on the pusher
	// entity (entity 3) using QCVM field offsets. This simulates a QC
	// function like button_fire → SUB_CalcMove that sets up movement on a
	// MOVETYPE_PUSH entity.
	const setPusherVelocityBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		pusherNum := 3
		vm.SetEVector(pusherNum, qc.EntFieldVelocity, [3]float32{0, 0, -100})
		vm.SetEFloat(pusherNum, qc.EntFieldNextThink, 1.5)
		vm.SetEInt(pusherNum, qc.EntFieldThink, 42)
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("touch_set_pusher_velocity"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(setPusherVelocityBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(setPusherVelocityBuiltinOfs, -1)

	e1 := s.AllocEdict()
	e1.SetTouch(s, 1)
	e1.SetSolid(s, float32(SolidTrigger))
	e2 := s.AllocEdict()
	e2.SetSolid(s, float32(SolidBSP))
	pusher := s.AllocEdict()
	pusher.SetMoveType(s, float32(MoveTypePush))
	pusher.SetOrigin(s, [3]float32{32, 0, 0})
	pusher.SetLTime(s, 0.3)
	s.Edicts = append(s.Edicts, e1, e2, pusher)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	// Initialize pusher QCVM fields to zero so mutations are detectable
	pusherNum := s.NumForEdict(pusher)
	vm.SetEVector(pusherNum, qc.EntFieldVelocity, [3]float32{0, 0, 0})
	vm.SetEFloat(pusherNum, qc.EntFieldNextThink, 0)
	vm.SetEInt(pusherNum, qc.EntFieldThink, 0)

	s.Impact(e1, e2)

	// After Impact, the pusher's mutated fields should be synced back to Go
	if got := pusher.Velocity(s); got != [3]float32{0, 0, -100} {
		t.Fatalf("pusher velocity = %v, want [0 0 -100]", got)
	}
	if got := pusher.NextThink(s); got != 1.5 {
		t.Fatalf("pusher nextthink = %v, want 1.5", got)
	}
	if got := pusher.Think(s); got != 42 {
		t.Fatalf("pusher think = %v, want 42", got)
	}
}

// TestExecuteQCFunctionSyncsPusherMutationsFromNonPusherThink verifies that
// when a non-pusher entity's think function (e.g. DelayedUse) calls
// SUB_UseTargets which targets a MOVETYPE_PUSH entity (e.g. func_train),
// the pusher's mutated fields (velocity, nextthink, think) are synced back
// to the Go server edict. Without this sync, the pusher would never move
// because the Go-side PhysicsPusher would never see the velocity/nextthink
// set by the QC callback.
func TestExecuteQCFunctionSyncsPusherMutationsFromNonPusherThink(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSSelf), Name: vm.AllocString("self")},
		{Type: uint16(qc.EvEntity), Ofs: uint16(qc.OFSOther), Name: vm.AllocString("other")},
		{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
	}

	// The think callback writes velocity, nextthink, and think on a pusher
	// entity (entity 2) using QCVM field offsets. This simulates a
	// DelayedUse think → SUB_UseTargets → train_use → train_next →
	// SUB_CalcMove that sets up movement on a MOVETYPE_PUSH entity.
	const setPusherVelocityBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		pusherNum := 2
		vm.SetEVector(pusherNum, qc.EntFieldVelocity, [3]float32{0, 0, -600})
		vm.SetEFloat(pusherNum, qc.EntFieldNextThink, 7.78)
		vm.SetEInt(pusherNum, qc.EntFieldThink, 99)
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("delayed_use_set_pusher_velocity"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(setPusherVelocityBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(setPusherVelocityBuiltinOfs, -1)

	// Entity 1: non-pusher (thinker, e.g. DelayedUse)
	thinker := s.AllocEdict()
	thinker.SetMoveType(s, float32(MoveTypeNone))
	thinker.SetThink(s, 1)
	thinker.SetNextThink(s, 0.05)

	// Entity 2: pusher (e.g. func_train)
	pusher := s.AllocEdict()
	pusher.SetMoveType(s, float32(MoveTypePush))
	pusher.SetOrigin(s, [3]float32{100, 200, 300})
	pusher.SetLTime(s, 0.1)

	s.Edicts = append(s.Edicts, thinker, pusher)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	// Initialize pusher QCVM fields to zero so mutations are detectable
	pusherNum := s.NumForEdict(pusher)
	vm.SetEVector(pusherNum, qc.EntFieldVelocity, [3]float32{0, 0, 0})
	vm.SetEFloat(pusherNum, qc.EntFieldNextThink, 0)
	vm.SetEInt(pusherNum, qc.EntFieldThink, 0)

	// Execute the thinker's think function via executeQCFunction (the same
	// path used by RunThink for non-pusher entities).
	thinkerNum := s.NumForEdict(thinker)
	s.QCVM.SetGlobal("self", thinkerNum)
	s.setQCTimeGlobal(1.0)
	err := s.executeQCFunction(int(thinker.Think(s)))
	if err != nil {
		t.Fatalf("executeQCFunction error: %v", err)
	}

	// After executeQCFunction, the pusher's mutated fields should be synced
	// back to Go so PhysicsPusher can move it on subsequent frames.
	if got := pusher.Velocity(s); got != [3]float32{0, 0, -600} {
		t.Fatalf("pusher velocity = %v, want [0 0 -600]", got)
	}
	if got := pusher.NextThink(s); got != 7.78 {
		t.Fatalf("pusher nextthink = %v, want 7.78", got)
	}
	if got := pusher.Think(s); got != 99 {
		t.Fatalf("pusher think = %v, want 99", got)
	}
}
