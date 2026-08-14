// This file belongs to the Tests subsystem: unit, integration, parity, and e2e tests for the server package.

package server

import (
	"testing"

	"github.com/darkliquid/ironwail-go/internal/bsp"
	"github.com/darkliquid/ironwail-go/internal/compatrand"
	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/internal/qc"
	qtypes "github.com/darkliquid/ironwail-go/pkg/types"
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
	ang.Y = 10
	ent.SetAngles(s, ang)
	ent.SetIdealYaw(s, 350)
	ent.SetYawSpeed(s, 15)

	s.changeYaw(ent)
	// anglemod uses 16-bit quantization matching C, so 355 becomes ~355.00122
	if got := ent.Angles(s).Y; got < 354.99 || got > 355.01 {
		t.Fatalf("angles yaw = %v, want ~355", got)
	}
}

func TestCloseEnough(t *testing.T) {
	s := newMovementTestServer()
	ent := s.AllocEdict()
	goal := s.AllocEdict()
	ent.SetAbsMin(s, qtypes.Vec3{X: 0, Y: 0, Z: 0})
	ent.SetAbsMax(s, qtypes.Vec3{X: 16, Y: 16, Z: 16})
	goal.SetAbsMin(s, qtypes.Vec3{X: 30, Y: 0, Z: 0})
	goal.SetAbsMax(s, qtypes.Vec3{X: 46, Y: 16, Z: 16})

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
	ent.SetOrigin(s, qtypes.Vec3{X: 10, Y: 20, Z: 30})
	ent.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	ent.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})

	h, offset := s.SV_HullForEntity(ent, qtypes.Vec3{}, qtypes.Vec3{})
	if h == nil {
		t.Fatalf("SV_HullForEntity returned nil hull")
	}
	if offset != ent.Origin(s) {
		t.Fatalf("offset = %v, want %v", offset, ent.Origin(s))
	}

	start := qtypes.Vec3{X: 0, Y: 0, Z: 0}
	end := qtypes.Vec3{X: 16, Y: 0, Z: 0}
	a := s.Move(start, qtypes.Vec3{}, qtypes.Vec3{}, end, MoveType(MoveNormal), nil)
	b := s.SV_Move(start, qtypes.Vec3{}, qtypes.Vec3{}, end, MoveType(MoveNormal), nil)
	if a.Fraction != b.Fraction || a.StartSolid != b.StartSolid || a.AllSolid != b.AllSolid || a.EndPos != b.EndPos {
		t.Fatalf("SV_Move wrapper mismatch: base=%+v wrapper=%+v", a, b)
	}
}

