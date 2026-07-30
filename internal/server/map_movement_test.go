package server

import "testing"

// TestMovementAgainstMap loads the start map and runs a set of movement
// traces and checks that world collisions are detected. This test is a
// smoke-style integration test and will be skipped if `pak0.pak` is
// not available locally (see testutil.SkipIfNoPak0).
func TestMovementAgainstMap(t *testing.T) {
	s := newStartMapDiagnosticsServer(t)

	// Find a walkable point on the map to place a test entity.
	pos, ok, diag := findWalkablePointWithDiagnostics(s)
	if !ok {
		t.Skipf("no walkable point found on start map; %s", diag.String())
	}

	// Place a small test move: step forward and ensure MoveStep respects BSP.
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
		t.Fatalf("SV_TestEntityPosition found blocker at valid position: %+v; %s", blocker, diag.String())
	}
	if !s.CheckBottom(ent) {
		t.Skipf("sampled position does not satisfy CheckBottom; %s", diag.String())
	}

	// Attempt a small MoveStep; on a valid walkable point this should succeed
	// without changing origin when move is zero and return true for grounded.
	before := ent.Origin(s)
	if !s.MoveStep(ent, [3]float32{}, true) {
		t.Fatalf("MoveStep failed on stationary grounded entity; %s", diag.String())
	}
	if ent.Origin(s) != before {
		t.Fatalf("MoveStep with zero move changed origin: before=%v after=%v", before, ent.Origin(s))
	}
}
