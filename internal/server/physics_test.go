// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"encoding/binary"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/cvar"
	"github.com/darkliquid/ironwail-go/internal/fs"
	inet "github.com/darkliquid/ironwail-go/internal/net"
	"github.com/darkliquid/ironwail-go/internal/qc"
	"github.com/darkliquid/ironwail-go/internal/testutil"
)

func newPhysicsTestServer() *Server {
	s := &Server{
		Gravity:      800,
		MaxVelocity:  2000,
		FrameTime:    0.1,
		CVar:         cvar.NewCVarSystem(),
		QCFieldAlpha: -1,
		QCFieldScale: -1,
		Static:       &ServerStatic{MaxClients: 1},
	}
	s.QCVM = newServerTestVM(s, 64)
	s.MaxEdicts = 64
	s.Edicts = []*Edict{{Num: 0}}
	s.NumEdicts = 1
	s.ReliableDatagram = NewMessageBuffer(1024)
	s.Signon = NewMessageBuffer(1024)
	s.Datagram = NewMessageBuffer(1024)
	return s
}

func allocPhysicsTestEdict(s *Server) *Edict {
	num := len(s.Edicts)
	ent := &Edict{Num: num}
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)
	if s.QCVM != nil {
		s.QCVM.NumEdicts = s.NumEdicts
	}
	return ent
}

func withPhysicsCVars(t *testing.T, s *Server, values map[string]string) {
	t.Helper()
	for name, value := range values {
		if s.CVar.Get(name) == nil {
			s.CVar.Register(name, "0", cvar.FlagServerInfo, "")
		}
		s.CVar.Set(name, value)
	}
}

func newPushMoveElevatorTestServer(t *testing.T) (*Server, *Edict, *Edict) {
	t.Helper()
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.WorldModel = CreateSyntheticWorldModel()
	s.Edicts[0].SetSolid(s, float32(SolidBSP))
	s.ClearWorld()
	s.FrameTime = 0.1

	rider := s.EdictNum(1) // preallocated client edict
	if rider == nil {
		t.Fatal("missing client edict 1")
	}
	rider.SetMoveType(s, float32(MoveTypeWalk))
	rider.SetSolid(s, float32(SolidSlideBox))
	rider.SetMins(s, [3]float32{-16, -16, -24})
	rider.SetMaxs(s, [3]float32{16, 16, 32})
	rider.SetOrigin(s, [3]float32{0, 0, 31.99})

	pusher := s.AllocEdict() // edict 2+: non-client pusher
	if pusher == nil {
		t.Fatal("failed to allocate pusher")
	}
	pusher.SetMoveType(s, float32(MoveTypePush))
	pusher.SetSolid(s, float32(SolidBSP))
	pusher.SetMins(s, [3]float32{-32, -32, -8})
	pusher.SetMaxs(s, [3]float32{32, 32, 8})
	pusher.SetOrigin(s, [3]float32{0, 0, 0})
	pusher.SetVelocity(s, [3]float32{0, 0, 10})

	s.LinkEdict(pusher, false)
	rider.SetFlags(s, float32(uint32(rider.Flags(s))|FlagOnGround))
	rider.SetGroundEntity(s, int32(s.NumForEdict(pusher)))
	s.LinkEdict(rider, false)

	return s, pusher, rider
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
	ent := allocPhysicsTestEdict(s)
	ent.SetVelocity(s, [3]float32{10, -5, 2})
	ent.SetAVelocity(s, [3]float32{0, 90, 0})

	s.PhysicsNoClip(ent)

	if got := ent.Origin(s); got != [3]float32{1, -0.5, 0.2} {
		t.Fatalf("origin = %v", got)
	}
	if got := ent.Angles(s); got != [3]float32{0, 9, 0} {
		t.Fatalf("angles = %v", got)
	}
}

// TestPhysicsPusherAdvancesLocalTimeWhenIdle tests the pusher (brush model) physics.
// It ensuring that moving platforms and doors advance their local time correctly, which is critical for their animation and movement logic.
// Where in C: SV_Physics_Pusher in sv_phys.c
func TestPhysicsPusherAdvancesLocalTimeWhenIdle(t *testing.T) {
	s := newPhysicsTestServer()
	ent := allocPhysicsTestEdict(s)
	ent.SetMoveType(s, float32(MoveTypePush))
	ent.SetLTime(s, 3)
	ent.SetNextThink(s, 10)
	s.PhysicsPusher(ent)

	if diff := ent.LTime(s) - 3.1; diff < -0.0001 || diff > 0.0001 {
		t.Fatalf("ltime = %v, want 3.1", ent.LTime(s))
	}
}

