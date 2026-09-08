package bspdec

import (
	"math"
	"testing"

	mapfile "github.com/darkliquid/ironwail-go/pkg/map"
)

func newSquareWinding() *Winding {
	return &Winding{Points: []mapfile.Vec3{vc(0, 0, 0), vc(64, 0, 0), vc(64, 64, 0), vc(0, 64, 0)}}
}

func TestClipKeepsFrontHalf(t *testing.T) {
	got := newSquareWinding().Clip(mapfile.Plane{Normal: vc(1, 0, 0), Dist: 32})
	if got == nil {
		t.Fatal("winding vanished")
	}
	if len(got.Points) != 4 {
		t.Fatalf("points = %d, want 4 (%v)", len(got.Points), got.Points)
	}
	if a := got.Area(); math.Abs(a-32*64) > 0.5 {
		t.Fatalf("area = %v, want %v", a, 32*64)
	}
	for _, p := range got.Points {
		if p.X < 32-0.01 {
			t.Fatalf("point %v behind clip plane", p)
		}
	}
}

func TestClipFullyBehindReturnsNil(t *testing.T) {
	if got := newSquareWinding().Clip(mapfile.Plane{Normal: vc(1, 0, 0), Dist: 100}); got != nil {
		t.Fatalf("expected nil, got %v", got.Points)
	}
}

func TestClipFullyInFrontReturnsReceiver(t *testing.T) {
	w := newSquareWinding()
	if got := w.Clip(mapfile.Plane{Normal: vc(1, 0, 0), Dist: -5}); got != w {
		t.Fatal("expected receiver back for unclipped winding")
	}
}

func TestBaseWindingLiesOnPlane(t *testing.T) {
	w := BaseWinding(mapfile.Plane{Normal: vc(0, 0, 1), Dist: 64})
	if w == nil || len(w.Points) != 4 {
		t.Fatalf("base winding = %v", w)
	}
	for _, p := range w.Points {
		if math.Abs(p.Z-64) > 0.01 {
			t.Fatalf("point %v off plane z=64", p)
		}
	}
	if a := w.Area(); a < 1e9 {
		t.Fatalf("base winding suspiciously small: %v", a)
	}
}

func TestAreaIsOrientationIndependent(t *testing.T) {
	a := newSquareWinding()
	rev := &Winding{Points: []mapfile.Vec3{a.Points[3], a.Points[2], a.Points[1], a.Points[0]}}
	if math.Abs(a.Area()-rev.Area()) > 1e-9 || a.Area() != 64*64 {
		t.Fatalf("areas %v vs %v", a.Area(), rev.Area())
	}
}

func TestCentroid(t *testing.T) {
	c := newSquareWinding().Centroid()
	if c != vc(32, 32, 0) {
		t.Fatalf("centroid = %v, want [32 32 0]", c)
	}
}