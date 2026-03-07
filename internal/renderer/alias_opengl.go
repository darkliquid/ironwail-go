//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"math"

	"github.com/ironwail/ironwail-go/internal/model"
)

// interpolateVertexPosition blends a vertex position between two poses.
// factor: 0 = fully pose1, 1 = fully pose2, 0.5 = halfway between.
func interpolateVertexPosition(pose1Vert, pose2Vert model.TriVertX, scale, origin [3]float32, factor float32) [3]float32 {
	pos1 := model.DecodeVertex(pose1Vert, scale, origin)
	pos2 := model.DecodeVertex(pose2Vert, scale, origin)

	// Linear interpolation: pos = pos1 + (pos2 - pos1) * factor
	result := [3]float32{
		pos1[0] + (pos2[0]-pos1[0])*factor,
		pos1[1] + (pos2[1]-pos1[1])*factor,
		pos1[2] + (pos2[2]-pos1[2])*factor,
	}
	return result
}

// buildAliasVerticesInterpolated builds vertices for an alias model with interpolation between two poses.
// pose1Index and pose2Index are the indices of the two poses to blend.
// blend: 0 = fully pose1, 1 = fully pose2.
func buildAliasVerticesInterpolated(alias *glAliasModel, mdl *model.Model, pose1Index, pose2Index int, blend float32, origin, angles [3]float32, entityScale float32, fullAngles bool) []WorldVertex {
	if alias == nil || mdl == nil || mdl.AliasHeader == nil {
		return nil
	}

	// Validate pose indices
	if pose1Index < 0 || pose1Index >= len(alias.poses) {
		return nil
	}
	if pose2Index < 0 || pose2Index >= len(alias.poses) {
		return nil
	}

	// Clamp blend to [0, 1]
	if blend < 0 {
		blend = 0
	} else if blend > 1 {
		blend = 1
	}
	if entityScale <= 0 {
		entityScale = 1
	}

	pose1 := alias.poses[pose1Index]
	pose2 := alias.poses[pose2Index]
	vertices := make([]WorldVertex, 0, len(alias.refs))

	hdr := mdl.AliasHeader
	scale := hdr.Scale
	origin_offset := hdr.ScaleOrigin

	for _, ref := range alias.refs {
		if ref.vertexIndex < 0 || ref.vertexIndex >= len(pose1) || ref.vertexIndex >= len(pose2) {
			continue
		}

		// Interpolate vertex position between the two poses
		position := interpolateVertexPosition(pose1[ref.vertexIndex], pose2[ref.vertexIndex], scale, origin_offset, blend)
		position[0] *= entityScale
		position[1] *= entityScale
		position[2] *= entityScale

		// For normal, we interpolate the compressed normal index values
		// Since we don't have interpolated normals, we use the first pose's normal
		// A more sophisticated approach would interpolate normal directions
		normal := model.GetNormal(pose1[ref.vertexIndex].LightNormalIndex)

		// Apply rotation transforms
		if fullAngles {
			position = rotateAliasAngles(position, angles)
			normal = rotateAliasAngles(normal, angles)
		} else {
			position = rotateAliasYaw(position, angles[1])
			normal = rotateAliasYaw(normal, angles[1])
		}

		// Apply entity position
		position[0] += origin[0]
		position[1] += origin[1]
		position[2] += origin[2]

		vertices = append(vertices, WorldVertex{
			Position:      position,
			TexCoord:      ref.texCoord,
			LightmapCoord: [2]float32{},
			Normal:        normal,
		})
	}

	return vertices
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
