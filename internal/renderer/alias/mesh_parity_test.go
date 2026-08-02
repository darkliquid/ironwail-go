package alias

import (
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/model"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

// TestModelSpaceVersusCPUTransformParity verifies that the GPU path
// (BuildVerticesModelSpaceInto + AliasEntityModelMatrix) produces the
// same world-space vertex positions as the CPU path
// (BuildVerticesInterpolatedInto). Any mismatch here causes models to
// appear at the wrong position or orientation, which manifests as
// flickering or jitter when the entity animates between frames.
func TestModelSpaceVersusCPUTransformParity(t *testing.T) {
	mesh := MeshFromRefs(
		[][]model.TriVertX{
			{
				{V: [3]byte{10, 20, 30}, LightNormalIndex: 0},
				{V: [3]byte{40, 50, 60}, LightNormalIndex: 0},
			},
			{
				{V: [3]byte{20, 30, 40}, LightNormalIndex: 0},
				{V: [3]byte{50, 60, 70}, LightNormalIndex: 0},
			},
		},
		[]MeshRef{
			{VertexIndex: 0, TexCoord: [2]float32{0.25, 0.75}},
			{VertexIndex: 1, TexCoord: [2]float32{0.5, 0.25}},
		},
	)
	hdr := &model.AliasHeader{
		Scale:       [3]float32{1, 1, 1},
		ScaleOrigin: [3]float32{},
	}

	cases := []struct {
		name        string
		origin      [3]float32
		angles      [3]float32
		entityScale float32
		fullAngles  bool
		blend       float32
	}{
		{"yaw_only", [3]float32{10, 20, 30}, [3]float32{0, 45, 0}, 1.0, false, 0.5},
		{"yaw_scale", [3]float32{100, 200, 300}, [3]float32{0, 90, 0}, 2.0, false, 0.5},
		{"full_angles", [3]float32{10, 20, 30}, [3]float32{15, 45, 30}, 1.0, true, 0.5},
		{"full_angles_scale", [3]float32{100, 200, 300}, [3]float32{30, 90, 0}, 2.0, true, 0.3},
		{"zero_yaw_origin", [3]float32{0, 0, 0}, [3]float32{0, 0, 0}, 1.0, false, 1.0},
		{"negative_yaw", [3]float32{50, 50, 50}, [3]float32{0, -90, 0}, 1.5, false, 0.7},
	}

	const eps = 0.01

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// CPU path: fully transformed vertices
			cpuVerts := BuildVerticesInterpolatedInto(nil, mesh, hdr, 0, 1, tc.blend,
				tc.origin, tc.angles, tc.entityScale, tc.fullAngles)

			// GPU path: model-space vertices + model matrix
			gpuVerts := BuildVerticesModelSpaceInto(nil, mesh, hdr, 0, 1, tc.blend)
			modelMat := AliasEntityModelMatrix(tc.origin, tc.angles, tc.entityScale, tc.fullAngles)

			if len(cpuVerts) != len(gpuVerts) {
				t.Fatalf("vertex count mismatch: cpu=%d gpu=%d", len(cpuVerts), len(gpuVerts))
			}

			for i := range cpuVerts {
				// Apply model matrix to GPU vertex position
				v := types.Vec4{X: gpuVerts[i].Position[0], Y: gpuVerts[i].Position[1], Z: gpuVerts[i].Position[2], W: 1.0}
				transformed := types.Mat4MulVec4(modelMat, v)

				for j := 0; j < 3; j++ {
					cpuPos := cpuVerts[i].Position[j]
					gpuPos := [3]float32{transformed.X, transformed.Y, transformed.Z}[j]
					diff := math.Abs(float64(cpuPos - gpuPos))
					if diff > eps {
						t.Errorf("vertex %d component %d: cpu=%f gpu=%f diff=%f (eps=%f)",
							i, j, cpuPos, gpuPos, diff, eps)
					}
				}
			}
		})
	}
}
