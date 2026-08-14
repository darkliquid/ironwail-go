package alias

import (
	"math"
	"testing"

	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
	"github.com/darkliquid/ironwail-go/pkg/types"
)

type convertibleBackendRef struct {
	index int
	uv    [2]float32
}

func (ref convertibleBackendRef) AliasMeshRef() MeshRef {
	return MeshRef{VertexIndex: ref.index, TexCoord: ref.uv}
}

func TestInterpolateVertexPosition(t *testing.T) {
	scale := types.Vec3{X: 1, Y: 1, Z: 1}
	origin := types.Vec3{}
	vert1 := model.TriVertX{V: [3]byte{10, 20, 30}}
	vert2 := model.TriVertX{V: [3]byte{20, 40, 60}}

	tests := []struct {
		name   string
		factor float32
		want   types.Vec3
	}{
		{name: "pose1", factor: 0, want: types.Vec3{X: 10, Y: 20, Z: 30}},
		{name: "halfway", factor: 0.5, want: types.Vec3{X: 15, Y: 30, Z: 45}},
		{name: "pose2", factor: 1, want: types.Vec3{X: 20, Y: 40, Z: 60}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InterpolateVertexPosition(vert1, vert2, scale, origin, tt.factor)
			if math.Abs(float64(got.X-tt.want.X)) > 0.01 {
				t.Fatalf("axis x = %f, want %f", got.X, tt.want.X)
			}
			if math.Abs(float64(got.Y-tt.want.Y)) > 0.01 {
				t.Fatalf("axis y = %f, want %f", got.Y, tt.want.Y)
			}
			if math.Abs(float64(got.Z-tt.want.Z)) > 0.01 {
				t.Fatalf("axis z = %f, want %f", got.Z, tt.want.Z)
			}

		})
	}
}

func TestBuildVerticesInterpolated(t *testing.T) {
	mesh := MeshFromRefs(
		[][]model.TriVertX{
			{{V: [3]byte{1, 0, 0}, LightNormalIndex: 0}},
			{{V: [3]byte{3, 0, 0}, LightNormalIndex: 0}},
		},
		[]MeshRef{{VertexIndex: 0, TexCoord: [2]float32{0.25, 0.75}}},
	)
	hdr := &model.AliasHeader{
		Scale:       types.Vec3{X: 1, Y: 1, Z: 1},
		ScaleOrigin: types.Vec3{},
	}

	got := BuildVerticesInterpolated(mesh, hdr, 0, 1, 0.5, types.Vec3{X: 10, Y: 20, Z: 30}, types.Vec3{X: 0, Y: 90, Z: 0}, 2, false)
	if len(got) != 1 {
		t.Fatalf("len(vertices) = %d, want 1", len(got))
	}

	wantPosition := types.Vec3{X: 10, Y: 24, Z: 30}
	diffX := math.Abs(float64(got[0].Position.X - wantPosition.X))
	diffY := math.Abs(float64(got[0].Position.Y - wantPosition.Y))
	diffZ := math.Abs(float64(got[0].Position.Z - wantPosition.Z))
	if diffX > 0.01 || diffY > 0.01 || diffZ > 0.01 {
		t.Fatalf("position = %v, want %v", got[0].Position, wantPosition)
	}
	if got[0].TexCoord != ([2]float32{0.25, 0.75}) {
		t.Fatalf("texcoord = %v", got[0].TexCoord)
	}
}

