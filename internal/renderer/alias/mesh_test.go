package alias

import (
	"math"
	"testing"

	"github.com/ironwail/ironwail-go/internal/model"
)

func TestInterpolateVertexPosition(t *testing.T) {
	scale := [3]float32{1, 1, 1}
	origin := [3]float32{}
	vert1 := model.TriVertX{V: [3]byte{10, 20, 30}}
	vert2 := model.TriVertX{V: [3]byte{20, 40, 60}}

	tests := []struct {
		name   string
		factor float32
		want   [3]float32
	}{
		{name: "pose1", factor: 0, want: [3]float32{10, 20, 30}},
		{name: "halfway", factor: 0.5, want: [3]float32{15, 30, 45}},
		{name: "pose2", factor: 1, want: [3]float32{20, 40, 60}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InterpolateVertexPosition(vert1, vert2, scale, origin, tt.factor)
			for i := range got {
				if math.Abs(float64(got[i]-tt.want[i])) > 0.01 {
					t.Fatalf("axis %d = %f, want %f", i, got[i], tt.want[i])
				}
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
		Scale:       [3]float32{1, 1, 1},
		ScaleOrigin: [3]float32{},
	}

	got := BuildVerticesInterpolated(mesh, hdr, 0, 1, 0.5, [3]float32{10, 20, 30}, [3]float32{0, 90, 0}, 2, false)
	if len(got) != 1 {
		t.Fatalf("len(vertices) = %d, want 1", len(got))
	}

	wantPosition := [3]float32{10, 24, 30}
	for i := range wantPosition {
		if math.Abs(float64(got[0].Position[i]-wantPosition[i])) > 0.01 {
			t.Fatalf("position[%d] = %f, want %f", i, got[0].Position[i], wantPosition[i])
		}
	}
	if got[0].TexCoord != ([2]float32{0.25, 0.75}) {
		t.Fatalf("texcoord = %v", got[0].TexCoord)
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
		Scale:       [3]float32{1, 1, 1},
		ScaleOrigin: [3]float32{},
	}

	got := BuildVertices(mesh, hdr, 0, [3]float32{1, 2, 3}, [3]float32{}, true)
	if len(got) != 1 {
		t.Fatalf("len(vertices) = %d, want 1", len(got))
	}
	want := [3]float32{3, 6, 9}
	for i := range want {
		if math.Abs(float64(got[0].Position[i]-want[i])) > 0.01 {
			t.Fatalf("position[%d] = %f, want %f", i, got[0].Position[i], want[i])
		}
	}
}
