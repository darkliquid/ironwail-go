package types

import (
	"testing"
)

func TestBox3(t *testing.T) {
	min := Vec3{X: -10, Y: -10, Z: -10}
	max := Vec3{X: 10, Y: 10, Z: 10}
	box := NewBox3(min, max)

	if box.Min != min || box.Max != max {
		t.Errorf("NewBox3 mismatch: got %+v", box)
	}

	center := box.Center()
	if center != (Vec3{X: 0, Y: 0, Z: 0}) {
		t.Errorf("Center expected (0,0,0), got %+v", center)
	}

	size := box.Size()
	if size != (Vec3{X: 20, Y: 20, Z: 20}) {
		t.Errorf("Size expected (20,20,20), got %+v", size)
	}

	extents := box.Extents()
	if extents != (Vec3{X: 10, Y: 10, Z: 10}) {
		t.Errorf("Extents expected (10,10,10), got %+v", extents)
	}

	if box.IsEmpty() {
		t.Errorf("Expected box to not be empty")
	}

	emptyBox := Box3{Min: Vec3{X: 10, Y: 10, Z: 10}, Max: Vec3{X: -10, Y: -10, Z: -10}}
	if !emptyBox.IsEmpty() {
		t.Errorf("Expected emptyBox.IsEmpty() to be true")
	}

	// ContainsPoint
	if !box.ContainsPoint(Vec3{X: 0, Y: 5, Z: -5}) {
		t.Errorf("Expected box to contain (0, 5, -5)")
	}
	if box.ContainsPoint(Vec3{X: 15, Y: 0, Z: 0}) {
		t.Errorf("Expected box to not contain (15, 0, 0)")
	}

	// ContainsBox & Intersects
	inner := NewBox3(Vec3{X: -5, Y: -5, Z: -5}, Vec3{X: 5, Y: 5, Z: 5})
	if !box.ContainsBox(inner) {
		t.Errorf("Expected box to contain inner box")
	}
	if inner.ContainsBox(box) {
		t.Errorf("Inner box should not contain outer box")
	}

	overlapping := NewBox3(Vec3{X: 5, Y: 5, Z: 5}, Vec3{X: 15, Y: 15, Z: 15})
	if !box.Intersects(overlapping) {
		t.Errorf("Expected box to intersect overlapping box")
	}

	separate := NewBox3(Vec3{X: 20, Y: 20, Z: 20}, Vec3{X: 30, Y: 30, Z: 30})
	if box.Intersects(separate) {
		t.Errorf("Expected box to not intersect separate box")
	}

	// Expand
	expanded := box.Expand(5)
	if expanded.Min != (Vec3{X: -15, Y: -15, Z: -15}) || expanded.Max != (Vec3{X: 15, Y: 15, Z: 15}) {
		t.Errorf("Expand unexpected result: %+v", expanded)
	}

	// Union & UnionPoint
	unionBox := box.Union(separate)
	if unionBox.Min != (Vec3{X: -10, Y: -10, Z: -10}) || unionBox.Max != (Vec3{X: 30, Y: 30, Z: 30}) {
		t.Errorf("Union unexpected result: %+v", unionBox)
	}

	pointUnion := box.UnionPoint(Vec3{X: 25, Y: -25, Z: 0})
	if pointUnion.Min != (Vec3{X: -10, Y: -25, Z: -10}) || pointUnion.Max != (Vec3{X: 25, Y: 10, Z: 10}) {
		t.Errorf("UnionPoint unexpected result: %+v", pointUnion)
	}

	// Translate
	translated := box.Translate(Vec3{X: 5, Y: -5, Z: 10})
	if translated.Min != (Vec3{X: -5, Y: -15, Z: 0}) || translated.Max != (Vec3{X: 15, Y: 5, Z: 20}) {
		t.Errorf("Translate unexpected result: %+v", translated)
	}

	// ClosestPoint
	closest := box.ClosestPoint(Vec3{X: 20, Y: 0, Z: -20})
	if closest != (Vec3{X: 10, Y: 0, Z: -10}) {
		t.Errorf("ClosestPoint expected (10, 0, -10), got %+v", closest)
	}

	// DistanceToPoint
	dist := box.DistanceToPoint(Vec3{X: 20, Y: 0, Z: 0})
	if dist != 10 {
		t.Errorf("DistanceToPoint expected 10, got %f", dist)
	}
}

func TestBox3FromCenterSize(t *testing.T) {
	center := Vec3{X: 10, Y: 20, Z: 30}
	size := Vec3{X: 4, Y: 6, Z: 8}
	box := Box3FromCenterSize(center, size)

	if box.Min != (Vec3{X: 8, Y: 17, Z: 26}) {
		t.Errorf("Expected Min (8,17,26), got %+v", box.Min)
	}
	if box.Max != (Vec3{X: 12, Y: 23, Z: 34}) {
		t.Errorf("Expected Max (12,23,34), got %+v", box.Max)
	}
}
