package types

import (
	"math"
	"testing"
)

func TestPlane(t *testing.T) {
	norm := Vec3{X: 0, Y: 0, Z: 1}
	dist := float32(10)
	p := NewPlane(norm, dist)

	if p.Normal != norm || p.Dist != dist {
		t.Errorf("NewPlane mismatch: got %+v", p)
	}
	if p.Type != PlaneZ {
		t.Errorf("Expected PlaneZ type, got %d", p.Type)
	}

	// Plane from points
	p1 := Vec3{X: 0, Y: 0, Z: 5}
	p2 := Vec3{X: 10, Y: 0, Z: 5}
	p3 := Vec3{X: 0, Y: 10, Z: 5}
	pFromPts := PlaneFromPoints(p1, p2, p3)
	if math.Abs(float64(pFromPts.Normal.Z-1)) > 0.0001 || math.Abs(float64(pFromPts.Dist-5)) > 0.0001 {
		t.Errorf("PlaneFromPoints unexpected result: %+v", pFromPts)
	}

	// DistanceToPoint
	ptAbove := Vec3{X: 5, Y: 5, Z: 15}
	if d := p.DistanceToPoint(ptAbove); d != 5 {
		t.Errorf("DistanceToPoint expected 5, got %f", d)
	}

	ptBelow := Vec3{X: 5, Y: 5, Z: 0}
	if d := p.DistanceToPoint(ptBelow); d != -10 {
		t.Errorf("DistanceToPoint expected -10, got %f", d)
	}

	ptOn := Vec3{X: 5, Y: 5, Z: 10}
	if d := p.DistanceToPoint(ptOn); d != 0 {
		t.Errorf("DistanceToPoint expected 0, got %f", d)
	}

	// PointOnSide
	if side := p.PointOnSide(ptAbove, 0.01); side != 1 {
		t.Errorf("PointOnSide expected 1 (front), got %d", side)
	}
	if side := p.PointOnSide(ptBelow, 0.01); side != -1 {
		t.Errorf("PointOnSide expected -1 (back), got %d", side)
	}
	if side := p.PointOnSide(ptOn, 0.01); side != 0 {
		t.Errorf("PointOnSide expected 0 (on), got %d", side)
	}

	// Project point onto plane
	proj := p.Project(ptAbove)
	if proj != (Vec3{X: 5, Y: 5, Z: 10}) {
		t.Errorf("Project expected (5, 5, 10), got %+v", proj)
	}

	// Reflect velocity vector
	vel := Vec3{X: 10, Y: 0, Z: -20}
	reflected := p.Reflect(vel, 1.0)
	if reflected != (Vec3{X: 10, Y: 0, Z: 20}) {
		t.Errorf("Reflect expected (10, 0, 20), got %+v", reflected)
	}
}
