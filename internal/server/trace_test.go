package server

import "testing"

func TestMoveAgainstBoxWorld(t *testing.T) {
	s := newMovementTestServer()

	// Configure the world edict as a simple box from -10..-10..-10 to 10..10..10
	world := s.Edicts[0]
	world.Vars.Mins = [3]float32{-10, -10, -10}
	world.Vars.Maxs = [3]float32{10, 10, 10}
	// Use non-SolidBSP so hullForEntity falls back to box hull
	world.Vars.Solid = float32(SolidBBox)

	// Move from outside towards the box center
	start := [3]float32{-20, 0, 0}
	end := [3]float32{0, 0, 0}
	trace := s.Move(start, [3]float32{}, [3]float32{}, end, MoveNormal, nil)

	if trace.Fraction == 1 {
		t.Fatalf("expected collision fraction < 1, got 1 (no collision)")
	}
	if trace.Entity == nil {
		t.Fatalf("expected entity collision, got nil")
	}
}

func TestMoveThroughEmptySpace(t *testing.T) {
	s := newMovementTestServer()

	// World still exists but set it to a distant box so path is empty
	world := s.Edicts[0]
	world.Vars.Mins = [3]float32{1000, 1000, 1000}
	world.Vars.Maxs = [3]float32{1010, 1010, 1010}
	world.Vars.Solid = float32(SolidBBox)

	start := [3]float32{0, 0, 0}
	end := [3]float32{16, 0, 0}
	trace := s.Move(start, [3]float32{}, [3]float32{}, end, MoveNormal, nil)

	if trace.Fraction != 1 {
		t.Fatalf("expected no collision (fraction==1), got %v", trace.Fraction)
	}
}