func TestPushMoveElevatorFixOffRevertsBlockedMove(t *testing.T) {
	s, pusher, rider := newPushMoveElevatorTestServer(t)
	withPhysicsCVars(t, s, map[string]string{"sv_gameplayfix_elevators": "0"})

	startPusher := pusher.Origin(s)
	startRider := rider.Origin(s)

	s.PushMove(pusher, s.FrameTime)

	if pusher.Origin(s) != startPusher {
		t.Fatalf("pusher moved with fix disabled: got=%v want=%v", pusher.Origin(s), startPusher)
	}
	if rider.Origin(s) != startRider {
		t.Fatalf("rider moved with fix disabled: got=%v want=%v", rider.Origin(s), startRider)
	}
}

func TestPushMoveElevatorFixNudgesClientWhenEnabled(t *testing.T) {
	s, pusher, rider := newPushMoveElevatorTestServer(t)
	withPhysicsCVars(t, s, map[string]string{"sv_gameplayfix_elevators": "1"})

	startPusher := pusher.Origin(s)
	startRider := rider.Origin(s)

	s.PushMove(pusher, s.FrameTime)

	if !(pusher.Origin(s)[2] > startPusher[2]) {
		t.Fatalf("pusher did not advance with fix enabled: start=%v got=%v", startPusher, pusher.Origin(s))
	}
	wantRiderZ := startRider[2] + pusher.Velocity(s)[2]*s.FrameTime + DistEpsilon
	if diff := rider.Origin(s)[2] - wantRiderZ; diff < -0.001 || diff > 0.001 {
		t.Fatalf("rider z = %.5f, want %.5f (move + DistEpsilon nudge)", rider.Origin(s)[2], wantRiderZ)
	}
}

// TestPhysicsTossOnGroundDoesNotMove tests the \"toss\" physics for items on the ground.
// It ensuring that items (like dropped weapons or health packs) remain stationary once they've landed on the floor.
// Where in C: SV_Physics_Toss in sv_phys.c
func TestPhysicsTossOnGroundDoesNotMove(t *testing.T) {
	s := newPhysicsTestServer()
	ent := allocPhysicsTestEdict(s)
	ent.SetFlags(s, float32(FlagOnGround))
	ent.SetOrigin(s, [3]float32{1, 2, 3})
	ent.SetVelocity(s, [3]float32{50, 60, 70})

	s.PhysicsToss(ent)

	if ent.Origin(s) != [3]float32{1, 2, 3} {
		t.Fatalf("origin changed on ground toss: %v", ent.Origin(s))
	}
}

// TestFlyMoveDoesNotGroundOnNonBSPFloor tests FlyMove collision behavior with different entity types.
// It verifying that entities only \"land\" (set onground flag) on BSP world geometry, not on simple trigger boxes or other non-solid entities.
// Where in C: SV_FlyMove in sv_phys.c
func TestFlyMoveDoesNotGroundOnNonBSPFloor(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	s.FrameTime = 0.1
	s.WorldModel = CreateSyntheticWorldModel()
	if len(s.Edicts) == 0 || s.Edicts[0] == nil {
		t.Fatal("missing world edict")
	}
	s.Edicts[0].SetSolid(s, float32(SolidBSP))
	s.ClearWorld()

	platform := s.AllocEdict()
	if platform == nil {
		t.Fatal("failed to alloc platform")
	}
	platform.SetOrigin(s, [3]float32{0, 0, 72})
	platform.SetMins(s, [3]float32{-64, -64, -8})
	platform.SetMaxs(s, [3]float32{64, 64, 8})
	platform.SetSolid(s, float32(SolidBBox))
	s.LinkEdict(platform, false)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to alloc mover")
	}
	ent.SetOrigin(s, [3]float32{0, 0, 112})
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetMoveType(s, float32(MoveTypeWalk))
	ent.SetVelocity(s, [3]float32{0, 0, -200})
	s.LinkEdict(ent, false)

	blocked := s.FlyMove(ent, s.FrameTime, nil)
	if blocked&1 == 0 {
		t.Fatalf("FlyMove blocked=%d, want floor contact bit set", blocked)
	}
	if uint32(ent.Flags(s))&FlagOnGround != 0 {
		t.Fatalf("Flags unexpectedly include onground after SolidBBox contact: flags=%#x", uint32(ent.Flags(s)))
	}
	if ent.GroundEntity(s) != 0 {
		t.Fatalf("ground entity = %d, want 0 for non-BSP contact", ent.GroundEntity(s))
	}
}

