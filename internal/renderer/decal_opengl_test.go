//go:build opengl || cgo
// +build opengl cgo

package renderer

import (
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

func TestPrepareDecalDrawsSortsBackToFront(t *testing.T) {
	camera := CameraState{Origin: types.NewVec3(0, 0, 0)}
	marks := []DecalMarkEntity{
		{Origin: [3]float32{10, 0, 0}, Normal: [3]float32{0, 0, 1}, Size: 8, Alpha: 1, Color: [3]float32{1, 1, 1}},
		{Origin: [3]float32{30, 0, 0}, Normal: [3]float32{0, 0, 1}, Size: 8, Alpha: 1, Color: [3]float32{1, 1, 1}},
		{Origin: [3]float32{20, 0, 0}, Normal: [3]float32{0, 0, 1}, Size: 8, Alpha: 1, Color: [3]float32{1, 1, 1}},
	}

	draws := prepareDecalDraws(marks, camera)
	if len(draws) != 3 {
		t.Fatalf("prepareDecalDraws len = %d, want 3", len(draws))
	}
	if draws[0].mark.Origin[0] != 30 || draws[1].mark.Origin[0] != 20 || draws[2].mark.Origin[0] != 10 {
		t.Fatalf("draw order = [%v %v %v], want [30 20 10]", draws[0].mark.Origin[0], draws[1].mark.Origin[0], draws[2].mark.Origin[0])
	}
}

func TestPrepareDecalDrawsFiltersInvalidMarks(t *testing.T) {
	camera := CameraState{Origin: types.NewVec3(0, 0, 0)}
	marks := []DecalMarkEntity{
		{Origin: [3]float32{0, 0, 0}, Normal: [3]float32{0, 0, 1}, Size: 0, Alpha: 1, Color: [3]float32{1, 1, 1}},
		{Origin: [3]float32{0, 0, 0}, Normal: [3]float32{0, 0, 1}, Size: 4, Alpha: -0.5, Color: [3]float32{1, 1, 1}},
		{Origin: [3]float32{0, 0, 0}, Normal: [3]float32{0, 0, 1}, Size: 4, Alpha: 0.5, Color: [3]float32{1, 1, 1}},
	}

	draws := prepareDecalDraws(marks, camera)
	if len(draws) != 1 {
		t.Fatalf("prepareDecalDraws len = %d, want 1", len(draws))
	}
	if draws[0].mark.Alpha != 0.5 {
		t.Fatalf("alpha = %v, want 0.5", draws[0].mark.Alpha)
	}
}

func TestBuildDecalQuadOnFloor(t *testing.T) {
	mark := DecalMarkEntity{
		Origin: [3]float32{100, 200, 300},
		Normal: [3]float32{0, 0, 1},
		Size:   16,
		Alpha:  1,
		Color:  [3]float32{1, 1, 1},
	}

	quad, ok := buildDecalQuad(mark)
	if !ok {
		t.Fatalf("buildDecalQuad returned !ok")
	}
	for i := 0; i < 4; i++ {
		if math.Abs(float64(quad[i][2]-(mark.Origin[2]+0.05))) > 1e-5 {
			t.Fatalf("quad[%d].z = %v, want %v", i, quad[i][2], mark.Origin[2]+0.05)
		}
	}
}

func TestBuildDecalTriangleVerticesCount(t *testing.T) {
	mark := DecalMarkEntity{
		Origin: [3]float32{0, 0, 0},
		Normal: [3]float32{0, 0, 1},
		Size:   8,
		Alpha:  0.8,
		Color:  [3]float32{0.1, 0.2, 0.3},
	}

	verts := buildDecalTriangleVertices(mark)
	if len(verts) != 6*decalFloatsPerVertex {
		t.Fatalf("vertex float count = %d, want %d", len(verts), 6*decalFloatsPerVertex)
	}

	for i := 8; i < len(verts); i += decalFloatsPerVertex {
		if math.Abs(float64(verts[i]-0.8)) > 1e-6 {
			t.Fatalf("vertex alpha = %v, want 0.8", verts[i])
		}
	}
}

func TestBuildDecalTriangleVerticesIncludesVariant(t *testing.T) {
	mark := DecalMarkEntity{
		Origin:  [3]float32{0, 0, 0},
		Normal:  [3]float32{0, 0, 1},
		Size:    8,
		Alpha:   1,
		Color:   [3]float32{1, 1, 1},
		Variant: DecalVariantScorch,
	}

	verts := buildDecalTriangleVertices(mark)
	if len(verts) != 6*decalFloatsPerVertex {
		t.Fatalf("vertex float count = %d, want %d", len(verts), 6*decalFloatsPerVertex)
	}
	for i := 9; i < len(verts); i += decalFloatsPerVertex {
		if verts[i] != float32(DecalVariantScorch) {
			t.Fatalf("vertex variant = %v, want %v", verts[i], float32(DecalVariantScorch))
		}
	}
}

func TestDecalTriangleVertexCountMatchesDrawStride(t *testing.T) {
	mark := DecalMarkEntity{
		Origin: [3]float32{0, 0, 0},
		Normal: [3]float32{0, 0, 1},
		Size:   8,
		Alpha:  1,
		Color:  [3]float32{1, 1, 1},
	}

	verts := buildDecalTriangleVertices(mark)
	if got := len(verts) / decalFloatsPerVertex; got != 6 {
		t.Fatalf("draw vertex count = %d, want 6", got)
	}
	if len(verts)%decalFloatsPerVertex != 0 {
		t.Fatalf("vertex float count = %d not divisible by stride %d", len(verts), decalFloatsPerVertex)
	}
}

func TestBuildDecalBasisOrthogonal(t *testing.T) {
	normal := [3]float32{0, 0, 1}
	tangent, bitangent := buildDecalBasis(normal, 0)

	dotNT := normal[0]*tangent[0] + normal[1]*tangent[1] + normal[2]*tangent[2]
	dotNB := normal[0]*bitangent[0] + normal[1]*bitangent[1] + normal[2]*bitangent[2]
	dotTB := tangent[0]*bitangent[0] + tangent[1]*bitangent[1] + tangent[2]*bitangent[2]

	if math.Abs(float64(dotNT)) > 1e-5 {
		t.Fatalf("dot(normal,tangent) = %v, want ~0", dotNT)
	}
	if math.Abs(float64(dotNB)) > 1e-5 {
		t.Fatalf("dot(normal,bitangent) = %v, want ~0", dotNB)
	}
	if math.Abs(float64(dotTB)) > 1e-5 {
		t.Fatalf("dot(tangent,bitangent) = %v, want ~0", dotTB)
	}
}

func TestNormalizeDecalVariant(t *testing.T) {
	if got := normalizeDecalVariant(DecalVariantMagic); got != DecalVariantMagic {
		t.Fatalf("normalizeDecalVariant(magic) = %v, want %v", got, DecalVariantMagic)
	}
	if got := normalizeDecalVariant(DecalVariant(-123)); got != DecalVariantBullet {
		t.Fatalf("normalizeDecalVariant(invalid) = %v, want %v", got, DecalVariantBullet)
	}
}

func TestGenerateDecalAtlasData(t *testing.T) {
	data := generateDecalAtlasData()

	// Check size
	expectedSize := 256 * 256 * 4 // RGBA
	if len(data) != expectedSize {
		t.Fatalf("atlas data size = %d, want %d", len(data), expectedSize)
	}

	// Check that all regions have some non-zero data
	regions := []struct {
		name   string
		x, y   int
		width  int
		height int
	}{
		{"bullet", 0, 0, 128, 128},
		{"chip", 128, 0, 128, 128},
		{"scorch", 0, 128, 128, 128},
		{"magic", 128, 128, 128, 128},
	}

	for _, region := range regions {
		hasData := false
		for y := region.y; y < region.y+region.height && !hasData; y++ {
			for x := region.x; x < region.x+region.width && !hasData; x++ {
				idx := (y*256 + x) * 4
				if data[idx+3] > 0 { // Check alpha channel
					hasData = true
				}
			}
		}
		if !hasData {
			t.Errorf("region %s has no visible data", region.name)
		}
	}
}

func TestSmoothstepf(t *testing.T) {
	tests := []struct {
		edge0, edge1, x float32
		want            float32
	}{
		{0, 1, 0, 0},
		{0, 1, 1, 1},
		{0, 1, 0.5, 0.5},
		{0, 1, -0.5, 0},
		{0, 1, 1.5, 1},
	}

	for _, tt := range tests {
		got := smoothstepf(tt.edge0, tt.edge1, tt.x)
		if tt.x <= tt.edge0 {
			if got != 0 {
				t.Errorf("smoothstepf(%v, %v, %v) = %v, want 0", tt.edge0, tt.edge1, tt.x, got)
			}
		} else if tt.x >= tt.edge1 {
			if got != 1 {
				t.Errorf("smoothstepf(%v, %v, %v) = %v, want 1", tt.edge0, tt.edge1, tt.x, got)
			}
		}
	}
}
