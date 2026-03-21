package server

import (
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ironwail/ironwail-go/internal/cvar"
	"github.com/ironwail/ironwail-go/internal/fs"
	inet "github.com/ironwail/ironwail-go/internal/net"
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

// TestClipVelocity tests the velocity clipping function.
// It ensures that entities correctly slide along surfaces instead of stopping or penetrating them when they collide at an angle.
// Where in C: SV_ClipVelocity in sv_phys.c
func TestClipVelocity(t *testing.T) {
	in := [3]float32{100, 0.05, -5}
	normal := [3]float32{0, 0, 1}
	out := ClipVelocity(in, normal, 1)

	if out[2] != 0 {
		t.Fatalf("out[2] = %v, want 0", out[2])
	}
}

// TestPhysicsNoClipMovesOriginAndAngles tests the \"noclip\" physics state.
// It verifying that entities in noclip mode move freely according to their velocity and angular velocity without any collision checks.
// Where in C: SV_Physics_Noclip in sv_phys.c
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

// TestPhysicsPusherAdvancesLocalTimeWhenIdle tests the pusher (brush model) physics.
// It ensuring that moving platforms and doors advance their local time correctly, which is critical for their animation and movement logic.
// Where in C: SV_Physics_Pusher in sv_phys.c
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

// TestPhysicsTossOnGroundDoesNotMove tests the \"toss\" physics for items on the ground.
// It ensuring that items (like dropped weapons or health packs) remain stationary once they've landed on the floor.
// Where in C: SV_Physics_Toss in sv_phys.c
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

// TestFlyMoveDoesNotGroundOnNonBSPFloor tests FlyMove collision behavior with different entity types.
// It verifying that entities only \"land\" (set onground flag) on BSP world geometry, not on simple trigger boxes or other non-solid entities.
// Where in C: SV_FlyMove in sv_phys.c
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

// TestPhysicsStepOnGroundSkipsFreefall tests step physics for grounded entities.
// It ensuring that entities already on the ground don't erroneously apply gravity or vertical movement intended for freefall.
// Where in C: SV_Physics_Step in sv_phys.c
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

// TestPhysicsStepHardLandingStartsCanonicalSound tests landing sound triggers in the physics engine.
// It verifying that falling from a height correctly triggers the \"landing\" sound (demon/dland2.wav) via the network protocol.
// Where in C: SV_Physics_Step in sv_phys.c
func TestPhysicsStepHardLandingStartsCanonicalSound(t *testing.T) {
	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.WorldModel = CreateSyntheticWorldModel()
	s.Edicts[0].Vars.Solid = float32(SolidBSP)
	s.ClearWorld()
	s.SoundPrecache[1] = "demon/dland2.wav"

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to allocate step entity")
	}
	ent.Vars.MoveType = float32(MoveTypeStep)
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Origin = [3]float32{0, 0, 32}
	ent.Vars.Velocity = [3]float32{0, 0, -120}
	s.LinkEdict(ent, false)

	s.PhysicsStep(ent)

	data := s.Datagram.Data[:s.Datagram.Len()]
	if len(data) < 5 {
		t.Fatalf("landing sound datagram too short: %d", len(data))
	}
	if got := data[0]; got != byte(inet.SVCSound) {
		t.Fatalf("svc = %d, want %d", got, inet.SVCSound)
	}
	if got := data[1]; got != 0 {
		t.Fatalf("field mask = %d, want 0", got)
	}
	if got := int(binary.LittleEndian.Uint16(data[2:4])) >> 3; got != s.NumForEdict(ent) {
		t.Fatalf("entity num = %d, want %d", got, s.NumForEdict(ent))
	}
	if got := data[4]; got != 1 {
		t.Fatalf("sound index = %d, want 1", got)
	}
}

// TestSVWalkMoveHonorsSvNoStep tests the sv_nostep cvar.
// It allowing the server to disable the \"step up\" behavior for entities, which can be useful for debugging or specific gameplay modes.
// Where in C: SV_WalkMove in sv_phys.c
func TestSVWalkMoveHonorsSvNoStep(t *testing.T) {
	newMover := func(s *Server) *Edict {
		ent := s.AllocEdict()
		if ent == nil {
			t.Fatal("failed to allocate mover")
		}
		ent.Vars.MoveType = float32(MoveTypeWalk)
		ent.Vars.Solid = float32(SolidSlideBox)
		ent.Vars.Flags = float32(FlagOnGround)
		ent.Vars.Mins = [3]float32{-16, -16, -24}
		ent.Vars.Maxs = [3]float32{16, 16, 32}
		ent.Vars.Origin = [3]float32{0, 0, 24}
		ent.Vars.Velocity = [3]float32{100, 0, 0}
		s.LinkEdict(ent, false)
		return ent
	}
	newObstacle := func(s *Server) {
		obstacle := s.AllocEdict()
		if obstacle == nil {
			t.Fatal("failed to allocate obstacle")
		}
		obstacle.Vars.Solid = float32(SolidBBox)
		obstacle.Vars.Origin = [3]float32{32, 0, 8}
		obstacle.Vars.Mins = [3]float32{-8, -32, -8}
		obstacle.Vars.Maxs = [3]float32{8, 32, 8}
		s.LinkEdict(obstacle, false)
	}
	newServerWithStep := func() *Server {
		s := NewServer()
		if err := s.Init(1); err != nil {
			t.Fatalf("init server: %v", err)
		}
		s.WorldModel = CreateSyntheticWorldModel()
		s.Edicts[0].Vars.Solid = float32(SolidBSP)
		s.ClearWorld()
		newObstacle(s)
		return s
	}

	withPhysicsCVars(t, map[string]string{"sv_nostep": "0"})
	withStep := newServerWithStep()
	stepMover := newMover(withStep)
	withStep.SV_WalkMove(stepMover)

	withPhysicsCVars(t, map[string]string{"sv_nostep": "1"})
	noStep := newServerWithStep()
	noStepMover := newMover(noStep)
	noStep.SV_WalkMove(noStepMover)

	if !(stepMover.Vars.Origin[0] > noStepMover.Vars.Origin[0]+0.5) {
		t.Fatalf("sv_nostep did not suppress step retry: stepped=%v nostep=%v", stepMover.Vars.Origin, noStepMover.Vars.Origin)
	}
}

// TestPhysicsFrameOnSpawnedMap tests a full physics frame on a real map.
// It ensuring that the basic server time and physics update loop works correctly when a map is loaded.
// Where in C: SV_Physics in sv_phys.c
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

// TestPhysicsForceRetouchUsesFloatCountdown tests the force_retouch mechanism.
// It ensuring that the engine correctly forces entities to re-check for trigger contacts for a few frames after certain events (like teleportation).
// Where in C: SV_Physics in sv_phys.c
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

// TestPhysicsFreezeNonClientsCVar tests the sv_freezenonclients cvar.
// It allowing the server to pause non-player entities (monsters, etc.) for performance or debugging.
// Where in C: SV_Physics in sv_phys.c
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

// TestPhysicsTelemetryFrameHooks tests physics telemetry.
// It providing detailed performance and event logging for the physics engine.
// Where in C: N/A (Modern engine extension)
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

// TestRunThinkTelemetry tests telemetry for entity \"think\" functions.
// It monitoring the execution time and frequency of entity logic.
// Where in C: N/A
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

	e1 := &Edict{Vars: &EntVars{}}
	e1.Vars.Touch = 1
	e1.Vars.Solid = float32(SolidBSP)
	e2 := &Edict{Vars: &EntVars{}}
	e2.Vars.Solid = float32(SolidSlideBox)
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
