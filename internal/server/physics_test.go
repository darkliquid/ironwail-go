package server

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/qc"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

func newPhysicsTestServer() *Server {
	s := &Server{
		Gravity:     800,
		MaxVelocity: 2000,
		FrameTime:   0.1,
		Edicts:      []*Edict{{Vars: &EntVars{}}},
		NumEdicts:   1,
	}
	return s
}

func withPhysicsCVars(t *testing.T, values map[string]string) {
	t.Helper()
	original := make(map[string]string, len(values))
	for name := range values {
		if cvar.Get(name) == nil {
			cvar.Register(name, "0", cvar.FlagServerInfo, "")
		}
		original[name] = cvar.StringValue(name)
	}
	for name, value := range values {
		cvar.Set(name, value)
	}
	t.Cleanup(func() {
		for name, value := range original {
			cvar.Set(name, value)
		}
	})
}

func TestClipVelocity(t *testing.T) {
	in := [3]float32{100, 0.05, -5}
	normal := [3]float32{0, 0, 1}
	out := ClipVelocity(in, normal, 1)

	if out[2] != 0 {
		t.Fatalf("out[2] = %v, want 0", out[2])
	}
}

func TestPhysicsNoClipMovesOriginAndAngles(t *testing.T) {
	s := newPhysicsTestServer()
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Velocity = [3]float32{10, -5, 2}
	ent.Vars.AVelocity = [3]float32{0, 90, 0}
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	s.PhysicsNoClip(ent)

	if ent.Vars.Origin != [3]float32{1, -0.5, 0.2} {
		t.Fatalf("origin = %v", ent.Vars.Origin)
	}
	if ent.Vars.Angles != [3]float32{0, 9, 0} {
		t.Fatalf("angles = %v", ent.Vars.Angles)
	}
}

func TestPhysicsPusherAdvancesLocalTimeWhenIdle(t *testing.T) {
	s := newPhysicsTestServer()
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.MoveType = float32(MoveTypePush)
	ent.Vars.LTime = 3
	ent.Vars.NextThink = 10
	s.PhysicsPusher(ent)

	if diff := ent.Vars.LTime - 3.1; diff < -0.0001 || diff > 0.0001 {
		t.Fatalf("ltime = %v, want 3.1", ent.Vars.LTime)
	}
}

func TestPhysicsTossOnGroundDoesNotMove(t *testing.T) {
	s := newPhysicsTestServer()
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Flags = float32(FlagOnGround)
	ent.Vars.Origin = [3]float32{1, 2, 3}
	ent.Vars.Velocity = [3]float32{50, 60, 70}

	s.PhysicsToss(ent)

	if ent.Vars.Origin != [3]float32{1, 2, 3} {
		t.Fatalf("origin changed on ground toss: %v", ent.Vars.Origin)
	}
}

func TestFlyMoveDoesNotGroundOnNonBSPFloor(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.1
	s.WorldModel = CreateSyntheticWorldModel()
	if len(s.Edicts) == 0 || s.Edicts[0] == nil {
		t.Fatal("missing world edict")
	}
	s.Edicts[0].Vars.Solid = float32(SolidBSP)
	s.ClearWorld()

	platform := s.AllocEdict()
	if platform == nil {
		t.Fatal("failed to alloc platform")
	}
	platform.Vars.Origin = [3]float32{0, 0, 72}
	platform.Vars.Mins = [3]float32{-64, -64, -8}
	platform.Vars.Maxs = [3]float32{64, 64, 8}
	platform.Vars.Solid = float32(SolidBBox)
	s.LinkEdict(platform, false)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to alloc mover")
	}
	ent.Vars.Origin = [3]float32{0, 0, 112}
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.MoveType = float32(MoveTypeWalk)
	ent.Vars.Velocity = [3]float32{0, 0, -200}
	s.LinkEdict(ent, false)

	blocked := s.FlyMove(ent, s.FrameTime, nil)
	if blocked&1 == 0 {
		t.Fatalf("FlyMove blocked=%d, want floor contact bit set", blocked)
	}
	if uint32(ent.Vars.Flags)&FlagOnGround != 0 {
		t.Fatalf("Flags unexpectedly include onground after SolidBBox contact: flags=%#x", uint32(ent.Vars.Flags))
	}
	if ent.Vars.GroundEntity != 0 {
		t.Fatalf("ground entity = %d, want 0 for non-BSP contact", ent.Vars.GroundEntity)
	}
}

