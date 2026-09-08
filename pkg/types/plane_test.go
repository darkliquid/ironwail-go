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
	pFromPts := Plane32FromPoints(p1, p2, p3)
	if math.Abs(float64(pFromPts.Normal.Z-1)) > 0.0001 || math.Abs(float64(pFromPts.Dist-5)) > 0.0001 {
		t.Errorf("Plane32FromPoints unexpected result: %+v", pFromPts)
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

func TestPlane64FromPointsAndClassification(t *testing.T) {
	// Axial +Z plane
	p0 := Vec3d{X: 64, Y: 0, Z: 0}
	p1 := Vec3d{X: 0, Y: 0, Z: 0}
	p2 := Vec3d{X: 0, Y: 64, Z: 0}
	planeZ, length := PlaneFromPoints(p0, p1, p2)
	if length <= 0 {
		t.Fatalf("expected positive length, got %v", length)
	}
	if planeZ.Normal != (Vec3d{X: 0, Y: 0, Z: 1}) {
		t.Fatalf("expected Normal {0 0 1}, got %v", planeZ.Normal)
	}
	if planeZ.Dist != 0 {
		t.Fatalf("expected Dist 0, got %v", planeZ.Dist)
	}
	if planeZ.Type != PlaneZ {
		t.Fatalf("expected PlaneZ, got %v", planeZ.Type)
	}

	// Axial -Z plane with distance
	p0 = Vec3d{X: 0, Y: 64, Z: 10}
	p1 = Vec3d{X: 0, Y: 0, Z: 10}
	p2 = Vec3d{X: 64, Y: 0, Z: 10}
	planeNegZ, length := PlaneFromPoints(p0, p1, p2)
	if length <= 0 {
		t.Fatalf("expected positive length, got %v", length)
	}
	if planeNegZ.Normal != (Vec3d{X: 0, Y: 0, Z: -1}) {
		t.Fatalf("expected Normal {0 0 -1}, got %v", planeNegZ.Normal)
	}
	if planeNegZ.Dist != -10 {
		t.Fatalf("expected Dist -10, got %v", planeNegZ.Dist)
	}
	if planeNegZ.Type != PlaneZ {
		t.Fatalf("expected PlaneZ, got %v", planeNegZ.Type)
	}

	// Axial +X plane with distance
	p0 = Vec3d{X: 64, Y: 0, Z: 0}
	p1 = Vec3d{X: 64, Y: 0, Z: 64}
	p2 = Vec3d{X: 64, Y: 64, Z: 0}
	planeX, length := PlaneFromPoints(p0, p1, p2)
	if length <= 0 {
		t.Fatalf("expected positive length, got %v", length)
	}
	if planeX.Normal != (Vec3d{X: 1, Y: 0, Z: 0}) {
		t.Fatalf("expected Normal {1 0 0}, got %v", planeX.Normal)
	}
	if planeX.Dist != 64 {
		t.Fatalf("expected Dist 64, got %v", planeX.Dist)
	}
	if planeX.Type != PlaneX {
		t.Fatalf("expected PlaneX, got %v", planeX.Type)
	}

	// Axial -X plane
	planeNegX, _ := PlaneFromPoints(p0, p2, p1)
	if planeNegX.Normal != (Vec3d{X: -1, Y: 0, Z: 0}) {
		t.Fatalf("expected Normal {-1 0 0}, got %v", planeNegX.Normal)
	}
	if planeNegX.Dist != -64 {
		t.Fatalf("expected Dist -64, got %v", planeNegX.Dist)
	}
	if planeNegX.Type != PlaneX {
		t.Fatalf("expected PlaneX, got %v", planeNegX.Type)
	}

	// Axial +Y plane with distance
	p0 = Vec3d{X: 0, Y: 32, Z: 64}
	p1 = Vec3d{X: 0, Y: 32, Z: 0}
	p2 = Vec3d{X: 64, Y: 32, Z: 0}
	planeY, length := PlaneFromPoints(p0, p1, p2)
	if length <= 0 {
		t.Fatalf("expected positive length, got %v", length)
	}
	if planeY.Normal != (Vec3d{X: 0, Y: 1, Z: 0}) {
		t.Fatalf("expected Normal {0 1 0}, got %v", planeY.Normal)
	}
	if planeY.Dist != 32 {
		t.Fatalf("expected Dist 32, got %v", planeY.Dist)
	}
	if planeY.Type != PlaneY {
		t.Fatalf("expected PlaneY, got %v", planeY.Type)
	}

	// Axial -Y plane
	planeNegY, _ := PlaneFromPoints(p2, p1, p0)
	if planeNegY.Normal != (Vec3d{X: 0, Y: -1, Z: 0}) {
		t.Fatalf("expected Normal {0 -1 0}, got %v", planeNegY.Normal)
	}
	if planeNegY.Dist != -32 {
		t.Fatalf("expected Dist -32, got %v", planeNegY.Dist)
	}
	if planeNegY.Type != PlaneY {
		t.Fatalf("expected PlaneY, got %v", planeNegY.Type)
	}

	// Non-axial PlaneAnyX (dominant X)
	p0 = Vec3d{X: 0, Y: 1, Z: -1}
	p1 = Vec3d{X: 0, Y: 0, Z: 0}
	p2 = Vec3d{X: -1, Y: 1, Z: 1}
	planeAnyX, _ := PlaneFromPoints(p0, p1, p2)
	if planeAnyX.Type != PlaneAnyX {
		t.Fatalf("expected PlaneAnyX, got %v", planeAnyX.Type)
	}

	// Non-axial PlaneAnyY (dominant Y)
	p0 = Vec3d{X: -1, Y: 0, Z: 1}
	p1 = Vec3d{X: 0, Y: 0, Z: 0}
	p2 = Vec3d{X: 1, Y: -1, Z: 1}
	planeAnyY, _ := PlaneFromPoints(p0, p1, p2)
	if planeAnyY.Type != PlaneAnyY {
		t.Fatalf("expected PlaneAnyY, got %v", planeAnyY.Type)
	}

	// Non-axial PlaneAnyZ (dominant Z)
	p0 = Vec3d{X: 1, Y: -1, Z: 0}
	p1 = Vec3d{X: 0, Y: 0, Z: 0}
	p2 = Vec3d{X: 1, Y: 1, Z: -1}
	planeAnyZ, _ := PlaneFromPoints(p0, p1, p2)
	if planeAnyZ.Type != PlaneAnyZ {
		t.Fatalf("expected PlaneAnyZ, got %v", planeAnyZ.Type)
	}

	// Degenerate plane (collinear points)
	p0 = Vec3d{X: 0, Y: 0, Z: 0}
	p1 = Vec3d{X: 1, Y: 1, Z: 1}
	p2 = Vec3d{X: 2, Y: 2, Z: 2}
	planeDegen, degenLen := PlaneFromPoints(p0, p1, p2)
	if degenLen != 0 {
		t.Fatalf("expected degenerate length 0, got %v", degenLen)
	}
	if planeDegen != (Plane64{}) {
		t.Fatalf("expected zero Plane64 for degenerate points, got %v", planeDegen)
	}

	// ClassifyPlaneType64 unit tests
	if got := ClassifyPlaneType64(Vec3d{X: 1, Y: 0, Z: 0}); got != PlaneX {
		t.Fatalf("ClassifyPlaneType64(1,0,0) = %v, want PlaneX", got)
	}
	if got := ClassifyPlaneType64(Vec3d{X: -1, Y: 0, Z: 0}); got != PlaneX {
		t.Fatalf("ClassifyPlaneType64(-1,0,0) = %v, want PlaneX", got)
	}
	if got := ClassifyPlaneType64(Vec3d{X: 0, Y: 1, Z: 0}); got != PlaneY {
		t.Fatalf("ClassifyPlaneType64(0,1,0) = %v, want PlaneY", got)
	}
	if got := ClassifyPlaneType64(Vec3d{X: 0, Y: -1, Z: 0}); got != PlaneY {
		t.Fatalf("ClassifyPlaneType64(0,-1,0) = %v, want PlaneY", got)
	}
	if got := ClassifyPlaneType64(Vec3d{X: 0, Y: 0, Z: 1}); got != PlaneZ {
		t.Fatalf("ClassifyPlaneType64(0,0,1) = %v, want PlaneZ", got)
	}
	if got := ClassifyPlaneType64(Vec3d{X: 0, Y: 0, Z: -1}); got != PlaneZ {
		t.Fatalf("ClassifyPlaneType64(0,0,-1) = %v, want PlaneZ", got)
	}
	if got := ClassifyPlaneType64(Vec3d{X: 0.8, Y: 0.5, Z: 0.3}); got != PlaneAnyX {
		t.Fatalf("ClassifyPlaneType64(0.8,0.5,0.3) = %v, want PlaneAnyX", got)
	}
	if got := ClassifyPlaneType64(Vec3d{X: 0.3, Y: 0.8, Z: 0.5}); got != PlaneAnyY {
		t.Fatalf("ClassifyPlaneType64(0.3,0.8,0.5) = %v, want PlaneAnyY", got)
	}
	if got := ClassifyPlaneType64(Vec3d{X: 0.3, Y: 0.5, Z: 0.8}); got != PlaneAnyZ {
		t.Fatalf("ClassifyPlaneType64(0.3,0.5,0.8) = %v, want PlaneAnyZ", got)
	}
}

func TestPlaneEqual(t *testing.T) {
	base := Plane64{
		Normal: Vec3d{X: 0, Y: 0, Z: 1},
		Dist:   10.0,
		Type:   PlaneZ,
	}

	// Exact match
	if !PlaneEqual(base, base, 1e-6) {
		t.Fatal("expected exact same plane to be equal")
	}

	// Within epsilon
	near := Plane64{
		Normal: Vec3d{X: 1e-5, Y: -1e-5, Z: 1.0 - 1e-5},
		Dist:   10.0 + 1e-5,
		Type:   PlaneZ,
	}
	if !PlaneEqual(base, near, 1e-4) {
		t.Fatal("expected plane within epsilon to be equal")
	}

	// Exceeding epsilon in Dist
	farDist := Plane64{
		Normal: Vec3d{X: 0, Y: 0, Z: 1},
		Dist:   10.01,
		Type:   PlaneZ,
	}
	if PlaneEqual(base, farDist, 1e-4) {
		t.Fatal("expected plane with distant Dist to not be equal")
	}

	// Exceeding epsilon in Normal
	farNorm := Plane64{
		Normal: Vec3d{X: 0.01, Y: 0, Z: 1},
		Dist:   10.0,
		Type:   PlaneZ,
	}
	if PlaneEqual(base, farNorm, 1e-4) {
		t.Fatal("expected plane with different Normal to not be equal")
	}
}

func TestPlane64Equal(t *testing.T) {
	TestPlaneEqual(t)
}

func TestPlane64PointDistance(t *testing.T) {
	p := Plane64{
		Normal: Vec3d{X: 0, Y: 0, Z: 1},
		Dist:   10.0,
		Type:   PlaneZ,
	}

	ptOn := Vec3d{X: 5, Y: 5, Z: 10}
	if d := p.DistToPoint(ptOn); math.Abs(d) > 1e-9 {
		t.Fatalf("DistToPoint on plane: expected 0, got %v", d)
	}

	ptFront := Vec3d{X: 5, Y: 5, Z: 15}
	if d := p.DistToPoint(ptFront); math.Abs(d-5) > 1e-9 {
		t.Fatalf("DistToPoint in front: expected 5, got %v", d)
	}

	ptBack := Vec3d{X: 5, Y: 5, Z: 0}
	if d := p.DistToPoint(ptBack); math.Abs(d-(-10)) > 1e-9 {
		t.Fatalf("DistToPoint behind: expected -10, got %v", d)
	}

	// Also check DistanceToPoint alias
	if d := p.DistanceToPoint(ptFront); math.Abs(d-5) > 1e-9 {
		t.Fatalf("DistanceToPoint: expected 5, got %v", d)
	}

	// PointOnSide tests
	if side := p.PointOnSide(ptOn, 1e-6); side != 0 {
		t.Fatalf("PointOnSide ptOn: expected 0, got %d", side)
	}
	if side := p.PointOnSide(ptFront, 1e-6); side != 1 {
		t.Fatalf("PointOnSide ptFront: expected 1, got %d", side)
	}
	if side := p.PointOnSide(ptBack, 1e-6); side != -1 {
		t.Fatalf("PointOnSide ptBack: expected -1, got %d", side)
	}

	// NewPlane64 and Plane64FromPoints alias
	np := NewPlane64(Vec3d{X: 0, Y: 0, Z: 2}, 10)
	if np.Normal != (Vec3d{X: 0, Y: 0, Z: 1}) || np.Dist != 10 || np.Type != PlaneZ {
		t.Fatalf("NewPlane64 unexpected result: %v", np)
	}

	p0 := Vec3d{X: 64, Y: 0, Z: 0}
	p1 := Vec3d{X: 0, Y: 0, Z: 0}
	p2 := Vec3d{X: 0, Y: 64, Z: 0}
	aliasPlane, _ := Plane64FromPoints(p0, p1, p2)
	if aliasPlane.Normal != (Vec3d{X: 0, Y: 0, Z: 1}) {
		t.Fatalf("Plane64FromPoints unexpected normal: %v", aliasPlane)
	}

	// NaN input check
	nanP0 := Vec3d{X: math.NaN(), Y: 0, Z: 0}
	nanPlane, nanLen := PlaneFromPoints(nanP0, p1, p2)
	if nanLen != 0 || nanPlane != (Plane64{}) {
		t.Fatalf("PlaneFromPoints with NaN input: expected (Plane64{}, 0), got (%v, %v)", nanPlane, nanLen)
	}
}