// TestPhysicsStepOnGroundSkipsFreefall tests step physics for grounded entities.
// It ensuring that entities already on the ground don't erroneously apply gravity or vertical movement intended for freefall.
// Where in C: SV_Physics_Step in sv_phys.c
func TestPhysicsStepOnGroundSkipsFreefall(t *testing.T) {
	s := newPhysicsTestServer()
	ent := allocPhysicsTestEdict(s)
	ent.SetFlags(s, float32(FlagOnGround))
	ent.SetVelocity(s, [3]float32{0, 0, 42})

	s.PhysicsStep(ent)

	if ent.Velocity(s)[2] != 42 {
		t.Fatalf("z velocity changed: %v", ent.Velocity(s)[2])
	}
}

// TestPhysicsStepHardLandingStartsCanonicalSound tests landing sound triggers in the physics engine.
// It verifying that falling from a height correctly triggers the \"landing\" sound (demon/dland2.wav) via the network protocol.
// Where in C: SV_Physics_Step in sv_phys.c
func TestPhysicsStepHardLandingStartsCanonicalSound(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.WorldModel = CreateSyntheticWorldModel()
	s.Edicts[0].SetSolid(s, float32(SolidBSP))
	s.ClearWorld()
	s.SoundPrecache[1] = "demon/dland2.wav"

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to allocate step entity")
	}
	ent.SetMoveType(s, float32(MoveTypeStep))
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})
	ent.SetOrigin(s, [3]float32{0, 0, 32})
	ent.SetVelocity(s, [3]float32{0, 0, -120})
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
		ent.SetMoveType(s, float32(MoveTypeWalk))
		ent.SetSolid(s, float32(SolidSlideBox))
		ent.SetFlags(s, float32(FlagOnGround))
		ent.SetMins(s, [3]float32{-16, -16, -24})
		ent.SetMaxs(s, [3]float32{16, 16, 32})
		ent.SetOrigin(s, [3]float32{0, 0, 24})
		ent.SetVelocity(s, [3]float32{100, 0, 0})
		s.LinkEdict(ent, false)
		return ent
	}
	newObstacle := func(s *Server) {
		obstacle := s.AllocEdict()
		if obstacle == nil {
			t.Fatal("failed to allocate obstacle")
		}
		obstacle.SetSolid(s, float32(SolidBBox))
		obstacle.SetOrigin(s, [3]float32{32, 0, 8})
		obstacle.SetMins(s, [3]float32{-8, -32, -8})
		obstacle.SetMaxs(s, [3]float32{8, 32, 8})
		s.LinkEdict(obstacle, false)
	}
	newServerWithStep := func() *Server {
		s := NewServer()
		newServerTestVM(s, 16)
		if err := s.Init(1); err != nil {
			t.Fatalf("init server: %v", err)
		}
		s.WorldModel = CreateSyntheticWorldModel()
		s.Edicts[0].SetSolid(s, float32(SolidBSP))
		s.ClearWorld()
		newObstacle(s)
		return s
	}

	withStep := newServerWithStep()
	withPhysicsCVars(t, withStep, map[string]string{"sv_nostep": "0"})
	stepMover := newMover(withStep)
	withStep.SV_WalkMove(stepMover)

	noStep := newServerWithStep()
	withPhysicsCVars(t, noStep, map[string]string{"sv_nostep": "1"})
	noStepMover := newMover(noStep)
	noStep.SV_WalkMove(noStepMover)

	if !(stepMover.Origin(withStep)[0] > noStepMover.Origin(noStep)[0]+0.5) {
		t.Fatalf("sv_nostep did not suppress step retry: stepped=%v nostep=%v", stepMover.Origin(withStep), noStepMover.Origin(noStep))
	}
}