func TestPhysicsStepOnGroundSkipsFreefall(t *testing.T) {
	s := newPhysicsTestServer()
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Flags = float32(FlagOnGround)
	ent.Vars.Velocity = [3]float32{0, 0, 42}

	s.PhysicsStep(ent)

	if ent.Vars.Velocity[2] != 42 {
		t.Fatalf("z velocity changed: %v", ent.Vars.Velocity[2])
	}
}

func TestPhysicsFrameOnSpawnedMap(t *testing.T) {
	pak0Path := testutil.SkipIfNoPak0(t)
	baseDir := filepath.Dir(pak0Path)
	if filepath.Base(baseDir) == "id1" {
		baseDir = filepath.Dir(baseDir)
	}

	vfs := fs.NewFileSystem()
	if err := vfs.Init(baseDir, "id1"); err != nil {
		t.Fatalf("init filesystem: %v", err)
	}
	defer vfs.Close()

	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	if err := s.SpawnServer("start", vfs); err != nil {
		t.Fatalf("spawn server: %v", err)
	}

	before := s.Time
	s.Physics()
	if s.Time <= before {
		t.Fatalf("time did not advance: before=%v after=%v", before, s.Time)
	}
}

func TestPhysicsForceRetouchUsesFloatCountdown(t *testing.T) {
	s := NewServer()
	s.Areanodes = make([]AreaNode, AreaNodes)
	s.ClearWorld()

	vm := newServerTestVM(s, 8)
	vm.GlobalDefs = append(vm.GlobalDefs,
		qc.DDef{Type: uint16(qc.EvFloat), Ofs: uint16(qc.OFSTime), Name: vm.AllocString("time")},
		qc.DDef{Type: uint16(qc.EvFloat), Ofs: 90, Name: vm.AllocString("force_retouch")},
	)

	triggerCalls := 0
	const callbackBuiltinOfs = 10
	vm.Builtins[1] = func(vm *qc.VM) {
		triggerCalls++
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

	mover := s.AllocEdict()
	trigger := s.AllocEdict()
	if mover == nil || trigger == nil {
		t.Fatal("failed to allocate edicts")
	}

	mover.Vars.Origin = [3]float32{}
	mover.Vars.Mins = [3]float32{-16, -16, -16}
	mover.Vars.Maxs = [3]float32{16, 16, 16}
	mover.Vars.Solid = float32(SolidBBox)
	mover.Vars.MoveType = float32(MoveTypeNone)
	s.LinkEdict(mover, false)

	trigger.Vars.Origin = [3]float32{}
	trigger.Vars.Mins = [3]float32{-8, -8, -8}
	trigger.Vars.Maxs = [3]float32{8, 8, 8}
	trigger.Vars.Solid = float32(SolidTrigger)
	trigger.Vars.Touch = 1
	trigger.Vars.MoveType = float32(MoveTypeNone)
	s.LinkEdict(trigger, false)

	vm.SetGlobal("force_retouch", float32(2))

	s.Physics()
	if got := vm.GetGlobalFloat("force_retouch"); got != 1 {
		t.Fatalf("force_retouch after first frame = %v, want 1", got)
	}
	firstCalls := triggerCalls
	if firstCalls == 0 {
		t.Fatal("force_retouch frame 1 did not trigger callback")
	}

	s.Physics()
	if got := vm.GetGlobalFloat("force_retouch"); got != 0 {
		t.Fatalf("force_retouch after second frame = %v, want 0", got)
	}
	secondCalls := triggerCalls
	if secondCalls <= firstCalls {
		t.Fatalf("force_retouch frame 2 did not trigger additional callback: first=%d second=%d", firstCalls, secondCalls)
	}

	s.Physics()
	if got := vm.GetGlobalFloat("force_retouch"); got != 0 {
		t.Fatalf("force_retouch after third frame = %v, want 0", got)
	}
	if triggerCalls != secondCalls {
		t.Fatalf("force_retouch kept triggering after countdown expired: before=%d after=%d", secondCalls, triggerCalls)
	}
}

func TestPhysicsFreezeNonClientsCVar(t *testing.T) {
	mkServer := func() (*Server, *Edict, *Edict) {
		s := newPhysicsTestServer()
		s.Static = &ServerStatic{MaxClients: 1}
		clientEnt := &Edict{Vars: &EntVars{}}
		clientEnt.Vars.MoveType = float32(MoveTypeNoClip)
		clientEnt.Vars.Velocity = [3]float32{10, 0, 0}
		nonClientEnt := &Edict{Vars: &EntVars{}}
		nonClientEnt.Vars.MoveType = float32(MoveTypeNoClip)
		nonClientEnt.Vars.Velocity = [3]float32{20, 0, 0}
		s.Edicts = append(s.Edicts, clientEnt, nonClientEnt)
		s.NumEdicts = len(s.Edicts)
		return s, clientEnt, nonClientEnt
	}

	t.Run("freeze enabled skips non-clients", func(t *testing.T) {
		withPhysicsCVars(t, map[string]string{"sv_freezenonclients": "1"})
		s, clientEnt, nonClientEnt := mkServer()

		s.Physics()

		if clientEnt.Vars.Origin[0] == 0 {
			t.Fatalf("client entity did not move with freeze enabled: origin=%v", clientEnt.Vars.Origin)
		}
		if nonClientEnt.Vars.Origin[0] != 0 {
			t.Fatalf("non-client entity moved with freeze enabled: origin=%v", nonClientEnt.Vars.Origin)
		}
	})

	t.Run("freeze disabled updates all entities", func(t *testing.T) {
		withPhysicsCVars(t, map[string]string{"sv_freezenonclients": "0"})
		s, clientEnt, nonClientEnt := mkServer()

		s.Physics()

		if clientEnt.Vars.Origin[0] == 0 {
			t.Fatalf("client entity did not move with freeze disabled: origin=%v", clientEnt.Vars.Origin)
		}
		if nonClientEnt.Vars.Origin[0] == 0 {
			t.Fatalf("non-client entity did not move with freeze disabled: origin=%v", nonClientEnt.Vars.Origin)
		}
	})
}

func TestPhysicsTelemetryFrameHooks(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.MoveType = float32(MoveTypeNoClip)
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	lines := make([]string, 0, 4)
	s.DebugTelemetry = NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{
			Enabled:      true,
			EventMask:    debugEventMaskFrame,
			EntityFilter: debugEntityFilter{all: true},
			SummaryMode:  2,
		}
	}, func(line string) {
		lines = append(lines, line)
	})
	oldEnable := debugTelemetryEnableCVar
	debugTelemetryEnableCVar = cvar.Register("sv_debug_telemetry_test_frame", "1", cvar.FlagNone, "")
	t.Cleanup(func() {
		debugTelemetryEnableCVar = oldEnable
	})

	s.Physics()

	joined := strings.Join(lines, "\n")
	for _, want := range []string{
		"kind=frame",
		"physics begin",
		"physics end",
		"summary total=2 qc=0",
		"counts=frame=2",
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("telemetry output missing %q in:\n%s", want, joined)
		}
	}
}

