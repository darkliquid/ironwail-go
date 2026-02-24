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
	if ent.Vars.Angles[1] != 355 {
		t.Fatalf("angles yaw = %v, want 355", ent.Vars.Angles[1])
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

func findWalkablePoint(s *Server) ([3]float32, bool) {
	wm, ok := s.WorldModel.(*model.Model)
	if !ok || wm == nil {
		return [3]float32{}, false
	}

	for xi := 1; xi < 15; xi++ {
		x := wm.Mins[0] + (wm.Maxs[0]-wm.Mins[0])*(float32(xi)/16)
		for yi := 1; yi < 15; yi++ {
			y := wm.Mins[1] + (wm.Maxs[1]-wm.Mins[1])*(float32(yi)/16)
			start := [3]float32{x, y, wm.Maxs[2] - 8}
			if s.PointContents(start) == bsp.ContentsSolid {
				continue
			}
			end := [3]float32{x, y, wm.Mins[2] - 256}
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