func TestSVWalkMoveStepDownGroundsOnBSPContactForNonBSPMover(t *testing.T) {
	s := NewServer()
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	s.WorldModel = CreateSyntheticWorldModel()
	s.Edicts[0].SetSolid(s, float32(SolidBSP))
	s.ClearWorld()

	obstacle := s.AllocEdict()
	if obstacle == nil {
		t.Fatal("failed to allocate obstacle")
	}
	obstacle.SetSolid(s, float32(SolidBSP))
	obstacle.SetOrigin(s, [3]float32{32, 0, 8})
	obstacle.SetMins(s, [3]float32{-8, -32, -8})
	obstacle.SetMaxs(s, [3]float32{8, 32, 8})
	s.LinkEdict(obstacle, false)

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to allocate mover")
	}
	ent.SetMoveType(s, float32(MoveTypeWalk))
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetFlags(s, float32(FlagOnGround))
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})
	ent.SetOrigin(s, [3]float32{0, 0, 24})
	ent.SetVelocity(s, [3]float32{100, 0, 0})
	s.LinkEdict(ent, false)

	withPhysicsCVars(t, s, map[string]string{"sv_nostep": "0"})
	s.SV_WalkMove(ent)

	if uint32(ent.Flags(s))&FlagOnGround == 0 {
		t.Fatalf("SV_WalkMove did not set onground after BSP contact: flags=%#x", uint32(ent.Flags(s)))
	}
	if got, want := ent.GroundEntity(s), int32(s.NumForEdict(obstacle)); got != want {
		t.Fatalf("ground entity = %d, want %d", got, want)
	}
}

func TestWalkMoveNeedsUnstickUsesDistEpsilonThreshold(t *testing.T) {
	oldOrg := [3]float32{100, 200, 0}

	// Strictly inside threshold on both axes should request unstick.
	inside := [3]float32{100 + DistEpsilon - 0.0001, 200 - DistEpsilon + 0.0001, 0}
	if !WalkMoveNeedsUnstick(oldOrg, inside) {
		t.Fatalf("WalkMoveNeedsUnstick(%v, %v) = false, want true", oldOrg, inside)
	}

	// Exact threshold should not request unstick (strict '<' parity with C code).
	xEdge := [3]float32{100 + DistEpsilon, 200, 0}
	if WalkMoveNeedsUnstick(oldOrg, xEdge) {
		t.Fatalf("WalkMoveNeedsUnstick(%v, %v) = true, want false", oldOrg, xEdge)
	}

	yEdge := [3]float32{100, 200 - DistEpsilon, 0}
	if WalkMoveNeedsUnstick(oldOrg, yEdge) {
		t.Fatalf("WalkMoveNeedsUnstick(%v, %v) = true, want false", oldOrg, yEdge)
	}

	// Outside threshold on either axis should not request unstick.
	outside := [3]float32{100 + DistEpsilon + 0.0001, 200, 0}
	if WalkMoveNeedsUnstick(oldOrg, outside) {
		t.Fatalf("WalkMoveNeedsUnstick(%v, %v) = true, want false", oldOrg, outside)
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
	newServerTestVM(s, 16)
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}
	qc.RegisterBuiltins(s.QCVM)
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

	mover.SetOrigin(s, [3]float32{})
	mover.SetMins(s, [3]float32{-16, -16, -16})
	mover.SetMaxs(s, [3]float32{16, 16, 16})
	mover.SetSolid(s, float32(SolidBBox))
	mover.SetMoveType(s, float32(MoveTypeNone))
	s.LinkEdict(mover, false)

	trigger.SetOrigin(s, [3]float32{})
	trigger.SetMins(s, [3]float32{-8, -8, -8})
	trigger.SetMaxs(s, [3]float32{8, 8, 8})
	trigger.SetSolid(s, float32(SolidTrigger))
	trigger.SetTouch(s, 1)
	trigger.SetMoveType(s, float32(MoveTypeNone))
	s.LinkEdict(trigger, false)

	vm.SetGlobal("force_retouch", float32(2))

	s.Physics()
	if got := vm.GlobalFloat("force_retouch"); got != 1 {
		t.Fatalf("force_retouch after first frame = %v, want 1", got)
	}
	firstCalls := triggerCalls
	if firstCalls == 0 {
		t.Fatal("force_retouch frame 1 did not trigger callback")
	}

	s.Physics()
	if got := vm.GlobalFloat("force_retouch"); got != 0 {
		t.Fatalf("force_retouch after second frame = %v, want 0", got)
	}
	secondCalls := triggerCalls
	if secondCalls <= firstCalls {
		t.Fatalf("force_retouch frame 2 did not trigger additional callback: first=%d second=%d", firstCalls, secondCalls)
	}

	s.Physics()
	if got := vm.GlobalFloat("force_retouch"); got != 0 {
		t.Fatalf("force_retouch after third frame = %v, want 0", got)
	}
	if triggerCalls != secondCalls {
		t.Fatalf("force_retouch kept triggering after countdown expired: before=%d after=%d", secondCalls, triggerCalls)
	}
}