func TestRunThinkTelemetry(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.NextThink = 0.05
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
	debugTelemetryEnableCVar = cvar.Register("sv_debug_telemetry_test_think", "1", cvar.FlagNone, "")
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

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.NextThink = 0.05
	ent.Vars.Think = 1
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	if ok := s.RunThink(ent); !ok {
		t.Fatal("RunThink unexpectedly returned false")
	}
	if got := s.QCVM.GetGlobalFloat("time"); got != 0.05 {
		t.Fatalf("QC global time = %v, want 0.05", got)
	}
}

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

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Solid = float32(SolidNot)
	ent.Vars.NextThink = 0.05
	ent.Vars.Think = 1
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	if ok := s.RunThink(ent); !ok {
		t.Fatal("RunThink unexpectedly returned false")
	}
	if got := ent.Vars.Solid; got != float32(SolidTrigger) {
		t.Fatalf("entity solid = %v, want %v after QC think", got, float32(SolidTrigger))
	}
}

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

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.NextThink = 0.05
	ent.Vars.Think = 1
	target := &Edict{Vars: &EntVars{}}
	s.Edicts = append(s.Edicts, ent, target)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts
	targetNum = s.NumForEdict(target)

	if ok := s.RunThink(ent); !ok {
		t.Fatal("RunThink unexpectedly returned false")
	}
	if got := target.Vars.Frame; got != 7 {
		t.Fatalf("target frame = %v, want 7", got)
	}
	if got := target.Vars.Think; got != 9 {
		t.Fatalf("target think = %v, want 9", got)
	}
	if got := target.Vars.NextThink; got != 1.25 {
		t.Fatalf("target nextthink = %v, want 1.25", got)
	}
}

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

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.NextThink = 0.05
	ent.Vars.Think = 1
	target := &Edict{Vars: &EntVars{}}
	target.Vars.Health = 100
	s.Edicts = append(s.Edicts, ent, target)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts
	targetNum = s.NumForEdict(target)

	if ok := s.RunThink(ent); !ok {
		t.Fatal("RunThink unexpectedly returned false")
	}
	if got := target.Vars.Health; got != 12 {
		t.Fatalf("target health = %v, want 12", got)
	}
	if got := target.Vars.Enemy; got != 1 {
		t.Fatalf("target enemy = %v, want 1", got)
	}
	if got := target.Vars.DeadFlag; got != 2 {
		t.Fatalf("target deadflag = %v, want 2", got)
	}
}

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

	e1 := &Edict{Vars: &EntVars{}}
	e1.Vars.Touch = 1
	e1.Vars.Solid = float32(SolidTrigger)
	e2 := &Edict{Vars: &EntVars{}}
	e2.Vars.Solid = float32(SolidBSP)
	s.Edicts = append(s.Edicts, e1, e2)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	s.Impact(e1, e2)

	if got := e2.Vars.Solid; got != float32(SolidNot) {
		t.Fatalf("other entity solid = %v, want %v after QC touch", got, float32(SolidNot))
	}
}

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

	e1 := &Edict{Vars: &EntVars{}}
	e1.Vars.Touch = 1
	e1.Vars.Solid = float32(SolidTrigger)
	e2 := &Edict{Vars: &EntVars{}}
	e2.Vars.Solid = float32(SolidBSP)
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

	e1 := &Edict{Vars: &EntVars{}}
	e1.Vars.Touch = 1
	e1.Vars.Solid = float32(SolidBSP)
	e2 := &Edict{Vars: &EntVars{}}
	e2.Vars.Solid = float32(SolidSlideBox)
	s.Edicts = append(s.Edicts, e1, e2)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	s.touchFrameActive = true
	clear(s.impactFrameSeen)
	s.Impact(e1, e2)
	s.Impact(e1, e2)
	s.touchFrameActive = false

	if callbacks != 1 {
		t.Fatalf("same-frame impact callbacks = %d, want 1", callbacks)
	}

	clear(s.impactFrameSeen)
	s.touchFrameActive = true
	s.Impact(e1, e2)
	s.touchFrameActive = false

	if callbacks != 2 {
		t.Fatalf("next-frame impact callbacks = %d, want 2", callbacks)
	}
}

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

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.MoveType = float32(MoveTypePush)
	ent.Vars.LTime = 0
	ent.Vars.NextThink = 0.05
	ent.Vars.Think = 1
	ent.Vars.Solid = float32(SolidNot)
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	entNum := s.NumForEdict(ent)
	vm.SetEFloat(entNum, qc.EntFieldNextThink, 123)
	vm.SetEInt(entNum, qc.EntFieldThink, 1)

	s.PhysicsPusher(ent)

	if got := ent.Vars.NextThink; got != 0 {
		t.Fatalf("nextthink = %v, want 0 after pusher think", got)
	}
	if got := ent.Vars.Solid; got != float32(SolidTrigger) {
		t.Fatalf("solid = %v, want %v after pusher think", got, float32(SolidTrigger))
	}
}

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

	e1 := &Edict{Vars: &EntVars{}}
	e1.Vars.MoveType = float32(MoveTypePush)
	e1.Vars.NextThink = 0.05
	e1.Vars.Think = 1
	target := &Edict{Vars: &EntVars{}}
	target.Vars.MoveType = float32(MoveTypePush)
	s.Edicts = append(s.Edicts, e1, target)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts
	targetNum = s.NumForEdict(target)

	s.PhysicsPusher(e1)

	if got := target.Vars.Velocity; got != [3]float32{0, 100, 0} {
		t.Fatalf("target velocity = %v, want [0 100 0]", got)
	}
	if got := target.Vars.NextThink; got != 0.5 {
		t.Fatalf("target nextthink = %v, want 0.5", got)
	}
	if got := target.Vars.Think; got != 7 {
		t.Fatalf("target think = %v, want 7", got)
	}
}

