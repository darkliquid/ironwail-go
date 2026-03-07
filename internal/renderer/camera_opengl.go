//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"math"

	"github.com/gogpu/gogpu/gmath"
)

// CameraState holds the player's camera position and orientation for view setup.
type CameraState struct {
	Origin gmath.Vec3
	Angles gmath.Vec3
	FOV    float32
	Time   float32
}

// ViewMatrixData holds cached view/projection matrices for rendering.
type ViewMatrixData struct {
	View       gmath.Mat4
	Projection gmath.Mat4
	VP         gmath.Mat4
	InverseVP  gmath.Mat4
}

// ComputeViewMatrix computes a view matrix from camera state using Quake angle conventions.
func ComputeViewMatrix(camera CameraState) gmath.Mat4 {
	pitchRad := float32(math.Pi) / 180.0 * camera.Angles.X
	yawRad := float32(math.Pi) / 180.0 * camera.Angles.Y
	rollRad := float32(math.Pi) / 180.0 * camera.Angles.Z

	forward := angleVectors(pitchRad, yawRad)
	right := angleVectorsRight(yawRad)
	up := angleVectorsUp(pitchRad, yawRad)

	if math.Abs(float64(rollRad)) > 1e-6 {
		cosRoll := float32(math.Cos(float64(rollRad)))
		sinRoll := float32(math.Sin(float64(rollRad)))
		oldRight := right
		right = gmath.Vec3{
			X: oldRight.X*cosRoll - up.X*sinRoll,
			Y: oldRight.Y*cosRoll - up.Y*sinRoll,
			Z: oldRight.Z*cosRoll - up.Z*sinRoll,
		}
		up = gmath.Vec3{
			X: oldRight.X*sinRoll + up.X*cosRoll,
			Y: oldRight.Y*sinRoll + up.Y*cosRoll,
			Z: oldRight.Z*sinRoll + up.Z*cosRoll,
		}
	}

	target := gmath.Vec3{
		X: camera.Origin.X + forward.X,
		Y: camera.Origin.Y + forward.Y,
		Z: camera.Origin.Z + forward.Z,
	}

	return gmath.LookAt(camera.Origin, target, up)
}

// ComputeProjectionMatrix computes a perspective projection matrix.
func ComputeProjectionMatrix(fovDegrees, aspect, near, far float32) gmath.Mat4 {
	fovRad := float32(math.Pi) / 180.0 * fovDegrees
	return gmath.Perspective(fovRad, aspect, near, far)
}

func angleVectors(pitch, yaw float32) gmath.Vec3 {
	cosPitch := float32(math.Cos(float64(pitch)))
	sinPitch := float32(math.Sin(float64(pitch)))
	cosYaw := float32(math.Cos(float64(yaw)))
	sinYaw := float32(math.Sin(float64(yaw)))
	return gmath.Vec3{
		X: cosYaw * cosPitch,
		Y: sinYaw * cosPitch,
		Z: -sinPitch,
	}
}

func angleVectorsRight(yaw float32) gmath.Vec3 {
	cosYaw := float32(math.Cos(float64(yaw)))
	sinYaw := float32(math.Sin(float64(yaw)))
	return gmath.Vec3{
		X: sinYaw,
		Y: -cosYaw,
		Z: 0,
	}
}

func angleVectorsUp(pitch, yaw float32) gmath.Vec3 {
	cosPitch := float32(math.Cos(float64(pitch)))
	sinPitch := float32(math.Sin(float64(pitch)))
	cosYaw := float32(math.Cos(float64(yaw)))
	sinYaw := float32(math.Sin(float64(yaw)))
	right := gmath.Vec3{X: sinYaw, Y: -cosYaw, Z: 0}
	forward := gmath.Vec3{
		X: cosYaw * cosPitch,
		Y: sinYaw * cosPitch,
		Z: -sinPitch,
	}
	return right.Cross(forward)
}

// ConvertClientStateToCamera converts predicted client state to camera state.
func ConvertClientStateToCamera(origin [3]float32, angles [3]float32, fov float32) CameraState {
	return CameraState{
		Origin: gmath.NewVec3(origin[0], origin[1], origin[2]),
		Angles: gmath.NewVec3(angles[0], angles[1], angles[2]),
		FOV:    fov,
	}
}

// UpdateCamera updates the camera state and cached matrices.
func (r *Renderer) UpdateCamera(camera CameraState, nearPlane, farPlane float32) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.cameraState = camera
	r.viewMatrices.View = ComputeViewMatrix(camera)

	aspect := float32(16.0 / 9.0)
	w, h := r.Size()
	if w > 0 && h > 0 {
		aspect = float32(w) / float32(h)
	}

	r.viewMatrices.Projection = ComputeProjectionMatrix(camera.FOV, aspect, nearPlane, farPlane)
	r.viewMatrices.VP = r.viewMatrices.Projection.Mul(r.viewMatrices.View)
}

// GetViewProjectionMatrix returns the cached view-projection matrix.
func (r *Renderer) GetViewProjectionMatrix() gmath.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.VP
}
