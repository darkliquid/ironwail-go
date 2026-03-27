//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"math"

	"github.com/ironwail/ironwail-go/internal/model"
	aliasimpl "github.com/ironwail/ironwail-go/internal/renderer/alias"
)

func interpolateVertexPosition(pose1Vert, pose2Vert model.TriVertX, scale, origin [3]float32, factor float32) [3]float32 {
	return aliasimpl.InterpolateVertexPosition(pose1Vert, pose2Vert, scale, origin, factor)
}

func buildAliasVerticesInterpolated(alias *glAliasModel, mdl *model.Model, pose1Index, pose2Index int, blend float32, origin, angles [3]float32, entityScale float32, fullAngles bool) []WorldVertex {
	if alias == nil || mdl == nil || mdl.AliasHeader == nil {
		return nil
	}
	return aliasimpl.BuildVerticesInterpolated(openGLAliasMesh(alias), mdl.AliasHeader, pose1Index, pose2Index, blend, origin, angles, entityScale, fullAngles)
}

func openGLAliasMesh(alias *glAliasModel) aliasimpl.Mesh {
	return aliasimpl.Mesh{
		Poses:    alias.poses,
		RefCount: len(alias.refs),
		RefAt: func(index int) aliasimpl.MeshRef {
			ref := alias.refs[index]
			return aliasimpl.MeshRef{VertexIndex: ref.vertexIndex, TexCoord: ref.texCoord}
		},
	}
}

// InterpolationData holds the result of frame interpolation setup.
type InterpolationData struct {
	Pose1 int
	Pose2 int
	Blend float32
}

// setupAliasFrameInterpolation computes which two poses to blend between and the blend factor.
// This is called each frame to update animation state.
func setupAliasFrameInterpolation(frameIndex int, frames []AliasFrameDesc, timeSeconds float64, lerpModels bool, flags int) InterpolationData {
	var result InterpolationData

	if frameIndex < 0 || frameIndex >= len(frames) {
		frameIndex = 0
	}

	frameDesc := frames[frameIndex]
	if frameDesc.NumPoses <= 0 {
		result.Pose1 = frameDesc.FirstPose
		result.Pose2 = frameDesc.FirstPose
		result.Blend = 0
		return result
	}

	// Calculate which pose within the frame's animation sequence
	poseOffset := 0
	if frameDesc.NumPoses > 1 {
		interval := frameDesc.Interval
		if interval <= 0 {
			interval = 0.1
		}
		poseOffset = int(timeSeconds/float64(interval)) % frameDesc.NumPoses
	}

	currentPose := frameDesc.FirstPose + poseOffset

	// If this frame has multiple poses (animation sequence), we can blend within the animation
	// Otherwise, we show the same pose (Pose1 == Pose2, Blend = 0)
	if frameDesc.NumPoses <= 1 {
		result.Pose1 = currentPose
		result.Pose2 = currentPose
		result.Blend = 0
		return result
	}

	// For multi-pose frames, blend between current and next pose
	nextPose := frameDesc.FirstPose + (poseOffset+1)%frameDesc.NumPoses

	shouldLerp := lerpModels && (flags&ModNoLerp == 0)
	if shouldLerp {
		interval := frameDesc.Interval
		if interval <= 0 {
			interval = 0.1
		}
		// Calculate blend factor within current pose interval
		timeInInterval := math.Mod(timeSeconds, float64(interval))
		result.Blend = clamp01(float32(timeInInterval / float64(interval)))
	} else {
		result.Blend = 0 // No interpolation, show current pose fully
	}

	result.Pose1 = currentPose
	result.Pose2 = nextPose

	return result
}

// AliasFrameDesc describes an alias model frame with animation info.
type AliasFrameDesc struct {
	FirstPose int
	NumPoses  int
	Interval  float32
	BBoxMin   [4]byte // trivertx_t packed
	BBoxMax   [4]byte // trivertx_t packed
	Frame     int
	Name      [16]byte
}
