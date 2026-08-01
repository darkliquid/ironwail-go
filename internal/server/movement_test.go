package server

import (
	"github.com/darkliquid/ironwail-go/internal/qc"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/compatrand"
	"github.com/darkliquid/ironwail-go/internal/model"
)

func newMovementTestServer() *Server {
	s := &Server{
		Gravity:     800,
		MaxVelocity: 2000,
		FrameTime:   0.1,
		MaxEdicts:   64,
		Edicts:      []*Edict{{}},
		NumEdicts:   1,
	}
	s.SetCompatRNG(compatrand.New())
	s.QCVM = qc.NewVM()
	newServerTestVM(s, 64)
	s.ensureQCVMEdictStorage()
	return s
}

func TestChangeYaw(t *testing.T) {
	s := newMovementTestServer()
	ent := s.AllocEdict()
	ang := ent.Angles(s)
	ang[1] = 10
	ent.SetAngles(s, ang)
	ent.SetIdealYaw(s, 350)
	ent.SetYawSpeed(s, 15)

	s.changeYaw(ent)
	// anglemod uses 16-bit quantization matching C, so 355 becomes ~355.00122
	if got := ent.Angles(s)[1]; got < 354.99 || got > 355.01 {
		t.Fatalf("angles yaw = %v, want ~355", got)
	}
}

func TestCloseEnough(t *testing.T) {
	s := newMovementTestServer()
	ent := s.AllocEdict()
	goal := s.AllocEdict()
	ent.SetAbsMin(s, [3]float32{0, 0, 0})
	ent.SetAbsMax(s, [3]float32{16, 16, 16})
	goal.SetAbsMin(s, [3]float32{30, 0, 0})
	goal.SetAbsMax(s, [3]float32{46, 16, 16})

	if s.CloseEnough(ent, goal, 13.9) {
		t.Fatalf("CloseEnough returned true with insufficient distance")
	}
	if !s.CloseEnough(ent, goal, 14.0) {
		t.Fatalf("CloseEnough returned false at touching distance")
	}
}

func TestSVHullForEntityAndSVMoveWrappers(t *testing.T) {
	s := newMovementTestServer()
	ent := s.AllocEdict()
	ent.SetOrigin(s, [3]float32{10, 20, 30})
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})

	h, offset := s.SV_HullForEntity(ent, [3]float32{}, [3]float32{})
	if h == nil {
		t.Fatalf("SV_HullForEntity returned nil hull")
	}
	if offset != ent.Origin(s) {
		t.Fatalf("offset = %v, want %v", offset, ent.Origin(s))
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

	ent := s.AllocEdict()
	ent.SetOrigin(s, [3]float32{})
	ent.SetSolid(s, float32(SolidBSP))
	ent.SetMoveType(s, float32(MoveTypePush))
	ent.SetModelIndex(s, 2)

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

func TestMovementOnSpawnedMap(t *testing.T) {
	s := newStartMapDiagnosticsServer(t)

	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if !ok {
		t.Skipf("no walkable point found on start map; %s", diag.String())
	}

	ent := s.AllocEdict()
	ent.SetOrigin(s, pos)
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetMoveType(s, float32(MoveTypeStep))
	ent.SetFlags(s, float32(FlagOnGround))
	s.Edicts = append(s.Edicts, ent)
	s.NumEdicts = len(s.Edicts)
	s.LinkEdict(ent, false)

	if blocker := s.SV_TestEntityPosition(ent); blocker != nil {
		t.Fatalf("SV_TestEntityPosition found blocker at valid position; %s", diag.String())
	}

	h, _ := s.SV_HullForEntity(ent, ent.Mins(s), ent.Maxs(s))
	if h == nil {
		t.Fatalf("SV_HullForEntity returned nil on spawned map")
	}

	if !s.CheckBottom(ent) {
		t.Skipf("sampled position does not satisfy CheckBottom; %s", diag.String())
	}

	before := ent.Origin(s)
	if !s.MoveStep(ent, [3]float32{}, true) {
		t.Fatalf("MoveStep failed on stationary grounded entity; %s", diag.String())
	}
	if ent.Origin(s) != before {
		t.Fatalf("MoveStep with zero move changed origin: before=%v after=%v", before, ent.Origin(s))
	}
}

func TestMoveToGoalRandomBranchUsesSharedCompatRNG(t *testing.T) {
	s := newMovementTestServer()

	goal := s.AllocEdict()
	goal.SetOrigin(s, [3]float32{64, 0, 0})
	goal.SetAbsMin(s, [3]float32{64, 0, 0})
	goal.SetAbsMax(s, [3]float32{64, 0, 0})

	ent := s.AllocEdict()
	ent.SetFlags(s, float32(FlagFly))
	ent.SetIdealYaw(s, 90)
	ang2 := ent.Angles(s)
	ang2[1] = 90
	ent.SetAngles(s, ang2)
	ent.SetYawSpeed(s, 360)
	ent.SetGoalEntity(s, 1)

	s.Edicts = append(s.Edicts, goal, ent)
	s.NumEdicts = len(s.Edicts)

	s.compatRand()
	s.compatRand()

	if !s.MoveToGoal(ent, 16) {
		t.Fatal("MoveToGoal returned false")
	}
	if got := ent.Origin(s); got != [3]float32{16, 0, 0} {
		t.Fatalf("origin = %v, want eastward chase step", got)
	}
	if got := ent.IdealYaw(s); got != 0 {
		t.Fatalf("IdealYaw = %v, want 0", got)
	}
}

func TestNewChaseDirUsesCanonicalQuakeSouthwestBias(t *testing.T) {
	s := newMovementTestServer()

	actor := s.AllocEdict()
	actor.SetFlags(s, float32(FlagFly))
	actor.SetYawSpeed(s, 360)

	enemy := s.AllocEdict()
	enemy.SetOrigin(s, [3]float32{-64, -64, 0})

	s.Edicts = append(s.Edicts, actor, enemy)
	s.NumEdicts = len(s.Edicts)

	s.NewChaseDir(actor, enemy, 16)

	wantX := float32(-13.10643)
	wantY := float32(-9.177243)
	if got := actor.Origin(s); got[0] < wantX-0.01 || got[0] > wantX+0.01 || got[1] < wantY-0.01 || got[1] > wantY+0.01 {
		t.Fatalf("origin = %v, want canonical 215-degree chase step", got)
	}
	if got := actor.IdealYaw(s); got != 215 {
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
	newServerTestVM(s, 16)
	s.WorldModel = createSyntheticPlatformWorldModel()
	if len(s.Edicts) > 0 && s.Edicts[0] != nil {
		s.Edicts[0].SetSolid(s, float32(SolidBSP))
	}
	s.ClearWorld()

	ent := s.AllocEdict()
	if ent == nil {
		t.Fatal("failed to allocate test edict")
	}
	ent.SetOrigin(s, [3]float32{32, 0, 24})
	ent.SetMins(s, [3]float32{-16, -16, -24})
	ent.SetMaxs(s, [3]float32{16, 16, 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetMoveType(s, float32(MoveTypeStep))
	ent.SetFlags(s, float32(FlagOnGround))
	s.LinkEdict(ent, false)

	if !s.CheckBottom(ent) {
		t.Fatal("expected starting position to be fully supported")
	}

	start := ent.Origin(s)
	if s.MoveStep(ent, [3]float32{-20, 0, 0}, true) {
		t.Fatalf("MoveStep unexpectedly accepted unsupported platform step: start=%v end=%v", start, ent.Origin(s))
	}
	if got := ent.Origin(s); got != start {
		t.Fatalf("origin after rejected step = %v, want %v", got, start)
	}
}