func TestSVHullForInlineBrushModelUsesSubmodelHeadnode(t *testing.T) {
	s := newMovementTestServer()
	wm := &model.Model{
		Type:   model.ModBrush,
		Planes: []model.MPlane{{Normal: qtypes.Vec3{X: 1, Y: 0, Z: 0}, Dist: 0, Type: 0}, {Normal: qtypes.Vec3{X: 1, Y: 0, Z: 0}, Dist: 10, Type: 0}},
	}
	wm.Hulls[1] = model.Hull{
		ClipNodes: []model.MClipNode{
			{PlaneNum: 0, Children: [2]int{bsp.ContentsEmpty, bsp.ContentsSolid}},
			{PlaneNum: 1, Children: [2]int{bsp.ContentsEmpty, bsp.ContentsSolid}},
		},
		Planes:        wm.Planes,
		FirstClipNode: 0,
		LastClipNode:  1,
		ClipMins:      qtypes.Vec3{X: -16, Y: -16, Z: -24},
		ClipMaxs:      qtypes.Vec3{X: 16, Y: 16, Z: 32},
	}
	s.WorldModel = wm
	s.WorldTree = &bsp.Tree{Models: []bsp.DModel{
		{HeadNode: [bsp.MaxMapHulls]int32{0, 0, 0, 0}},
		{HeadNode: [bsp.MaxMapHulls]int32{0, 1, 1, 0}},
	}}

	ent := s.AllocEdict()
	ent.SetOrigin(s, qtypes.Vec3{})
	ent.SetSolid(s, float32(SolidBSP))
	ent.SetMoveType(s, float32(MoveTypePush))
	ent.SetModelIndex(s, 2)

	h, _ := s.SV_HullForEntity(ent, qtypes.Vec3{X: -16, Y: -16, Z: -24}, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	if h == nil {
		t.Fatal("SV_HullForEntity returned nil hull")
	}
	if h.FirstClipNode != 1 {
		t.Fatalf("first clip node = %d, want 1", h.FirstClipNode)
	}

	trace := s.clipMoveToEntity(ent, qtypes.Vec3{X: 20, Y: 0, Z: 0}, qtypes.Vec3{X: -16, Y: -16, Z: -24}, qtypes.Vec3{X: 16, Y: 16, Z: 32}, qtypes.Vec3{X: -20, Y: 0, Z: 0})
	if trace.Fraction >= 1 {
		t.Fatalf("trace fraction = %v, want collision", trace.Fraction)
	}
	if trace.EndPos.X < 9.9 || trace.EndPos.X > 10.1 {
		t.Fatalf("trace end x = %v, want about 10", trace.EndPos.X)
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
	ent.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	ent.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
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
	if !s.MoveStep(ent, qtypes.Vec3{}, true) {
		t.Fatalf("MoveStep failed on stationary grounded entity; %s", diag.String())
	}
	if ent.Origin(s) != before {
		t.Fatalf("MoveStep with zero move changed origin: before=%v after=%v", before, ent.Origin(s))
	}
}

func TestMoveToGoalRandomBranchUsesSharedCompatRNG(t *testing.T) {
	s := newMovementTestServer()

	goal := s.AllocEdict()
	goal.SetOrigin(s, qtypes.Vec3{X: 64, Y: 0, Z: 0})
	goal.SetAbsMin(s, qtypes.Vec3{X: 64, Y: 0, Z: 0})
	goal.SetAbsMax(s, qtypes.Vec3{X: 64, Y: 0, Z: 0})

	ent := s.AllocEdict()
	ent.SetFlags(s, float32(FlagFly))
	ent.SetIdealYaw(s, 90)
	ang2 := ent.Angles(s)
	ang2.Y = 90
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
	if got := ent.Origin(s); got != (qtypes.Vec3{X: 16, Y: 0, Z: 0}) {
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
	enemy.SetOrigin(s, qtypes.Vec3{X: -64, Y: -64, Z: 0})

	s.Edicts = append(s.Edicts, actor, enemy)
	s.NumEdicts = len(s.Edicts)

	s.NewChaseDir(actor, enemy, 16)

	wantX := float32(-13.10643)
	wantY := float32(-9.177243)
	got := actor.Origin(s)
	if got.X < wantX-0.01 || got.X > wantX+0.01 || got.Y < wantY-0.01 || got.Y > wantY+0.01 {
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
		{Normal: qtypes.Vec3{X: 1, Y: 0, Z: 0}, Dist: 0, Type: 0},
		{Normal: qtypes.Vec3{X: 0, Y: 0, Z: 1}, Dist: 0, Type: 2},
	}
	hull.ClipNodes = []model.MClipNode{
		{PlaneNum: 0, Children: [2]int{1, bsp.ContentsEmpty}},
		{PlaneNum: 1, Children: [2]int{bsp.ContentsEmpty, bsp.ContentsSolid}},
	}
	hull.FirstClipNode = 0
	hull.LastClipNode = 1
	hull.ClipMins = qtypes.Vec3{X: -512, Y: -512, Z: -512}
	hull.ClipMaxs = qtypes.Vec3{X: 512, Y: 512, Z: 512}

	m.Hulls[0] = hull
	m.Mins = qtypes.Vec3{X: -512, Y: -512, Z: -512}
	m.Maxs = qtypes.Vec3{X: 512, Y: 512, Z: 512}
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
	ent.SetOrigin(s, qtypes.Vec3{X: 32, Y: 0, Z: 24})
	ent.SetMins(s, qtypes.Vec3{X: -16, Y: -16, Z: -24})
	ent.SetMaxs(s, qtypes.Vec3{X: 16, Y: 16, Z: 32})
	ent.SetSolid(s, float32(SolidSlideBox))
	ent.SetMoveType(s, float32(MoveTypeStep))
	ent.SetFlags(s, float32(FlagOnGround))
	s.LinkEdict(ent, false)

	if !s.CheckBottom(ent) {
		t.Fatal("expected starting position to be fully supported")
	}

	start := ent.Origin(s)
	if s.MoveStep(ent, qtypes.Vec3{X: -20, Y: 0, Z: 0}, true) {
		t.Fatalf("MoveStep unexpectedly accepted unsupported platform step: start=%v end=%v", start, ent.Origin(s))
	}
	if got := ent.Origin(s); got != start {
		t.Fatalf("origin after rejected step = %v, want %v", got, start)
	}
}
