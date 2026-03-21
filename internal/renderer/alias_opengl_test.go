//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"math"
	"testing"

	"github.com/ironwail/ironwail-go/internal/model"
)

// TestInterpolateVertexPosition tests vertex position interpolation between two poses.
func TestInterpolateVertexPosition(t *testing.T) {
	scale := [3]float32{1.0, 1.0, 1.0}
	origin := [3]float32{0.0, 0.0, 0.0}

	tests := []struct {
		name      string
		vert1     model.TriVertX
		vert2     model.TriVertX
		factor    float32
		expectedX float32
		expectedY float32
		expectedZ float32
	}{
		{
			name:      "factor 0 should return position from pose1",
			vert1:     model.TriVertX{V: [3]byte{10, 20, 30}, LightNormalIndex: 0},
			vert2:     model.TriVertX{V: [3]byte{20, 40, 60}, LightNormalIndex: 0},
			factor:    0.0,
			expectedX: 10.0,
			expectedY: 20.0,
			expectedZ: 30.0,
		},
		{
			name:      "factor 1 should return position from pose2",
			vert1:     model.TriVertX{V: [3]byte{10, 20, 30}, LightNormalIndex: 0},
			vert2:     model.TriVertX{V: [3]byte{20, 40, 60}, LightNormalIndex: 0},
			factor:    1.0,
			expectedX: 20.0,
			expectedY: 40.0,
			expectedZ: 60.0,
		},
		{
			name:      "factor 0.5 should interpolate halfway",
			vert1:     model.TriVertX{V: [3]byte{10, 20, 30}, LightNormalIndex: 0},
			vert2:     model.TriVertX{V: [3]byte{20, 40, 60}, LightNormalIndex: 0},
			factor:    0.5,
			expectedX: 15.0,
			expectedY: 30.0,
			expectedZ: 45.0,
		},
		{
			name:      "factor 0.25 should interpolate at 25%",
			vert1:     model.TriVertX{V: [3]byte{0, 0, 0}, LightNormalIndex: 0},
			vert2:     model.TriVertX{V: [3]byte{100, 100, 100}, LightNormalIndex: 0},
			factor:    0.25,
			expectedX: 25.0,
			expectedY: 25.0,
			expectedZ: 25.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := interpolateVertexPosition(tt.vert1, tt.vert2, scale, origin, tt.factor)

			tolerance := float32(0.01)
			if math.Abs(float64(result[0]-tt.expectedX)) > float64(tolerance) ||
				math.Abs(float64(result[1]-tt.expectedY)) > float64(tolerance) ||
				math.Abs(float64(result[2]-tt.expectedZ)) > float64(tolerance) {
				t.Errorf("interpolateVertexPosition() = [%f, %f, %f], want [%f, %f, %f]",
					result[0], result[1], result[2], tt.expectedX, tt.expectedY, tt.expectedZ)
			}
		})
	}
}

// TestClamp01 tests clamping to [0, 1] range.
func TestClamp01(t *testing.T) {
	tests := []struct {
		name     string
		value    float32
		expected float32
	}{
		{"negative value clamped to 0", -0.5, 0.0},
		{"zero stays zero", 0.0, 0.0},
		{"value in range stays same", 0.5, 0.5},
		{"one stays one", 1.0, 1.0},
		{"value > 1 clamped to 1", 1.5, 1.0},
		{"large value clamped to 1", 10.0, 1.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := clamp01(tt.value)
			if result != tt.expected {
				t.Errorf("clamp01(%f) = %f, want %f", tt.value, result, tt.expected)
			}
		})
	}
}

// TestSetupAliasFrameInterpolation tests animation frame setup with interpolation.
func TestSetupAliasFrameInterpolation(t *testing.T) {
	// Create test frame descriptors
	frames := []AliasFrameDesc{
		{
			FirstPose: 0,
			NumPoses:  1,
			Interval:  0.1,
			Frame:     0,
		},
		{
			FirstPose: 1,
			NumPoses:  3,
			Interval:  0.05,
			Frame:     1,
		},
		{
			FirstPose: 4,
			NumPoses:  1,
			Interval:  0.1,
			Frame:     2,
		},
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
		blendRange    float32 // tolerance for blend comparison
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
			name:          "multi-pose frame at t=0 shows blend at start",
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
			name:          "multi-pose frame at t=0.025 is halfway through interval",
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
			name:          "with ModNoLerp flag, blend is always 0",
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
			name:          "invalid frame index defaults to 0",
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
			result := setupAliasFrameInterpolation(tt.frameIndex, frames, tt.timeSeconds, tt.lerpModels, tt.flags)

			if result.Pose1 != tt.expectedPose1 {
				t.Errorf("Pose1: got %d, want %d", result.Pose1, tt.expectedPose1)
			}
			if result.Pose2 != tt.expectedPose2 {
				t.Errorf("Pose2: got %d, want %d", result.Pose2, tt.expectedPose2)
			}

			if tt.blendRange == 0 {
				tt.blendRange = 0.01 // default tolerance
			}
			if math.Abs(float64(result.Blend-tt.expectedBlend)) > float64(tt.blendRange) {
				t.Errorf("Blend: got %f, want %f (±%f)", result.Blend, tt.expectedBlend, tt.blendRange)
			}
		})
	}
}

// TestLerpVec3 is not needed here as lerpVec3 is used internally only
// and lerpf is defined in screen.go

// BenchmarkInterpolateVertexPosition benchmarks vertex interpolation performance.
func BenchmarkInterpolateVertexPosition(b *testing.B) {
	scale := [3]float32{1.0, 1.0, 1.0}
	origin := [3]float32{0.0, 0.0, 0.0}
	vert1 := model.TriVertX{V: [3]byte{10, 20, 30}, LightNormalIndex: 0}
	vert2 := model.TriVertX{V: [3]byte{20, 40, 60}, LightNormalIndex: 0}
	factor := float32(0.5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = interpolateVertexPosition(vert1, vert2, scale, origin, factor)
	}
}

// BenchmarkSetupAliasFrameInterpolation benchmarks animation frame setup.
func BenchmarkSetupAliasFrameInterpolation(b *testing.B) {
	frames := []AliasFrameDesc{
		{FirstPose: 0, NumPoses: 1, Interval: 0.1},
		{FirstPose: 1, NumPoses: 3, Interval: 0.05},
		{FirstPose: 4, NumPoses: 2, Interval: 0.1},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = setupAliasFrameInterpolation(1, frames, float64(i)*0.016, true, 0)
	}
}

func TestResolveAliasSkinSlotUsesGroupedSkinTiming(t *testing.T) {
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
