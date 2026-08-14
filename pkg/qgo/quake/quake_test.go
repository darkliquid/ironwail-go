package quake

import (
	"testing"
)

func TestQuakeVec3Methods(t *testing.T) {
	a := Vec3{1, 2, 3}
	b := Vec3{4, 5, 6}

	if a.Add(b) != (Vec3{5, 7, 9}) {
		t.Errorf("Add failed")
	}
	if b.Sub(a) != (Vec3{3, 3, 3}) {
		t.Errorf("Sub failed")
	}
	if a.Mul(2) != (Vec3{2, 4, 6}) || a.Scale(2) != (Vec3{2, 4, 6}) {
		t.Errorf("Mul/Scale failed")
	}
	if (Vec3{2, 4, 6}).Div(2) != a {
		t.Errorf("Div failed")
	}
	if a.Neg() != (Vec3{-1, -2, -3}) || a.Negate() != (Vec3{-1, -2, -3}) {
		t.Errorf("Neg/Negate failed")
	}
	if a.Dot(b) != 32 {
		t.Errorf("Dot expected 32, got %f", a.Dot(b))
	}
	if a.LenSq() != 14 || a.LengthSq() != 14 {
		t.Errorf("LenSq expected 14, got %f", a.LenSq())
	}
	if (Vec3{3, 4, 0}).Len() != 5 || (Vec3{3, 4, 0}).Length() != 5 {
		t.Errorf("Len expected 5, got %f", (Vec3{3, 4, 0}).Len())
	}
	if (Vec3{3, 0, 0}).Distance(Vec3{0, 4, 0}) != 5 || (Vec3{3, 0, 0}).Dist(Vec3{0, 4, 0}) != 5 {
		t.Errorf("Distance expected 5, got %f", (Vec3{3, 0, 0}).Distance(Vec3{0, 4, 0}))
	}
	if (Vec3{3, 0, 0}).DistanceSq(Vec3{0, 4, 0}) != 25 {
		t.Errorf("DistanceSq expected 25, got %f", (Vec3{3, 0, 0}).DistanceSq(Vec3{0, 4, 0}))
	}
	norm := (Vec3{10, 0, 0}).Normalize()
	if norm != (Vec3{1, 0, 0}) {
		t.Errorf("Normalize expected (1,0,0), got %+v", norm)
	}
	ma := a.MA(2, b)
	if ma != (Vec3{9, 12, 15}) || a.MultiplyAdd(2, b) != (Vec3{9, 12, 15}) {
		t.Errorf("MA failed: %+v", ma)
	}
	if a.X() != 1 || a.Y() != 2 || a.Z() != 3 {
		t.Errorf("Accessors failed")
	}
	if !a.Equals(Vec3{1, 2, 3}) {
		t.Errorf("Equals failed")
	}
	if !a.ApproxEqual(Vec3{1.00001, 2.00001, 2.99999}, 0.001) {
		t.Errorf("ApproxEqual failed")
	}
}

func TestQuakeBox3(t *testing.T) {
	box := NewBox3(Vec3{-10, -10, -10}, Vec3{10, 10, 10})
	if box.Center() != (Vec3{0, 0, 0}) {
		t.Errorf("Center expected (0,0,0), got %+v", box.Center())
	}
	if box.Size() != (Vec3{20, 20, 20}) {
		t.Errorf("Size expected (20,20,20), got %+v", box.Size())
	}
	if !box.ContainsPoint(Vec3{0, 5, -5}) {
		t.Errorf("ContainsPoint failed")
	}
	if !box.Intersects(NewBox3(Vec3{5, 5, 5}, Vec3{15, 15, 15})) {
		t.Errorf("Intersects failed")
	}
	expanded := box.Expand(5)
	if expanded.Min != (Vec3{-15, -15, -15}) || expanded.Max != (Vec3{15, 15, 15}) {
		t.Errorf("Expand failed: %+v", expanded)
	}
}

func TestQuakePlane(t *testing.T) {
	p := NewPlane(Vec3{0, 0, 1}, 10)
	if p.DistanceToPoint(Vec3{5, 5, 15}) != 5 {
		t.Errorf("DistanceToPoint expected 5, got %f", p.DistanceToPoint(Vec3{5, 5, 15}))
	}
	if p.PointOnSide(Vec3{5, 5, 15}, 0.01) != 1 {
		t.Errorf("PointOnSide expected 1")
	}
	if p.PointOnSide(Vec3{5, 5, 0}, 0.01) != -1 {
		t.Errorf("PointOnSide expected -1")
	}
}
