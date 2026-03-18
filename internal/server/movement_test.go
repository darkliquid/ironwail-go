package server

import (
	"path/filepath"
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/model"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

func newMovementTestServer() *Server {
	return &Server{
		Gravity:     800,
		MaxVelocity: 2000,
		FrameTime:   0.1,
		Edicts:      []*Edict{{Vars: &EntVars{}}},
		NumEdicts:   1,
	}
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
	mins, maxs, ok := s.modelBounds(s.ModelName)
	if !ok {
		return [3]float32{}, false
	}

	for xi := 1; xi < 15; xi++ {
		x := mins[0] + (maxs[0]-mins[0])*(float32(xi)/16)
		for yi := 1; yi < 15; yi++ {
			y := mins[1] + (maxs[1]-mins[1])*(float32(yi)/16)
			start := [3]float32{x, y, maxs[2] - 8}
			if s.PointContents(start) == bsp.ContentsSolid {
				continue
			}
			end := [3]float32{x, y, mins[2] - 256}
			trace := s.SV_Move(start, [3]float32{}, [3]float32{}, end, MoveType(MoveNoMonsters), nil)
			if trace.Fraction == 1 || trace.AllSolid {
				continue
			}
			pos := trace.EndPos
			pos[2] += 24
			if s.PointContents(pos) == bsp.ContentsEmpty {
				return pos, true
			}
		}
	}

	return [3]float32{}, false
}

func TestMovementOnSpawnedMap(t *testing.T) {
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

	pos, ok := findWalkablePoint(s)
	if !ok {
		t.Skip("no walkable point found on start map")
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
		t.Fatalf("SV_TestEntityPosition found blocker at valid position")
	}

	h, _ := s.SV_HullForEntity(ent, ent.Vars.Mins, ent.Vars.Maxs)
	if h == nil {
		t.Fatalf("SV_HullForEntity returned nil on spawned map")
	}

	if !s.CheckBottom(ent) {
		t.Skip("sampled position does not satisfy CheckBottom")
	}

	before := ent.Vars.Origin
	if !s.MoveStep(ent, [3]float32{}, true) {
		t.Fatalf("MoveStep failed on stationary grounded entity")
	}
	if ent.Vars.Origin != before {
		t.Fatalf("MoveStep with zero move changed origin: before=%v after=%v", before, ent.Vars.Origin)
	}
}