func TestBuildVerticesInterpolatedInto(t *testing.T) {
	mesh := MeshFromRefs(
		[][]model.TriVertX{
			{{V: [3]byte{1, 0, 0}, LightNormalIndex: 0}},
			{{V: [3]byte{3, 0, 0}, LightNormalIndex: 0}},
		},
		[]MeshRef{{VertexIndex: 0, TexCoord: [2]float32{0.25, 0.75}}},
	)
	hdr := &model.AliasHeader{
		Scale:       types.Vec3{X: 1, Y: 1, Z: 1},
		ScaleOrigin: types.Vec3{},
	}
	input := make([]worldimpl.WorldVertex, 0, 4)
	input = append(input, worldimpl.WorldVertex{})
	input = input[:0]
	beforePtr := &input[:cap(input)][0]

	got := BuildVerticesInterpolatedInto(input, mesh, hdr, 0, 1, 0.5, types.Vec3{X: 10, Y: 20, Z: 30}, types.Vec3{X: 0, Y: 90, Z: 0}, 2, false)
	if len(got) != 1 {
		t.Fatalf("len(vertices) = %d, want 1", len(got))
	}
	afterPtr := &got[:cap(got)][0]
	if beforePtr != afterPtr {
		t.Fatalf("expected BuildVerticesInterpolatedInto to reuse caller buffer")
	}

	wantPosition := types.Vec3{X: 10, Y: 24, Z: 30}
	diffX := math.Abs(float64(got[0].Position.X - wantPosition.X))
	diffY := math.Abs(float64(got[0].Position.Y - wantPosition.Y))
	diffZ := math.Abs(float64(got[0].Position.Z - wantPosition.Z))
	if diffX > 0.01 || diffY > 0.01 || diffZ > 0.01 {
		t.Fatalf("position = %v, want %v", got[0].Position, wantPosition)
	}

	allocs := testing.AllocsPerRun(100, func() {
		_ = BuildVerticesInterpolatedInto(input, mesh, hdr, 0, 1, 0.5, types.Vec3{X: 10, Y: 20, Z: 30}, types.Vec3{X: 0, Y: 90, Z: 0}, 2, false)
	})
	if allocs != 0 {
		t.Fatalf("BuildVerticesInterpolatedInto allocated %.2f times per run, want 0", allocs)
	}
}

func TestBuildVerticesUsesSinglePose(t *testing.T) {
	mesh := MeshFromRefs(
		[][]model.TriVertX{
			{{V: [3]byte{2, 4, 6}, LightNormalIndex: 0}},
		},
		[]MeshRef{{VertexIndex: 0, TexCoord: [2]float32{1, 0}}},
	)
	hdr := &model.AliasHeader{
		Scale:       types.Vec3{X: 1, Y: 1, Z: 1},
		ScaleOrigin: types.Vec3{},
	}

	got := BuildVertices(mesh, hdr, 0, types.Vec3{X: 1, Y: 2, Z: 3}, types.Vec3{}, true)
	if len(got) != 1 {
		t.Fatalf("len(vertices) = %d, want 1", len(got))
	}
	want := types.Vec3{X: 3, Y: 6, Z: 9}
	diffX := math.Abs(float64(got[0].Position.X - want.X))
	diffY := math.Abs(float64(got[0].Position.Y - want.Y))
	diffZ := math.Abs(float64(got[0].Position.Z - want.Z))
	if diffX > 0.01 || diffY > 0.01 || diffZ > 0.01 {
		t.Fatalf("position = %v, want %v", got[0].Position, want)
	}
}

func BenchmarkBuildVerticesInterpolatedInto(b *testing.B) {
	poses := make([][]model.TriVertX, 2)
	poses[0] = make([]model.TriVertX, 500)
	poses[1] = make([]model.TriVertX, 500)
	refs := make([]MeshRef, 1500)
	for i := 0; i < 500; i++ {
		poses[0][i] = model.TriVertX{V: [3]byte{byte(i % 250), byte((i * 2) % 250), byte((i * 3) % 250)}, LightNormalIndex: byte(i % 162)}
		poses[1][i] = model.TriVertX{V: [3]byte{byte((i + 10) % 250), byte((i*2 + 10) % 250), byte((i*3 + 10) % 250)}, LightNormalIndex: byte(i % 162)}
	}
	for i := 0; i < 1500; i++ {
		refs[i] = MeshRef{VertexIndex: i % 500, TexCoord: [2]float32{float32(i) / 1500.0, float32(i) / 1500.0}}
	}
	mesh := MeshFromRefs(poses, refs)
	hdr := &model.AliasHeader{
		Scale:       types.Vec3{X: 1, Y: 1, Z: 1},
		ScaleOrigin: types.Vec3{X: 0, Y: 0, Z: 0},
	}
	dst := make([]worldimpl.WorldVertex, 0, 1500)
	angles := types.Vec3{X: 15, Y: 45, Z: 10}
	origin := types.Vec3{X: 100, Y: 200, Z: 300}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		dst = BuildVerticesInterpolatedInto(dst, mesh, hdr, 0, 1, 0.5, origin, angles, 1.0, true)
	}
}

