package alias

import (
	"math"

	"github.com/ironwail/ironwail-go/internal/model"
	worldimpl "github.com/ironwail/ironwail-go/internal/renderer/world"
)

type MeshRef struct {
	VertexIndex int
	TexCoord    [2]float32
}

type Mesh struct {
	Poses    [][]model.TriVertX
	RefCount int
	RefAt    func(index int) MeshRef
}

type FrameDesc struct {
	FirstPose int
	NumPoses  int
	Interval  float32
	BBoxMin   [4]byte
	BBoxMax   [4]byte
	Frame     int
	Name      [16]byte
}

type InterpolationData struct {
	Pose1 int
	Pose2 int
	Blend float32
}

func MeshFromRefs(poses [][]model.TriVertX, refs []MeshRef) Mesh {
	return Mesh{
		Poses:    poses,
		RefCount: len(refs),
		RefAt: func(index int) MeshRef {
			return refs[index]
		},
	}
}

func InterpolateVertexPosition(pose1Vert, pose2Vert model.TriVertX, scale, origin [3]float32, factor float32) [3]float32 {
	pos1 := model.DecodeVertex(pose1Vert, scale, origin)
	pos2 := model.DecodeVertex(pose2Vert, scale, origin)
	return [3]float32{
		pos1[0] + (pos2[0]-pos1[0])*factor,
		pos1[1] + (pos2[1]-pos1[1])*factor,
		pos1[2] + (pos2[2]-pos1[2])*factor,
	}
}

func BuildVertices(mesh Mesh, hdr *model.AliasHeader, poseIndex int, origin, angles [3]float32, fullAngles bool) []worldimpl.WorldVertex {
	return BuildVerticesInterpolated(mesh, hdr, poseIndex, poseIndex, 0, origin, angles, 1, fullAngles)
}

func SetupFrameInterpolation(frameIndex int, frames []FrameDesc, timeSeconds float64, lerpModels bool, flags int) InterpolationData {
	var result InterpolationData
	if len(frames) == 0 {
		return result
	}
	if frameIndex < 0 || frameIndex >= len(frames) {
		frameIndex = 0
	}

	frameDesc := frames[frameIndex]
	if frameDesc.NumPoses <= 0 {
		result.Pose1 = frameDesc.FirstPose
		result.Pose2 = frameDesc.FirstPose
		return result
	}

	poseOffset := 0
	if frameDesc.NumPoses > 1 {
		interval := frameDesc.Interval
		if interval <= 0 {
			interval = 0.1
		}
		poseOffset = int(timeSeconds/float64(interval)) % frameDesc.NumPoses
	}

	currentPose := frameDesc.FirstPose + poseOffset
	if frameDesc.NumPoses <= 1 {
		result.Pose1 = currentPose
		result.Pose2 = currentPose
		return result
	}

	nextPose := frameDesc.FirstPose + (poseOffset+1)%frameDesc.NumPoses
	if lerpModels && (flags&ModNoLerp == 0) {
		interval := frameDesc.Interval
		if interval <= 0 {
			interval = 0.1
		}
		timeInInterval := math.Mod(timeSeconds, float64(interval))
		result.Blend = clamp01(float32(timeInInterval / float64(interval)))
	}

	result.Pose1 = currentPose
	result.Pose2 = nextPose
	return result
}

func BuildVerticesInterpolated(mesh Mesh, hdr *model.AliasHeader, pose1Index, pose2Index int, blend float32, origin, angles [3]float32, entityScale float32, fullAngles bool) []worldimpl.WorldVertex {
	if hdr == nil || mesh.RefAt == nil {
		return nil
	}
	if pose1Index < 0 || pose1Index >= len(mesh.Poses) || pose2Index < 0 || pose2Index >= len(mesh.Poses) {
		return nil
	}
	blend = clamp01(blend)
	if entityScale <= 0 {
		entityScale = 1
	}

	pose1 := mesh.Poses[pose1Index]
	pose2 := mesh.Poses[pose2Index]
	vertices := make([]worldimpl.WorldVertex, 0, mesh.RefCount)
	for i := 0; i < mesh.RefCount; i++ {
		ref := mesh.RefAt(i)
		if ref.VertexIndex < 0 || ref.VertexIndex >= len(pose1) || ref.VertexIndex >= len(pose2) {
			continue
		}

		position := InterpolateVertexPosition(pose1[ref.VertexIndex], pose2[ref.VertexIndex], hdr.Scale, hdr.ScaleOrigin, blend)
		position[0] *= entityScale
		position[1] *= entityScale
		position[2] *= entityScale

		normal := model.GetNormal(pose1[ref.VertexIndex].LightNormalIndex)
		if fullAngles {
			position = RotateAngles(position, angles)
			normal = RotateAngles(normal, angles)
		} else {
			position = RotateYaw(position, angles[1])
			normal = RotateYaw(normal, angles[1])
		}

		position[0] += origin[0]
		position[1] += origin[1]
		position[2] += origin[2]

		vertices = append(vertices, worldimpl.WorldVertex{
			Position:      position,
			TexCoord:      ref.TexCoord,
			LightmapCoord: [2]float32{},
			Normal:        normal,
		})
	}
	return vertices
}

func RotateAngles(v [3]float32, angles [3]float32) [3]float32 {
	v = RotateYaw(v, angles[1])
	v = RotatePitch(v, angles[0])
	v = RotateRoll(v, angles[2])
	return v
}

func RotateYaw(v [3]float32, yawDegrees float32) [3]float32 {
	if yawDegrees == 0 {
		return v
	}
	yaw := float32(math.Pi) * yawDegrees / 180.0
	sinYaw := float32(math.Sin(float64(yaw)))
	cosYaw := float32(math.Cos(float64(yaw)))
	return [3]float32{
		v[0]*cosYaw - v[1]*sinYaw,
		v[0]*sinYaw + v[1]*cosYaw,
		v[2],
	}
}

func RotatePitch(v [3]float32, pitchDegrees float32) [3]float32 {
	if pitchDegrees == 0 {
		return v
	}
	pitch := float32(math.Pi) * pitchDegrees / 180.0
	sinPitch := float32(math.Sin(float64(pitch)))
	cosPitch := float32(math.Cos(float64(pitch)))
	return [3]float32{
		v[0],
		v[1]*cosPitch - v[2]*sinPitch,
		v[1]*sinPitch + v[2]*cosPitch,
	}
}

func RotateRoll(v [3]float32, rollDegrees float32) [3]float32 {
	if rollDegrees == 0 {
		return v
	}
	roll := float32(math.Pi) * rollDegrees / 180.0
	sinRoll := float32(math.Sin(float64(roll)))
	cosRoll := float32(math.Cos(float64(roll)))
	return [3]float32{
		v[0]*cosRoll + v[2]*sinRoll,
		v[1],
		-v[0]*sinRoll + v[2]*cosRoll,
	}
}