// TestPhysicsSendIntervalMatchesFitzQuakeParity verifies the FitzQuake
// sendinterval gate remains byte-for-byte aligned with the C reference in
// sv_phys.c: non-default cadences round from (nextthink-oldthinktime)*255,
// 25/26 are suppressed as "close enough" to 0.1, and non-step/non-walk
// entities only send when their animation frame changed.
func TestPhysicsSendIntervalMatchesFitzQuakeParity(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		moveType MoveType
		frame    float32
		oldFrame float32
		ticks    float32
		wantSend bool
	}{
		{name: "frame change sends 24 tick cadence", moveType: MoveTypeNone, frame: 1, oldFrame: 0, ticks: 24, wantSend: true},
		{name: "frame change suppresses 25 tick cadence", moveType: MoveTypeNone, frame: 1, oldFrame: 0, ticks: 25, wantSend: false},
		{name: "frame change suppresses 26 tick cadence", moveType: MoveTypeNone, frame: 1, oldFrame: 0, ticks: 26, wantSend: false},
		{name: "frame change sends 27 tick cadence", moveType: MoveTypeNone, frame: 1, oldFrame: 0, ticks: 27, wantSend: true},
		{name: "step mover sends without frame change", moveType: MoveTypeStep, frame: 0, oldFrame: 0, ticks: 27, wantSend: true},
		{name: "non step unchanged frame does not send", moveType: MoveTypeNone, frame: 0, oldFrame: 0, ticks: 27, wantSend: false},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			s := newPhysicsTestServer()
			s.Time = 10
			s.FrameTime = 0.001

			ent := allocPhysicsTestEdict(s)
			ent.SetMoveType(s, float32(tc.moveType))
			ent.SetFrame(s, tc.frame)
			ent.OldFrame = tc.oldFrame
			ent.OldThinkTime = s.Time
			ent.SetNextThink(s, s.Time+tc.ticks/255)
			if tc.moveType == MoveTypeStep {
				ent.SetFlags(s, float32(FlagOnGround))
			}

			s.Edicts = append(s.Edicts, ent)
			s.NumEdicts = len(s.Edicts)

			s.Physics()

			if ent.SendInterval != tc.wantSend {
				t.Fatalf("SendInterval=%v for moveType=%v frame=%v oldFrame=%v ticks=%v nextThink=%v oldThinkTime=%v",
					ent.SendInterval, tc.moveType, tc.frame, tc.oldFrame, tc.ticks, ent.NextThink(s), ent.OldThinkTime)
			}
		})
	}
}

