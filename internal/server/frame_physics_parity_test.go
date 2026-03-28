package server

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/model"
)

func newSyntheticClientServer(t *testing.T) (*Server, *Client, *Edict) {
	t.Helper()

	s := NewServer()
	if err := s.Init(1); err != nil {
		t.Fatalf("init server: %v", err)
	}

	s.Active = true
	s.WorldModel = CreateSyntheticWorldModel()
	s.Edicts[0].Vars.Solid = float32(SolidBSP)
	s.ClearWorld()

	client := s.Static.Clients[0]
	client.Active = true
	client.Spawned = true
	client.Message = NewMessageBuffer(256)

	ent := client.Edict
	ent.Free = false
	ent.Vars.MoveType = float32(MoveTypeWalk)
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.Health = 100
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Size = [3]float32{32, 32, 56}
	ent.Vars.Origin = [3]float32{0, 0, 24}
	ent.Vars.Velocity = [3]float32{}
	ent.Vars.Flags = float32(FlagOnGround)
	s.LinkEdict(ent, true)

	return s, client, ent
}

func TestFrameProcessesClientMoveBeforePhysics(t *testing.T) {
	s, client, ent := newSyntheticClientServer(t)

	msg := NewMessageBuffer(128)
	msg.WriteChar(int8(CLCMove))
	msg.WriteFloat(s.Time - 0.05)
	msg.WriteShort(0)
	msg.WriteShort(0)
	msg.WriteShort(0)
	msg.WriteShort(200)
	msg.WriteShort(0)
	msg.WriteShort(0)
	msg.WriteByte(0)
	msg.WriteByte(0)
	msg.WriteChar(-1)
	client.Message = finalizeMessage(msg)

	before := ent.Vars.Origin
	if err := s.Frame(0.05); err != nil {
		t.Fatalf("frame failed: %v", err)
	}

	if ent.Vars.Origin[0] <= before[0] {
		t.Fatalf("expected same-frame move along +X, before=%v after=%v", before, ent.Vars.Origin)
	}
}

func TestFrameAdvancesTimeOnce(t *testing.T) {
	s, _, _ := newSyntheticClientServer(t)

	before := s.Time
	if err := s.Frame(0.05); err != nil {
		t.Fatalf("frame failed: %v", err)
	}

	if got, want := s.FrameTime, float32(0.05); got != want {
		t.Fatalf("frametime = %v, want %v", got, want)
	}
	if got, want := s.Time, before+0.05; got != want {
		t.Fatalf("time advanced incorrectly: got %v want %v (before %v)", got, want, before)
	}
}

func TestPhysicsWalkAppliesGravityAirborne(t *testing.T) {
	s, _, ent := newSyntheticClientServer(t)
	ent.Vars.Flags = 0
	ent.Vars.Origin = [3]float32{0, 0, 128}
	ent.Vars.Velocity = [3]float32{}
	s.LinkEdict(ent, true)

	beforeZ := ent.Vars.Origin[2]
	s.PhysicsWalk(ent)

	if ent.Vars.Velocity[2] >= 0 {
		t.Fatalf("expected downward velocity after gravity, got %v", ent.Vars.Velocity[2])
	}
	if ent.Vars.Origin[2] >= beforeZ {
		t.Fatalf("expected airborne entity to descend, before=%v after=%v", beforeZ, ent.Vars.Origin[2])
	}
}

func TestAddGravityUsesQCGravityFieldWhenPresent(t *testing.T) {
	s, _, ent := newSyntheticClientServer(t)
	s.FrameTime = 0.1
	ent.Vars.Velocity = [3]float32{}

	vm := newTestQCVM()
	s.QCVM = vm
	s.QCFieldGravity = 0

	entNum := s.NumForEdict(ent)
	if entNum < 0 {
		t.Fatal("expected client edict to be linked into server edict table")
	}

	cases := []struct {
		name    string
		gravity float32
		wantZ   float32
	}{
		{name: "custom multiplier", gravity: 0.5, wantZ: -40},
		{name: "zero falls back to world gravity", gravity: 0, wantZ: -80},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ent.Vars.Velocity = [3]float32{}
			vm.SetEFloat(entNum, s.QCFieldGravity, tc.gravity)

			s.AddGravity(ent)

			if got := ent.Vars.Velocity[2]; got < tc.wantZ-0.001 || got > tc.wantZ+0.001 {
				t.Fatalf("velocity[2] = %v, want %v", got, tc.wantZ)
			}
		})
	}
}

