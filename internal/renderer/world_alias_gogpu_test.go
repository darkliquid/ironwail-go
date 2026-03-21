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

func TestResolveAliasSkinSlotUsesGroupedSkinTimingGoGPU(t *testing.T) {
	hdr := &model.AliasHeader{
		Skins: make([][]byte, 4),
		SkinDescs: []model.AliasSkinDesc{
			{FirstFrame: 0, NumFrames: 1},
			{FirstFrame: 1, NumFrames: 3, Intervals: []float32{0.1, 0.2, 0.3}},
		},
	}

	if got := resolveAliasSkinSlot(hdr, 1, 0.05, 4); got != 1 {
		t.Fatalf("slot at t=0.05 = %d, want 1", got)
	}
	if got := resolveAliasSkinSlot(hdr, 1, 0.15, 4); got != 2 {
		t.Fatalf("slot at t=0.15 = %d, want 2", got)
	}
	if got := resolveAliasSkinSlot(hdr, 1, 0.25, 4); got != 3 {
		t.Fatalf("slot at t=0.25 = %d, want 3", got)
	}
	if got := resolveAliasSkinSlot(hdr, 1, 0.35, 4); got != 1 {
		t.Fatalf("slot at t=0.35 = %d, want 1", got)
	}
}