func TestPhysicsPusherSyncsNewTriggerSpawnedDuringThinkFromQCVM(t *testing.T) {
	s := NewServer()
	s.FrameTime = 0.1
	s.ClearWorld()
	vm := newServerTestVM(s, 8)
	qc.RegisterBuiltins(vm)
	t.Cleanup(func() {
		qc.RegisterServerHooks(nil)
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

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.MoveType = float32(MoveTypePush)
	ent.Vars.NextThink = 0.05
	ent.Vars.Think = 1
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	s.PhysicsPusher(ent)

	if spawnedNum <= 0 || spawnedNum >= s.NumEdicts {
		t.Fatalf("spawned edict num = %d, want valid new edict", spawnedNum)
	}
	spawned := s.EdictNum(spawnedNum)
	if spawned == nil || spawned.Vars == nil {
		t.Fatal("spawned trigger missing")
	}
	if got := spawned.Vars.Solid; got != float32(SolidTrigger) {
		t.Fatalf("spawned solid = %v, want %v", got, float32(SolidTrigger))
	}
	if got := spawned.Vars.Touch; got != 99 {
		t.Fatalf("spawned touch = %v, want 99", got)
	}
	if spawned.AreaPrev == nil || spawned.AreaNext == nil {
		t.Fatalf("spawned trigger was not linked: prev=%p next=%p", spawned.AreaPrev, spawned.AreaNext)
	}
}

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

	e1 := &Edict{Vars: &EntVars{}}
	e1.Vars.Touch = 1
	e1.Vars.Solid = float32(SolidTrigger)
	e2 := &Edict{Vars: &EntVars{}}
	e2.Vars.Solid = float32(SolidBSP)
	pusher := &Edict{Vars: &EntVars{}}
	pusher.Vars.MoveType = float32(MoveTypePush)
	pusher.Vars.Origin = [3]float32{32, 0, 0}
	pusher.Vars.LTime = 0.3
	pusher.Vars.NextThink = 0.6
	s.Edicts = append(s.Edicts, e1, e2, pusher)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	pusherNum := s.NumForEdict(pusher)
	vm.SetEVector(pusherNum, qc.EntFieldOrigin, [3]float32{})
	vm.SetEFloat(pusherNum, qc.EntFieldLTime, 0)
	vm.SetEFloat(pusherNum, qc.EntFieldNextThink, 0)

	s.Impact(e1, e2)

	if got := pusher.Vars.Origin; got != [3]float32{32, 0, 0} {
		t.Fatalf("pusher origin = %v, want [32 0 0]", got)
	}
	if got := pusher.Vars.LTime; got != 0.3 {
		t.Fatalf("pusher ltime = %v, want 0.3", got)
	}
	if got := pusher.Vars.NextThink; got != 0.6 {
		t.Fatalf("pusher nextthink = %v, want 0.6", got)
	}
}