func TestCheckWaterTransitionSetsOutOfWaterLevelToContentsValue(t *testing.T) {
	s, _, ent := newSyntheticClientServer(t)
	// Force PointContents(origin) to report open air.
	s.WorldModel = &model.Model{
		Hulls: [4]model.Hull{{
			ClipNodes: []model.MClipNode{{
				PlaneNum: 0,
				Children: [2]int{bsp.ContentsEmpty, bsp.ContentsEmpty},
			}},
			Planes: []model.MPlane{{Normal: [3]float32{0, 0, 1}, Dist: 0}},
		}},
	}

	ent.Vars.Origin = [3]float32{0, 0, 128}
	ent.Vars.WaterType = float32(bsp.ContentsWater)
	ent.Vars.WaterLevel = 1

	s.CheckWaterTransition(ent)

	if got, want := ent.Vars.WaterType, float32(bsp.ContentsEmpty); got != want {
		t.Fatalf("watertype = %v, want %v", got, want)
	}
	if got, want := ent.Vars.WaterLevel, float32(bsp.ContentsEmpty); got != want {
		t.Fatalf("waterlevel = %v, want %v", got, want)
	}
}

func TestPhysicsWalkSkipsGravityUnderwater(t *testing.T) {
	s, _, ent := newSyntheticClientServer(t)
	// SV_CheckWater needs a WorldModel to perform PointContents checks
	s.WorldModel = &model.Model{
		Hulls: [4]model.Hull{{
			ClipNodes: []model.MClipNode{{
				PlaneNum: 0,
				Children: [2]int{bsp.ContentsWater, bsp.ContentsWater},
			}},
			Planes: []model.MPlane{{Normal: [3]float32{0, 0, 1}, Dist: 0}},
		}},
	}

	ent.Vars.Flags = 0
	ent.Vars.WaterLevel = 2
	ent.Vars.Origin = [3]float32{0, 0, 128}
	ent.Vars.Velocity = [3]float32{}
	s.LinkEdict(ent, true)

	before := ent.Vars.Origin
	s.PhysicsWalk(ent)

	if ent.Vars.Velocity[2] != 0 {
		t.Fatalf("expected no gravity underwater, got velocity %v", ent.Vars.Velocity)
	}
	if ent.Vars.Origin != before {
		t.Fatalf("expected underwater entity to stay put without movement input, before=%v after=%v", before, ent.Vars.Origin)
	}
}

func TestPhysicsWalkCollidesWithWorldSolid(t *testing.T) {
	s := newStartMapDiagnosticsServer(t)

	s.ConnectClient(0)
	client := s.Static.Clients[0]
	client.Active = true
	client.Spawned = true
	ent := client.Edict
	ent.Free = false
	ent.Vars.MoveType = float32(MoveTypeWalk)
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.Health = 100
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Size = [3]float32{32, 32, 56}

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if !ok {
		t.Skipf("no walkable point found on start map; %s", diag.String())
	}

	directions := [][3]float32{
		{1, 0, 0},
		{-1, 0, 0},
		{0, 1, 0},
		{0, -1, 0},
		{0.70710677, 0.70710677, 0},
		{0.70710677, -0.70710677, 0},
		{-0.70710677, 0.70710677, 0},
		{-0.70710677, -0.70710677, 0},
	}

	var (
		haveCollision bool
		start         [3]float32
		velocity      [3]float32
		expectedEnd   [3]float32
	)
	for _, dir := range directions {
		farEnd := [3]float32{pos[0] + dir[0]*256, pos[1] + dir[1]*256, pos[2]}
		farTrace := s.Move(pos, ent.Vars.Mins, ent.Vars.Maxs, farEnd, MoveNormal, ent)
		if farTrace.Fraction >= 1 {
			continue
		}

		wallDistance := 256 * farTrace.Fraction
		if wallDistance <= 90 {
			continue
		}

		start = [3]float32{
			pos[0] + dir[0]*(wallDistance-70),
			pos[1] + dir[1]*(wallDistance-70),
			pos[2],
		}
		ent.Vars.Origin = start
		s.LinkEdict(ent, true)
		if blocker := s.TestEntityPosition(ent); blocker != nil {
			continue
		}

		plannedEnd := [3]float32{
			start[0] + dir[0]*60,
			start[1] + dir[1]*60,
			start[2],
		}
		trace := s.Move(start, ent.Vars.Mins, ent.Vars.Maxs, plannedEnd, MoveNormal, ent)
		if trace.Fraction < 1 {
			haveCollision = true
			expectedEnd = trace.EndPos
			velocity = [3]float32{dir[0] * 600, dir[1] * 600, 0}
			break
		}
	}
	if !haveCollision {
		t.Skip("could not find near-wall movement vector on start map")
	}

	ent.Vars.Flags = float32(FlagOnGround)
	ent.Vars.Velocity = velocity
	ent.Vars.Origin = start
	s.LinkEdict(ent, true)

	s.PhysicsWalk(ent)

	const epsilon = 0.01
	for i := 0; i < 3; i++ {
		delta := ent.Vars.Origin[i] - expectedEnd[i]
		if delta < -epsilon || delta > epsilon {
			t.Fatalf("axis %d mismatch after wall collision: got=%v expected=%v", i, ent.Vars.Origin, expectedEnd)
		}
	}
}