func TestMeshFromAccessor(t *testing.T) {
	type backendRef struct {
		index int
		uv    [2]float32
	}

	mesh := MeshFromAccessor(
		[][]model.TriVertX{{}},
		[]backendRef{{index: 3, uv: [2]float32{0.5, 0.25}}},
		func(ref backendRef) MeshRef {
			return MeshRef{VertexIndex: ref.index, TexCoord: ref.uv}
		},
	)

	if mesh.RefCount != 1 {
		t.Fatalf("RefCount = %d, want 1", mesh.RefCount)
	}
	got := mesh.RefAt(0)
	if got.VertexIndex != 3 || got.TexCoord != ([2]float32{0.5, 0.25}) {
		t.Fatalf("RefAt(0) = %#v", got)
	}
}

func TestMeshFromConvertibleRefs(t *testing.T) {
	mesh := MeshFromConvertibleRefs(
		[][]model.TriVertX{{}},
		[]convertibleBackendRef{{index: 7, uv: [2]float32{0.75, 0.5}}},
	)

	if mesh.RefCount != 1 {
		t.Fatalf("RefCount = %d, want 1", mesh.RefCount)
	}
	got := mesh.RefAt(0)
	if got.VertexIndex != 7 || got.TexCoord != ([2]float32{0.75, 0.5}) {
		t.Fatalf("RefAt(0) = %#v", got)
	}
}

func TestSetupFrameInterpolation(t *testing.T) {
	frames := []FrameDesc{
		{FirstPose: 0, NumPoses: 1, Interval: 0.1},
		{FirstPose: 1, NumPoses: 3, Interval: 0.05},
		{FirstPose: 4, NumPoses: 1, Interval: 0.1},
	}

	tests := []struct {
		name          string
		frameIndex    int
		timeSeconds   float64
		lerpModels    bool
		flags         int
		expectedPose1 int
		expectedPose2 int
		expectedBlend float32
		blendRange    float32
	}{
		{
			name:          "single-pose frame has no blend",
			frameIndex:    0,
			timeSeconds:   0.0,
			lerpModels:    true,
			flags:         0,
			expectedPose1: 0,
			expectedPose2: 0,
			expectedBlend: 0.0,
		},
		{
			name:          "multi-pose frame at t=0 starts at first pose",
			frameIndex:    1,
			timeSeconds:   0.0,
			lerpModels:    true,
			flags:         0,
			expectedPose1: 1,
			expectedPose2: 2,
			expectedBlend: 0.0,
			blendRange:    0.1,
		},
		{
			name:          "multi-pose frame halfway through interval blends",
			frameIndex:    1,
			timeSeconds:   0.025,
			lerpModels:    true,
			flags:         0,
			expectedPose1: 1,
			expectedPose2: 2,
			expectedBlend: 0.5,
			blendRange:    0.1,
		},
		{
			name:          "ModNoLerp disables blend",
			frameIndex:    1,
			timeSeconds:   0.025,
			lerpModels:    true,
			flags:         ModNoLerp,
			expectedPose1: 1,
			expectedPose2: 2,
			expectedBlend: 0.0,
			blendRange:    0.01,
		},
		{
			name:          "invalid frame defaults to zero",
			frameIndex:    99,
			timeSeconds:   0.0,
			lerpModels:    true,
			flags:         0,
			expectedPose1: 0,
			expectedPose2: 0,
			expectedBlend: 0.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SetupFrameInterpolation(tt.frameIndex, frames, tt.timeSeconds, tt.lerpModels, tt.flags)
			if got.Pose1 != tt.expectedPose1 {
				t.Fatalf("Pose1 = %d, want %d", got.Pose1, tt.expectedPose1)
			}
			if got.Pose2 != tt.expectedPose2 {
				t.Fatalf("Pose2 = %d, want %d", got.Pose2, tt.expectedPose2)
			}
			if tt.blendRange == 0 {
				tt.blendRange = 0.01
			}
			if math.Abs(float64(got.Blend-tt.expectedBlend)) > float64(tt.blendRange) {
				t.Fatalf("Blend = %f, want %f (±%f)", got.Blend, tt.expectedBlend, tt.blendRange)
			}
		})
	}
}

