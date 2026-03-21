package server

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/compatrand"
	"github.com/ironwail/ironwail-go/internal/model"
)

func newMovementTestServer() *Server {
	s := &Server{
		Gravity:     800,
		MaxVelocity: 2000,
		FrameTime:   0.1,
		Edicts:      []*Edict{{Vars: &EntVars{}}},
		NumEdicts:   1,
	}
	s.SetCompatRNG(compatrand.New())
	return s
}

func TestChangeYaw(t *testing.T) {
	s := newMovementTestServer()
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Angles[1] = 10
	ent.Vars.IdealYaw = 350
	ent.Vars.YawSpeed = 15

	s.changeYaw(ent)
	// anglemod uses 16-bit quantization matching C, so 355 becomes ~355.00122
	if got := ent.Vars.Angles[1]; got < 354.99 || got > 355.01 {
		t.Fatalf("angles yaw = %v, want ~355", got)
	}
}

func TestCloseEnough(t *testing.T) {
	s := newMovementTestServer()
	ent := &Edict{Vars: &EntVars{}}
	goal := &Edict{Vars: &EntVars{}}
	ent.Vars.AbsMin = [3]float32{0, 0, 0}
	ent.Vars.AbsMax = [3]float32{16, 16, 16}
	goal.Vars.AbsMin = [3]float32{30, 0, 0}
	goal.Vars.AbsMax = [3]float32{46, 16, 16}

	if s.CloseEnough(ent, goal, 13.9) {
		t.Fatalf("CloseEnough returned true with insufficient distance")
	}
	if !s.CloseEnough(ent, goal, 14.0) {
		t.Fatalf("CloseEnough returned false at touching distance")
	}
}

func TestSVHullForEntityAndSVMoveWrappers(t *testing.T) {
	s := newMovementTestServer()
	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Origin = [3]float32{10, 20, 30}
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}

	h, offset := s.SV_HullForEntity(ent, [3]float32{}, [3]float32{})
	if h == nil {
		t.Fatalf("SV_HullForEntity returned nil hull")
	}
	if offset != ent.Vars.Origin {
		t.Fatalf("offset = %v, want %v", offset, ent.Vars.Origin)
	}

	start := [3]float32{0, 0, 0}
	end := [3]float32{16, 0, 0}
	a := s.Move(start, [3]float32{}, [3]float32{}, end, MoveType(MoveNormal), nil)
	b := s.SV_Move(start, [3]float32{}, [3]float32{}, end, MoveType(MoveNormal), nil)
	if a.Fraction != b.Fraction || a.StartSolid != b.StartSolid || a.AllSolid != b.AllSolid || a.EndPos != b.EndPos {
		t.Fatalf("SV_Move wrapper mismatch: base=%+v wrapper=%+v", a, b)
	}
}

func TestSVHullForInlineBrushModelUsesSubmodelHeadnode(t *testing.T) {
	s := newMovementTestServer()
	wm := &model.Model{
		Type:   model.ModBrush,
		Planes: []model.MPlane{{Normal: [3]float32{1, 0, 0}, Dist: 0, Type: 0}, {Normal: [3]float32{1, 0, 0}, Dist: 10, Type: 0}},
	}
	wm.Hulls[1] = model.Hull{
		ClipNodes: []model.MClipNode{
			{PlaneNum: 0, Children: [2]int{bsp.ContentsEmpty, bsp.ContentsSolid}},
			{PlaneNum: 1, Children: [2]int{bsp.ContentsEmpty, bsp.ContentsSolid}},
		},
		Planes:        wm.Planes,
		FirstClipNode: 0,
		LastClipNode:  1,
		ClipMins:      [3]float32{-16, -16, -24},
		ClipMaxs:      [3]float32{16, 16, 32},
	}
	s.WorldModel = wm
	s.WorldTree = &bsp.Tree{Models: []bsp.DModel{
		{HeadNode: [bsp.MaxMapHulls]int32{0, 0, 0, 0}},
		{HeadNode: [bsp.MaxMapHulls]int32{0, 1, 1, 0}},
	}}

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Origin = [3]float32{}
	ent.Vars.Solid = float32(SolidBSP)
	ent.Vars.MoveType = float32(MoveTypePush)
	ent.Vars.ModelIndex = 2

	h, _ := s.SV_HullForEntity(ent, [3]float32{-16, -16, -24}, [3]float32{16, 16, 32})
	if h == nil {
		t.Fatal("SV_HullForEntity returned nil hull")
	}
	if h.FirstClipNode != 1 {
		t.Fatalf("first clip node = %d, want 1", h.FirstClipNode)
	}

	trace := s.clipMoveToEntity(ent, [3]float32{20, 0, 0}, [3]float32{-16, -16, -24}, [3]float32{16, 16, 32}, [3]float32{-20, 0, 0})
	if trace.Fraction >= 1 {
		t.Fatalf("trace fraction = %v, want collision", trace.Fraction)
	}
	if trace.EndPos[0] < 9.9 || trace.EndPos[0] > 10.1 {
		t.Fatalf("trace end x = %v, want about 10", trace.EndPos[0])
	}
}

func findWalkablePoint(s *Server) ([3]float32, bool) {
	pos, ok, _ := findWalkablePointWithDiagnostics(s)
	return pos, ok
}

