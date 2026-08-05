package decal

import (
	"math"
	"testing"
)

// markStub is a minimal MarkEntity for tests.
type markStub struct {
	origin   [3]float32
	normal   [3]float32
	size     float32
	rotation float32
}

func (m markStub) DecalOrigin() [3]float32   { return m.origin }
func (m markStub) DecalNormal() [3]float32   { return m.normal }
func (m markStub) DecalSize() float32        { return m.size }
func (m markStub) DecalRotation() float32    { return m.rotation }

func TestBuildQuadFloorFacingUp(t *testing.T) {
	got, ok := BuildQuad(markStub{
		origin:   [3]float32{100, 200, 300},
		normal:   [3]float32{0, 0, 1},
		size:     16,
		rotation: 0,
	})
	if !ok {
		t.Fatal("BuildQuad = !ok, want ok")
	}
	// Quad centered at origin + normal*0.05, half-size 8.
	center := [3]float32{100, 200, 300.05}
	for _, c := range got {
		if math.Abs(float64(c[0]-center[0])) > 8.001 || math.Abs(float64(c[1]-center[1])) > 8.001 {
			t.Fatalf("corner %v outside expected square around %v (size 16)", c, center)
		}
		if math.Abs(float64(c[2]-center[2])) > 0.001 {
			t.Fatalf("corner %v Z = %v, want %.3f (flat on plane)", c, c[2], center[2])
		}
	}
}

func TestBuildQuadDefaultNormalUp(t *testing.T) {
	// Direct zero normal is rejected by Normalize3 inside BuildQuad; callers
	// (PrepareDraws' legacy root adapter) default normals before building.
	if _, ok := BuildQuad(markStub{
		origin:   [3]float32{10, 20, 30},
		size:     8,
		rotation: 0,
	}); ok {
		t.Fatal("BuildQuad with zero normal = ok, want !ok")
	}
}

func TestSystemLifetime(t *testing.T) {
	s := NewSystem()
	mark := markStub{origin: [3]float32{1, 2, 3}, size: 8}
	s.AddMark(mark, 1.0, 0)
	if s.ActiveCount() != 1 {
		t.Fatalf("ActiveCount after add = %d, want 1", s.ActiveCount())
	}
	s.Run(0.5)
	if s.ActiveCount() != 1 {
		t.Fatalf("ActiveCount at t=0.5 = %d, want 1", s.ActiveCount())
	}
	s.Run(2.0)
	if s.ActiveCount() != 0 {
		t.Fatalf("ActiveCount at t=2.0 = %d, want 0", s.ActiveCount())
	}
}

func TestSystemIgnoresZeroSizeAndNonPositiveLifetime(t *testing.T) {
	s := NewSystem()
	s.AddMark(markStub{origin: [3]float32{1, 2, 3}, size: 0}, 1, 0)
	s.AddMark(markStub{origin: [3]float32{1, 2, 3}, size: 8}, 0, 0)
	s.AddMark(markStub{origin: [3]float32{1, 2, 3}, size: 8}, -1, 0)
	if s.ActiveCount() != 0 {
		t.Fatalf("ActiveCount = %d, want 0", s.ActiveCount())
	}
}

func TestPrepareDrawsSortsFarToNear(t *testing.T) {
	marks := []MarkEntity{
		markStub{origin: [3]float32{100, 0, 0}, size: 8},
		markStub{origin: [3]float32{1, 0, 0}, size: 8},
		markStub{origin: [3]float32{50, 0, 0}, size: 8},
	}
	draws := PrepareDraws(marks, [3]float32{0, 0, 0})
	if len(draws) != 3 {
		t.Fatalf("len(draws) = %d, want 3", len(draws))
	}
	wantOrder := []float32{100, 50, 1}
	for i, want := range wantOrder {
		if draws[i].Mark.DecalOrigin()[0] != want {
			t.Fatalf("draws[%d].Origin[0] = %v, want %v", i, draws[i].Mark.DecalOrigin()[0], want)
		}
	}
}

func TestPrepareDrawsSkipsZeroSize(t *testing.T) {
	marks := []MarkEntity{
		markStub{origin: [3]float32{100, 0, 0}, size: 0},
		markStub{origin: [3]float32{10, 0, 0}, size: 4},
	}
	draws := PrepareDraws(marks, [3]float32{0, 0, 0})
	if len(draws) != 1 {
		t.Fatalf("len(draws) = %d, want 1", len(draws))
	}
}

func TestBuildBasisFloorIdentity(t *testing.T) {
	tangent, bitangent := BuildBasis([3]float32{0, 0, 1}, 0)
	// For a floor normal, up is replaced with +Y: tangent = Y x Z = X axis,
	// bitangent = Z x X = Y axis with handedness preserved.
	if math.Abs(float64(tangent[0])-1) > 0.001 || math.Abs(float64(tangent[1])) > 0.001 || math.Abs(float64(tangent[2])) > 0.001 {
		t.Fatalf("tangent = %v, want (1,0,0)", tangent)
	}
	if math.Abs(float64(bitangent[0])) > 0.001 || math.Abs(float64(bitangent[1])-1) > 0.001 || math.Abs(float64(bitangent[2])) > 0.001 {
		t.Fatalf("bitangent = %v, want (0,1,0)", bitangent)
	}
}

func TestAtlasDataDeterministic(t *testing.T) {
	a := AtlasData()
	b := AtlasData()
	if len(a) != 256*256*4 {
		t.Fatalf("len(atlas) = %d, want %d", len(a), 256*256*4)
	}
	for i := range a {
		if a[i] != b[i] {
			t.Fatalf("atlas not deterministic at byte %d", i)
		}
	}
	// Out-of-circle region must be fully transparent.
	// Pixel (0,0): px=py=-1, d2=2.
	if a[3] != 0 || a[255*1024+3] != 0 {
		t.Fatalf("expected out-of-circle corner alpha 0, got first=%d last=%d", a[3], a[255*1024+3])
	}
}

func TestSmoothstepClamps(t *testing.T) {
	if got := Smoothstep(0.5, 1.0, 0.0); got != 0 {
		t.Fatalf("Smoothstep(0.5,1,0) = %v, want 0", got)
	}
	if got := Smoothstep(0.5, 1.0, 2.0); got != 1 {
		t.Fatalf("Smoothstep(0.5,1,2) = %v, want 1", got)
	}
	mid := Smoothstep(0.0, 1.0, 0.5)
	if math.Abs(float64(mid-0.5)) > 0.01 {
		t.Fatalf("Smoothstep(0,1,0.5) = %v, want 0.5", mid)
	}
}
