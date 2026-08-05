package alias

import (
	"math"

	"github.com/darkliquid/ironwail-go/internal/model"
	worldimpl "github.com/darkliquid/ironwail-go/internal/renderer/world"
)

type MeshRef struct {
	VertexIndex int
	TexCoord    [2]float32
}

type MeshRefConvertible interface {
	AliasMeshRef() MeshRef
}

type Mesh struct {
	Poses    [][]model.TriVertX
	RefCount int
	RefAt    func(index int) MeshRef
	Refs     []MeshRef
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
		Refs:     refs,
		RefAt: func(index int) MeshRef {
			return refs[index]
		},
	}
}

func MeshFromAccessor[R any](poses [][]model.TriVertX, refs []R, adapt func(R) MeshRef) Mesh {
	if adapt == nil {
		return Mesh{Poses: poses}
	}
	return Mesh{
		Poses:    poses,
		RefCount: len(refs),
		RefAt: func(index int) MeshRef {
			return adapt(refs[index])
		},
	}
}

func MeshFromConvertibleRefs[R MeshRefConvertible](poses [][]model.TriVertX, refs []R) Mesh {
	return Mesh{
		Poses:    poses,
		RefCount: len(refs),
		RefAt: func(index int) MeshRef {
			return refs[index].AliasMeshRef()
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
	return BuildVerticesInterpolatedInto(nil, mesh, hdr, pose1Index, pose2Index, blend, origin, angles, entityScale, fullAngles)
}

func BuildVerticesInterpolatedInto(dst []worldimpl.WorldVertex, mesh Mesh, hdr *model.AliasHeader, pose1Index, pose2Index int, blend float32, origin, angles [3]float32, entityScale float32, fullAngles bool) []worldimpl.WorldVertex {
	if hdr == nil || (mesh.RefAt == nil && mesh.Refs == nil) {
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
	vertices := dst[:0]
	if vertices == nil {
		vertices = make([]worldimpl.WorldVertex, 0, mesh.RefCount)
	}

	// Pre-compute rotation ONCE per entity draw call instead of per vertex
	useYawOnly := !fullAngles || (angles[0] == 0 && angles[2] == 0)
	var (
		sinYaw, cosYaw float32
		rotMat         [9]float32
	)

	if useYawOnly {
		if angles[1] != 0 {
			yaw := float32(math.Pi) * angles[1] / 180.0
			sinYaw = float32(math.Sin(float64(yaw)))
			cosYaw = float32(math.Cos(float64(yaw)))
		} else {
			cosYaw = 1.0
		}
	} else {
		rotMat = entityRotationMatrix(angles)
	}

	fastRefs := mesh.Refs

	scale := hdr.Scale
	scaleOrigin := hdr.ScaleOrigin

	for i := 0; i < mesh.RefCount; i++ {
		var ref MeshRef
		if fastRefs != nil {
			ref = fastRefs[i]
		} else {
			ref = mesh.RefAt(i)
		}
		if ref.VertexIndex < 0 || ref.VertexIndex >= len(pose1) || ref.VertexIndex >= len(pose2) {
			continue
		}

		v1 := pose1[ref.VertexIndex]
		v1x := float32(v1.V[0])
		v1y := float32(v1.V[1])
		v1z := float32(v1.V[2])

		var px, py, pz float32
		if blend > 0 {
			v2 := pose2[ref.VertexIndex]
			px = (v1x + (float32(v2.V[0])-v1x)*blend) * scale[0] + scaleOrigin[0]
			py = (v1y + (float32(v2.V[1])-v1y)*blend) * scale[1] + scaleOrigin[1]
			pz = (v1z + (float32(v2.V[2])-v1z)*blend) * scale[2] + scaleOrigin[2]
		} else {
			px = v1x*scale[0] + scaleOrigin[0]
			py = v1y*scale[1] + scaleOrigin[1]
			pz = v1z*scale[2] + scaleOrigin[2]
		}

		if entityScale != 1.0 {
			px *= entityScale
			py *= entityScale
			pz *= entityScale
		}

		normal := model.GetNormal(v1.LightNormalIndex)
		var nx, ny, nz float32
		if useYawOnly {
			if angles[1] != 0 {
				rpx := px*cosYaw - py*sinYaw
				rpy := px*sinYaw + py*cosYaw
				px = rpx
				py = rpy

				rnx := normal[0]*cosYaw - normal[1]*sinYaw
				rny := normal[0]*sinYaw + normal[1]*cosYaw
				nx = rnx
				ny = rny
				nz = normal[2]
			} else {
				nx = normal[0]
				ny = normal[1]
				nz = normal[2]
			}
		} else {
			rpx := rotMat[0]*px + rotMat[1]*py + rotMat[2]*pz
			rpy := rotMat[3]*px + rotMat[4]*py + rotMat[5]*pz
			rpz := rotMat[6]*px + rotMat[7]*py + rotMat[8]*pz
			px = rpx
			py = rpy
			pz = rpz

			nx = rotMat[0]*normal[0] + rotMat[1]*normal[1] + rotMat[2]*normal[2]
			ny = rotMat[3]*normal[0] + rotMat[4]*normal[1] + rotMat[5]*normal[2]
			nz = rotMat[6]*normal[0] + rotMat[7]*normal[1] + rotMat[8]*normal[2]
		}

		px += origin[0]
		py += origin[1]
		pz += origin[2]

		vertices = append(vertices, worldimpl.WorldVertex{
			Position:      [3]float32{px, py, pz},
			TexCoord:      ref.TexCoord,
			LightmapCoord: [2]float32{},
			Normal:        [3]float32{nx, ny, nz},
		})
	}
	return vertices
}


func entityRotationMatrix(angles [3]float32) [9]float32 {
	pitch := float32(math.Pi) * angles[0] / 180.0
	yaw := float32(math.Pi) * angles[1] / 180.0
	roll := float32(math.Pi) * angles[2] / 180.0

	sp := float32(math.Sin(float64(pitch)))
	cp := float32(math.Cos(float64(pitch)))
	sy := float32(math.Sin(float64(yaw)))
	cy := float32(math.Cos(float64(yaw)))
	sr := float32(math.Sin(float64(roll)))
	cr := float32(math.Cos(float64(roll)))

	// RotateAngles sequence: Rx(roll) -> Ry(-pitch) -> Rz(yaw)
	// Output row-major 3x3 matrix:
	return [9]float32{
		cy*cp, -sy*cr - cy*sp*sr, sy*sr - cy*sp*cr,
		sy*cp, cy*cr - sy*sp*sr, -cy*sr - sy*sp*cr,
		sp, cp * sr, cp * cr,
	}
}


// RotateAngles rotates v by Quake Euler angles (pitch, yaw, roll) matching
// C R_EntityMatrix / R_DrawAliasModel's sequence of glRotatef calls:
//
//	glRotatef ( angles[YAW],   0, 0, 1)   // yaw   about Z
//	glRotatef (-angles[PITCH], 0, 1, 0)   // pitch about Y with NEGATED sign
//	glRotatef ( angles[ROLL],  1, 0, 0)   // roll  about X
//
// In OpenGL each glRotatef right-multiplies the current matrix, so the
// resulting model matrix is M = Rz(yaw) · Ry(−pitch) · Rx(roll), which when
// applied to a model-space vertex v means Rx(roll) runs FIRST and Rz(yaw)
// LAST. Implement it in that order to stay bit-for-bit parity with C; the
// previous ordering (yaw → pitch → roll) composed the inverse matrix and
// produced visibly wrong viewmodel orientation whenever both yaw and pitch
// were nonzero (e.g. looking up/down after having turned away from yaw=0).
func RotateAngles(v [3]float32, angles [3]float32) [3]float32 {
	v = RotateRoll(v, angles[2])
	v = RotatePitch(v, angles[0])
	v = RotateYaw(v, angles[1])
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

// RotatePitch rotates v about the Y axis by -pitchDegrees, matching Quake's
// convention (glRotatef(-angles[PITCH], 0, 1, 0)). With Quake's axes
// (X=forward, Y=left, Z=up), a positive network-angle pitch tilts the nose
// DOWN, so we rotate world-space forward vectors by -pitch about Y.
func RotatePitch(v [3]float32, pitchDegrees float32) [3]float32 {
	if pitchDegrees == 0 {
		return v
	}
	pitch := float32(math.Pi) * pitchDegrees / 180.0
	s := float32(math.Sin(float64(pitch)))
	c := float32(math.Cos(float64(pitch)))
	// Rotation about Y by angle -pitch: applied to (x,y,z):
	//   x' = x*cos(-p) + z*sin(-p) =  x*c - z*s
	//   z' = -x*sin(-p) + z*cos(-p) = x*s + z*c
	return [3]float32{
		v[0]*c - v[2]*s,
		v[1],
		v[0]*s + v[2]*c,
	}
}

// RotateRoll rotates v about the X axis by rollDegrees, matching Quake's
// convention (glRotatef(angles[ROLL], 1, 0, 0)).
func RotateRoll(v [3]float32, rollDegrees float32) [3]float32 {
	if rollDegrees == 0 {
		return v
	}
	roll := float32(math.Pi) * rollDegrees / 180.0
	s := float32(math.Sin(float64(roll)))
	c := float32(math.Cos(float64(roll)))
	// Rotation about X by roll: applied to (x,y,z):
	//   y' = y*c - z*s
	//   z' = y*s + z*c
	return [3]float32{
		v[0],
		v[1]*c - v[2]*s,
		v[1]*s + v[2]*c,
	}
}