func TestMovementOnSpawnedMap(t *testing.T) {
	s := newStartMapDiagnosticsServer(t)

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if !ok {
		t.Skipf("no walkable point found on start map; %s", diag.String())
	}

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Origin = pos
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.MoveType = float32(MoveTypeStep)
	ent.Vars.Flags = float32(FlagOnGround)
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)
	s.LinkEdict(ent, false)

	if blocker := s.SV_TestEntityPosition(ent); blocker != nil {
		t.Fatalf("SV_TestEntityPosition found blocker at valid position; %s", diag.String())
	}

	h, _ := s.SV_HullForEntity(ent, ent.Vars.Mins, ent.Vars.Maxs)
	if h == nil {
		t.Fatalf("SV_HullForEntity returned nil on spawned map")
	}

	if !s.CheckBottom(ent) {
		t.Skipf("sampled position does not satisfy CheckBottom; %s", diag.String())
	}

	before := ent.Vars.Origin
	if !s.MoveStep(ent, [3]float32{}, true) {
		t.Fatalf("MoveStep failed on stationary grounded entity; %s", diag.String())
	}
	if ent.Vars.Origin != before {
		t.Fatalf("MoveStep with zero move changed origin: before=%v after=%v", before, ent.Vars.Origin)
	}
}

func TestMoveToGoalRandomBranchUsesSharedCompatRNG(t *testing.T) {
	s := newMovementTestServer()

	goal := &Edict{Vars: &EntVars{}}
	goal.Vars.Origin = [3]float32{64, 0, 0}
	goal.Vars.AbsMin = [3]float32{64, 0, 0}
	goal.Vars.AbsMax = [3]float32{64, 0, 0}

	ent := &Edict{Vars: &EntVars{}}
	ent.Vars.Flags = float32(FlagFly)
	ent.Vars.IdealYaw = 90
	ent.Vars.Angles[1] = 90
	ent.Vars.YawSpeed = 360
	ent.Vars.GoalEntity = 1

	s.Edicts = append(s.Edicts, goal, ent)
	s.NumEdicts = len(s.Edicts)

	s.compatRand()
	s.compatRand()

	if !s.MoveToGoal(ent, 16) {
		t.Fatal("MoveToGoal returned false")
	}
	if got := ent.Vars.Origin; got != [3]float32{16, 0, 0} {
		t.Fatalf("origin = %v, want eastward chase step", got)
	}
	if got := ent.Vars.IdealYaw; got != 0 {
		t.Fatalf("IdealYaw = %v, want 0", got)
	}
}

func TestNewChaseDirUsesCanonicalQuakeSouthwestBias(t *testing.T) {
	s := newMovementTestServer()

	actor := &Edict{Vars: &EntVars{}}
	actor.Vars.Flags = float32(FlagFly)
	actor.Vars.YawSpeed = 360

	enemy := &Edict{Vars: &EntVars{}}
	enemy.Vars.Origin = [3]float32{-64, -64, 0}

	s.Edicts = append(s.Edicts, actor, enemy)
	s.NumEdicts = len(s.Edicts)

	s.NewChaseDir(actor, enemy, 16)

	wantX := float32(-13.10643)
	wantY := float32(-9.177243)
	if got := actor.Vars.Origin; got[0] < wantX-0.01 || got[0] > wantX+0.01 || got[1] < wantY-0.01 || got[1] > wantY+0.01 {
		t.Fatalf("origin = %v, want canonical 215-degree chase step", got)
	}
	if got := actor.Vars.IdealYaw; got != 215 {
		t.Fatalf("IdealYaw = %v, want 215", got)
	}
}

func createSyntheticPlatformWorldModel() *model.Model {
	m := &model.Model{}

	var hull model.Hull
	hull.Planes = []model.MPlane{
		{Normal: [3]float32{1, 0, 0}, Dist: 0, Type: 0},
		{Normal: [3]float32{0, 0, 1}, Dist: 0, Type: 2},
	}
	hull.ClipNodes = []model.MClipNode{
		{PlaneNum: 0, Children: [2]int{1, bsp.ContentsEmpty}},
		{PlaneNum: 1, Children: [2]int{bsp.ContentsEmpty, bsp.ContentsSolid}},
	}
	hull.FirstClipNode = 0
	hull.LastClipNode = 1
	hull.ClipMins = [3]float32{-512, -512, -512}
	hull.ClipMaxs = [3]float32{512, 512, 512}

	m.Hulls[0] = hull
	m.Mins = [3]float32{-512, -512, -512}
	m.Maxs = [3]float32{512, 512, 512}
	m.ClipBox = true
	m.ClipMins = m.Mins
	m.ClipMaxs = m.Maxs

	return m
}

func TestMoveStepRejectsUnsupportedStepOffPlatform(t *testing.T) {
	s := NewServer()
	s.WorldModel = createSyntheticPlatformWorldModel()
	if s.Edicts != nil && len(s.Edicts) > 0 && s.Edicts[0] != nil && s.Edicts[0].Vars != nil {
		s.Edicts[0].Vars.Solid = float32(SolidBSP)
	}
	s.ClearWorld()

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to allocate test edict")
	}
	ent.Vars.Origin = [3]float32{32, 0, 24}
	ent.Vars.Mins = [3]float32{-16, -16, -24}
	ent.Vars.Maxs = [3]float32{16, 16, 32}
	ent.Vars.Solid = float32(SolidSlideBox)
	ent.Vars.MoveType = float32(MoveTypeStep)
	ent.Vars.Flags = float32(FlagOnGround)
	s.LinkEdict(ent, false)

	if !s.CheckBottom(ent) {
		t.Fatal("expected starting position to be fully supported")
	}

	start := ent.Vars.Origin
	if s.MoveStep(ent, [3]float32{-20, 0, 0}, true) {
		t.Fatalf("MoveStep unexpectedly accepted unsupported platform step: start=%v end=%v", start, ent.Vars.Origin)
	}
	if got := ent.Vars.Origin; got != start {
		t.Fatalf("origin after rejected step = %v, want %v", got, start)
	}
}