// TestPhysicsFreezeNonClientsCVar tests the sv_freezenonclients cvar.
// It allowing the server to pause non-player entities (monsters, etc.) for performance or debugging.
// Where in C: SV_Physics in sv_phys.c
func TestPhysicsFreezeNonClientsCVar(t *testing.T) {
	mkServer := func() (*Server, *Edict, *Edict) {
		s := newPhysicsTestServer()
		s.Static = &ServerStatic{MaxClients: 1}
		clientEnt := allocPhysicsTestEdict(s)
		clientEnt.SetMoveType(s, float32(MoveTypeNoClip))
		clientEnt.SetVelocity(s, [3]float32{10, 0, 0})
		nonClientEnt := allocPhysicsTestEdict(s)
		nonClientEnt.SetMoveType(s, float32(MoveTypeNoClip))
		nonClientEnt.SetVelocity(s, [3]float32{20, 0, 0})
		s.Edicts = append(s.Edicts, clientEnt, nonClientEnt)
		s.NumEdicts = len(s.Edicts)
		return s, clientEnt, nonClientEnt
	}

	t.Run("freeze enabled skips non-clients", func(t *testing.T) {
		s, clientEnt, nonClientEnt := mkServer()
		withPhysicsCVars(t, s, map[string]string{"sv_freezenonclients": "1"})
		before := s.Time

		s.Physics()

		if clientEnt.Origin(s)[0] == 0 {
			t.Fatalf("client entity did not move with freeze enabled: origin=%v", clientEnt.Origin(s))
		}
		if nonClientEnt.Origin(s)[0] != 0 {
			t.Fatalf("non-client entity moved with freeze enabled: origin=%v", nonClientEnt.Origin(s))
		}
		if s.Time != before {
			t.Fatalf("server time advanced with freeze enabled: before=%v after=%v", before, s.Time)
		}
	})

	t.Run("freeze disabled updates all entities", func(t *testing.T) {
		s, clientEnt, nonClientEnt := mkServer()
		withPhysicsCVars(t, s, map[string]string{"sv_freezenonclients": "0"})

		s.Physics()

		if clientEnt.Origin(s)[0] == 0 {
			t.Fatalf("client entity did not move with freeze disabled: origin=%v", clientEnt.Origin(s))
		}
		if nonClientEnt.Origin(s)[0] == 0 {
			t.Fatalf("non-client entity did not move with freeze disabled: origin=%v", nonClientEnt.Origin(s))
		}
	})
}

func TestPhysicsSkipsInvalidMoveType(t *testing.T) {
	s := newPhysicsTestServer()
	bad := allocPhysicsTestEdict(s)
	bad.SetMoveType(s, 999)
	s.Edicts = append(s.Edicts, bad)
	s.NumEdicts = len(s.Edicts)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Physics() unexpectedly panicked on invalid movetype: %v", r)
		}
	}()

	s.Physics()
}

// TestPhysicsTelemetryFrameHooks tests physics telemetry.
// It providing detailed performance and event logging for the physics engine.
// Where in C: N/A (Modern engine extension)
func TestPhysicsTelemetryFrameHooks(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()

	ent := allocPhysicsTestEdict(s)
	ent.SetMoveType(s, float32(MoveTypeNoClip))
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)

	lines := make([]string, 0, 4)
	s.DebugTelemetry = NewDebugTelemetryWithConfig(func() DebugTelemetryConfig {
		return DebugTelemetryConfig{
			Enabled:      true,
			EventMask:    debugEventMaskFrame,
			EntityFilter: debugEntityFilter{All: true},
			SummaryMode:  2,
		}
	}, func(line string) {
		lines = append(lines, line)
	})
	oldEnable := debugTelemetryEnableCVar
	debugTelemetryEnableCVar = s.CVar.Register("sv_debug_telemetry_test_frame", "1", cvar.FlagNone, "")
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

func TestPhysicsStartFrameSeesFreshEdictButtonState(t *testing.T) {
	s := newPhysicsTestServer()
	s.QCVM = qc.NewVM()
	vm := newServerTestVM(s, 8)

	const (
		captureBuiltinOfs = 10
		capturedButtonOfs = 20
	)

	vm.GlobalDefs = []qc.DDef{
		{Type: uint16(qc.EvFloat), Ofs: uint16(capturedButtonOfs), Name: vm.AllocString("captured_button0")},
	}
	vm.Builtins[1] = func(vm *qc.VM) {
		vm.SetGFloat(capturedButtonOfs, vm.EFloat(1, qc.EntFieldButton0))
	}
	vm.Functions = []qc.DFunction{
		{},
		{Name: vm.AllocString("StartFrame"), FirstStatement: 0},
	}
	vm.Statements = []qc.DStatement{
		{Op: uint16(qc.OPCall0), A: uint16(captureBuiltinOfs)},
		{Op: uint16(qc.OPDone)},
	}
	vm.SetGInt(captureBuiltinOfs, -1)

	player := allocPhysicsTestEdict(s)
	player.SetButton0(s, 1)
	s.Edicts = append(s.Edicts, player)
	s.NumEdicts = len(s.Edicts)
	vm.NumEdicts = s.NumEdicts

	s.Physics()

	if got := vm.GFloat(capturedButtonOfs); got != 1 {
		t.Fatalf("StartFrame saw button0 = %v, want 1 from latest edict state", got)
	}
}