// rEntityMatrixReference computes the column-major model matrix produced by
// C Ironwail's R_EntityMatrix (gl_rmain.c:442) for the alias path (no pitch
// pre-negation), with scale=1. Only the 3×3 rotation block is returned since
// the viewmodel test below only checks rotation of a point at origin offset.
func rEntityMatrixReference(angles types.Vec3) [9]float32 {
	pitch := float64(angles.X) * math.Pi / 180
	yaw := float64(angles.Y) * math.Pi / 180
	roll := float64(angles.Z) * math.Pi / 180
	sy := math.Sin(yaw)
	sp := math.Sin(pitch)
	sr := math.Sin(roll)
	cy := math.Cos(yaw)
	cp := math.Cos(pitch)
	cr := math.Cos(roll)
	return [9]float32{
		// col 0 (local X -> world)
		float32(cy * cp),
		float32(sy * cp),
		float32(sp),
		// col 1
		float32(-cy*sp*sr - cr*sy),
		float32(cr*cy - sy*sp*sr),
		float32(cp * sr),
		// col 2
		float32(sy*sr - cr*cy*sp),
		float32(-cy*sr - cr*sy*sp),
		float32(cr * cp),
	}
}

func applyColMajor3x3(m [9]float32, v types.Vec3) types.Vec3 {
	return types.Vec3{
		X: m[0]*v.X + m[3]*v.Y + m[6]*v.Z,
		Y: m[1]*v.X + m[4]*v.Y + m[7]*v.Z,
		Z: m[2]*v.X + m[5]*v.Y + m[8]*v.Z,
	}
}

// TestRotateAnglesMatchesREntityMatrix verifies that the Go helper produces
// bit-close parity with C R_EntityMatrix (the alias-path variant that does
// not pre-negate pitch). Regression test for the viewmodel positioning bug
// where extreme pitch with a nonzero yaw composed the rotations in the wrong
// order.
func TestRotateAnglesMatchesREntityMatrix(t *testing.T) {
	cases := []struct {
		name   string
		angles types.Vec3
	}{
		{"identity", types.Vec3{}},
		{"yaw only", types.Vec3{Y: 45}},
		{"pitch only", types.Vec3{X: 30}},
		{"roll only", types.Vec3{Z: 15}},
		{"yaw+pitch looking up while facing east", types.Vec3{X: 60, Y: 90}},
		{"yaw+pitch looking down turned 180", types.Vec3{X: -60, Y: 180}},
		{"full tilt", types.Vec3{X: 35, Y: 110, Z: 20}},
	}
	probes := []types.Vec3{
		{X: 1, Y: 0, Z: 0}, {X: 0, Y: 1, Z: 0}, {X: 0, Y: 0, Z: 1}, {X: 0.5, Y: -0.5, Z: 0.25},
	}
	const eps = 1e-4
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref := rEntityMatrixReference(tc.angles)
			for _, p := range probes {
				got := RotateAngles(p, tc.angles)
				want := applyColMajor3x3(ref, p)
				diffX := float64(got.X - want.X)
				diffY := float64(got.Y - want.Y)
				diffZ := float64(got.Z - want.Z)
				if math.Abs(diffX) > eps || math.Abs(diffY) > eps || math.Abs(diffZ) > eps {
					t.Fatalf("angles=%v probe=%v: got %v want %v", tc.angles, p, got, want)
				}
			}
		})
	}
}
