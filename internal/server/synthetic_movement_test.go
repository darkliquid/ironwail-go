package server

import (
	"testing"

	"github.com/ironwail/ironwail-go/internal/bsp"
)

func TestSyntheticMovement(t *testing.T) {
	s := NewServer()
	// Use a tiny deterministic world: flat ground at z=0
	s.WorldModel = CreateSyntheticWorldModel()
	// Make the world entity use BSP clipping so traces consult WorldModel
	if s.Edicts != nil && len(s.Edicts) > 0 && s.Edicts[0] != nil && s.Edicts[0].Vars != nil {
		s.Edicts[0].Vars.Solid = float32(SolidBSP)
	}
	s.ClearWorld()

	// Allocate an entity and place it above the ground
	e := s.AllocEdict()
	if e == nil {
		t.Fatal("failed to alloc edict")
	}

	e.Vars.Origin = [3]float32{0, 0, 16}
	// Use a small point-like hull so the server will pick hull 0.
	e.Vars.Mins = [3]float32{-1, -1, 0}
	e.Vars.Maxs = [3]float32{1, 1, 56}
	e.Vars.Solid = float32(SolidBSP)

	// Link the entity so it participates in area checks
	s.LinkEdict(e, true)

	// The point above ground should be empty
	if s.PointContents(e.Vars.Origin) == bsp.ContentsSolid {
		t.Fatalf("expected empty at origin, got solid")
	}

	// Step forward along +Y by 64 units
	ok := s.StepDirection(e, 90, 64)
	if !ok {
		t.Fatalf("StepDirection failed")
	}

	// Now test a downward trace from high above to the ground
	start := [3]float32{0, 0, 256}
	end := [3]float32{0, 0, -256}
	trace := s.SV_Move(start, [3]float32{}, [3]float32{}, end, MoveType(MoveNormal), nil)
	if trace.Fraction == 1 {
		t.Fatalf("expected trace to hit ground, got fraction=1")
	}
	if !(trace.EndPos[2] >= -DistEpsilon && trace.EndPos[2] <= DistEpsilon) {
		t.Fatalf("expected end z==0 within epsilon, got %v", trace.EndPos[2])
	}

	// Ensure PointContents on the hit position is solid (below plane)
	below := [3]float32{0, 0, -1}
	if s.PointContents(below) != bsp.ContentsSolid {
		t.Fatalf("expected solid below ground")
	}
}
