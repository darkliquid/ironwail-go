package decal

import (
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

// markStub is a minimal MarkEntity for tests.
type markStub struct {
	origin   types.Vec3
	normal   types.Vec3
	size     float32
	rotation float32
	alpha    float32
	variant  int
}

func (m markStub) DecalOrigin() types.Vec3 { return m.origin }
func (m markStub) DecalNormal() types.Vec3 { return m.normal }
func (m markStub) DecalSize() float32      { return m.size }
func (m markStub) DecalRotation() float32  { return m.rotation }
func (m markStub) DecalAlpha() float32     { return m.alpha }
func (m markStub) DecalVariant() int       { return m.variant }

func TestBuildQuadFloorFacingUp(t *testing.T) {
	got, ok := BuildQuad(markStub{
		origin:   types.Vec3{X: 100, Y: 200, Z: 300},
		normal:   types.Vec3{X: 0, Y: 0, Z: 1},
		size:     16,
		rotation: 0,
	})
	if !ok {
		t.Fatal("BuildQuad = !ok, want ok")
	}
	// Quad centered at origin + normal*0.05, half-size 8.
	center := types.Vec3{X: 100, Y: 200, Z: 300.05}
	for _, c := range got {
		if math.Abs(float64(c.X-center.X)) > 8.001 || math.Abs(float64(c.Y-center.Y)) > 8.001 {
			t.Fatalf("corner %v outside expected square around %v (size 16)", c, center)
		}
		if math.Abs(float64(c.Z-center.Z)) > 0.001 {
			t.Fatalf("corner %v Z = %v, want %.3f (flat on plane)", c, c.Z, center.Z)
		}
	}
}

func TestBuildQuadDefaultNormalUp(t *testing.T) {
	// Direct zero normal is rejected by Normalize3 inside BuildQuad; callers
	// (PrepareDraws' legacy root adapter) default normals before building.
	if _, ok := BuildQuad(markStub{
		origin:   types.Vec3{X: 10, Y: 20, Z: 30},
		size:     8,
		rotation: 0,
	}); ok {
		t.Fatal("BuildQuad with zero normal = ok, want !ok")
	}
}

func TestSystemLifetime(t *testing.T) {
	s := NewSystem()
	mark := markStub{origin: types.Vec3{X: 1, Y: 2, Z: 3}, size: 8, alpha: 1}
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
	s.AddMark(markStub{origin: types.Vec3{X: 1, Y: 2, Z: 3}, size: 0}, 1, 0)
	s.AddMark(markStub{origin: types.Vec3{X: 1, Y: 2, Z: 3}, size: 8}, 0, 0)
	s.AddMark(markStub{origin: types.Vec3{X: 1, Y: 2, Z: 3}, size: 8}, -1, 0)
	if s.ActiveCount() != 0 {
		t.Fatalf("ActiveCount = %d, want 0", s.ActiveCount())
	}
}

func TestPrepareDrawsSortsFarToNear(t *testing.T) {
	marks := []MarkEntity{
		markStub{origin: types.Vec3{X: 100, Y: 0, Z: 0}, size: 8, alpha: 1},
		markStub{origin: types.Vec3{X: 1, Y: 0, Z: 0}, size: 8, alpha: 1},
		markStub{origin: types.Vec3{X: 50, Y: 0, Z: 0}, size: 8, alpha: 1},
	}
	draws := PrepareDraws(marks, types.Vec3{X: 0, Y: 0, Z: 0})
	if len(draws) != 3 {
		t.Fatalf("len(draws) = %d, want 3", len(draws))
	}
	wantOrder := []float32{100, 50, 1}
	for i, want := range wantOrder {
		if draws[i].Mark.DecalOrigin().X != want {
			t.Fatalf("draws[%d].Origin.X = %v, want %v", i, draws[i].Mark.DecalOrigin().X, want)
		}
	}
}

func TestPrepareDrawsSkipsZeroSize(t *testing.T) {
	marks := []MarkEntity{
		markStub{origin: types.Vec3{X: 100, Y: 0, Z: 0}, size: 0, alpha: 1},
		markStub{origin: types.Vec3{X: 10, Y: 0, Z: 0}, size: 4, alpha: 1},
	}
	draws := PrepareDraws(marks, types.Vec3{X: 0, Y: 0, Z: 0})
	if len(draws) != 1 {
		t.Fatalf("len(draws) = %d, want 1", len(draws))
	}
}

func TestBuildBasisFloorIdentity(t *testing.T) {
	tangent, bitangent := BuildBasis(types.Vec3{X: 0, Y: 0, Z: 1}, 0)
	// For a floor normal, up is replaced with +Y: tangent = Y x Z = X axis,
	// bitangent = Z x X = Y axis with handedness preserved.
	if math.Abs(float64(tangent.X-1)) > 0.001 || math.Abs(float64(tangent.Y)) > 0.001 || math.Abs(float64(tangent.Z)) > 0.001 {
		t.Fatalf("tangent = %v, want (1,0,0)", tangent)
	}
	if math.Abs(float64(bitangent.X)) > 0.001 || math.Abs(float64(bitangent.Y-1)) > 0.001 || math.Abs(float64(bitangent.Z)) > 0.001 {
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

// TestPrepareDrawsClampsAlpha pins the alpha clamp + alpha<=0 skip that was
// dropped in the decal extraction and restored in 6b6f7a8: a mark with alpha
// > 1 must be clamped and still drawn; alpha == 0 must be dropped.
func TestPrepareDrawsClampsAlpha(t *testing.T) {
	over := markStub{origin: types.Vec3{X: 10, Y: 0, Z: 0}, size: 8, alpha: 1.5, variant: 2}
	zero := markStub{origin: types.Vec3{X: 20, Y: 0, Z: 0}, size: 8, alpha: 0, variant: 2}
	draws := PrepareDraws([]MarkEntity{over, zero}, types.Vec3{})
	if len(draws) != 1 {
		t.Fatalf("len(draws) = %d, want 1 (alpha=0 mark dropped, alpha=1.5 kept)", len(draws))
	}
	// The returned mark carries the clamped alpha via the normalized wrapper;
	// BuildQuad must still produce a valid quad with the clamped value.
	if _, ok := BuildQuad(draws[0].Mark); !ok {
		t.Fatal("BuildQuad on clamped mark = !ok")
	}
}

// TestPrepareDrawsDefaultsZeroNormal pins the +Z normal defaulting restored
// in 6b6f7a8: a mark with a zero normal must still produce an up-facing quad.
func TestPrepareDrawsDefaultsZeroNormal(t *testing.T) {
	m := markStub{origin: types.Vec3{X: 10, Y: 20, Z: 30}, size: 8, alpha: 1}
	draws := PrepareDraws([]MarkEntity{m}, types.Vec3{})
	if len(draws) != 1 {
		t.Fatalf("len(draws) = %d, want 1", len(draws))
	}
	corners, ok := BuildQuad(draws[0].Mark)
	if !ok {
		t.Fatal("BuildQuad = !ok, want ok (zero normal defaulted to +Z)")
	}
	// Floor quad: all corners lie on z = origin.z + 0.05.
	for _, c := range corners {
		if math.Abs(float64(c.Z-30.05)) > 0.001 {
			t.Fatalf("corner %v z = %v, want 30.05 (flat +Z quad)", c, c.Z)
		}
	}
}

// TestNormalizeVariantPinsInvalidDefault pins that invalid variants map to
// the bullet (0) atlas region.
func TestNormalizeVariantPinsInvalidDefault(t *testing.T) {
	if got := NormalizeVariant(0); got != 0 {
		t.Fatalf("NormalizeVariant(0) = %d, want 0", got)
	}
	if got := NormalizeVariant(3); got != 3 {
		t.Fatalf("NormalizeVariant(3) = %d, want 3", got)
	}
	if got := NormalizeVariant(99); got != 0 {
		t.Fatalf("NormalizeVariant(99) = %d, want 0 (invalid -> bullet)", got)
	}
	if got := NormalizeVariant(-1); got != 0 {
		t.Fatalf("NormalizeVariant(-1) = %d, want 0 (invalid -> bullet)", got)
	}
}
