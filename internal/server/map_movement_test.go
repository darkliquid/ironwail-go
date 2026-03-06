package server

import (
	"path/filepath"
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
	"github.com/ironwail/ironwail-go/internal/fs"
	"github.com/ironwail/ironwail-go/internal/testutil"
)

// TestMovementAgainstMap loads the start map and runs a set of movement
// traces and checks that world collisions are detected. This test is a
// smoke-style integration test and will be skipped if `pak0.pak` is
// not available locally (see testutil.SkipIfNoPak0).
func TestMovementAgainstMap(t *testing.T) {
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

	// Find a walkable point on the map to place a test entity.
	pos, ok := findWalkablePoint(s)
	if !ok {
		t.Skip("no walkable point found on start map")
	}

	// Place a small test move: step forward and ensure MoveStep respects BSP.
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
		t.Fatalf("SV_TestEntityPosition found blocker at valid position: %+v", blocker)
	}

	// Attempt a small MoveStep; on a valid walkable point this should succeed
	// without changing origin when move is zero and return true for grounded.
	before := ent.Vars.Origin
	if !s.MoveStep(ent, [3]float32{}, true) {
		t.Fatalf("MoveStep failed on stationary grounded entity")
	}
	if ent.Vars.Origin != before {
		t.Fatalf("MoveStep with zero move changed origin: before=%v after=%v", before, ent.Vars.Origin)
	}

	// Test downward trace: ensure SV_Move can find the floor below this point
	start := pos
	start[2] = pos[2] + 64
	end := pos
	end[2] = pos[2] - 512
	trace := s.SV_Move(start, [3]float32{}, [3]float32{}, end, MoveType(MoveNoMonsters), nil)
	if trace.Fraction == 1 || trace.AllSolid {
		t.Fatalf("expected floor trace to hit something, got fraction=%v allSolid=%v", trace.Fraction, trace.AllSolid)
	}
	if trace.Entity == nil {
		t.Fatalf("expected trace to report an entity (world), got nil")
	}
	if trace.Entity == nil {
		t.Fatalf("trace entity nil; contents: %v", s.PointContents(trace.EndPos))
	}
	// Ensure the hit contents are not empty
	if s.PointContents(trace.EndPos) == bsp.ContentsEmpty {
		t.Fatalf("trace ended in empty space at %v", trace.EndPos)
	}
}
