//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"math"
	"testing"

	"github.com/ironwail/ironwail-go/internal/model"
)

func TestSetupAliasFrameInterpolationBlendsMultiPoseFrame(t *testing.T) {
	frames := []model.AliasFrameDesc{{FirstPose: 2, NumPoses: 2, Interval: 0.2}}
	result := setupAliasFrameInterpolation(0, frames, 0.1, true, 0)
	if result.Pose1 != 2 || result.Pose2 != 3 {
		t.Fatalf("poses = (%d,%d), want (2,3)", result.Pose1, result.Pose2)
	}
	if math.Abs(float64(result.Blend-0.5)) > 0.001 {
		t.Fatalf("blend = %f, want 0.5", result.Blend)
	}
}

func TestBuildAliasVerticesInterpolatedAppliesYawRotation(t *testing.T) {
	mdl := &model.Model{AliasHeader: &model.AliasHeader{Scale: [3]float32{1, 1, 1}, ScaleOrigin: [3]float32{0, 0, 0}}}
	alias := &gpuAliasModel{
		refs: []gpuAliasVertexRef{{vertexIndex: 0, texCoord: [2]float32{0.25, 0.75}}},
		poses: [][]model.TriVertX{
			{{V: [3]byte{1, 0, 0}}},
			{{V: [3]byte{1, 0, 0}}},
		},
	}

	vertices := buildAliasVerticesInterpolated(alias, mdl, 0, 1, 0, [3]float32{4, 5, 6}, [3]float32{0, 90, 0}, 1, false)
	if len(vertices) != 1 {
		t.Fatalf("vertex count = %d, want 1", len(vertices))
	}
	got := vertices[0]
	if math.Abs(float64(got.Position[0]-4)) > 0.001 || math.Abs(float64(got.Position[1]-6)) > 0.001 || math.Abs(float64(got.Position[2]-6)) > 0.001 {
		t.Fatalf("rotated position = %v, want [4 6 6]", got.Position)
	}
	if got.TexCoord != [2]float32{0.25, 0.75} {
		t.Fatalf("texcoord = %v, want [0.25 0.75]", got.TexCoord)
	}
}
