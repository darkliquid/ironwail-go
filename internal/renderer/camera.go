//go:build gogpu && !cgo
// +build gogpu,!cgo

// Package renderer provides GPU-accelerated rendering for the Ironwail-Go engine.
package renderer

import (
	"log/slog"
	"math"

	"github.com/gogpu/gogpu/gmath"
)

// CameraState holds the player's camera position and orientation for view setup.
// This is populated from client prediction state each frame.
type CameraState struct {
	// Origin is the camera's world position (player eye position).
	// This comes from client.PredictedOrigin.
	Origin gmath.Vec3

	// Angles are the pitch, yaw, and roll rotations in degrees.
	// Pitch: rotation around the right/X axis (up/down looking)
	// Yaw: rotation around the up/Z axis (left/right turning)
	// Roll: rotation around the forward/Y axis (rarely used, usually 0)
	// These come from client.ViewAngles.
	Angles gmath.Vec3

	// FOV is the field of view in degrees (typically 90 or 96 for widescreen).
	FOV float32

	// Time is the current game time, used for animation (view bob, etc).
	Time float32
}

// ViewMatrixData holds precomputed view and projection matrices for rendering.
// All matrices are cached to avoid recomputation each frame.
type ViewMatrixData struct {
	// View matrix transforms world coordinates to camera-relative coordinates.
	// Computed from camera position and orientation.
	View gmath.Mat4

	// Projection matrix transforms camera-relative coordinates to NDC (-1..1).
	// Computed from FOV, aspect ratio, and near/far clipping distances.
	Projection gmath.Mat4

	// VP is the combined View × Projection matrix.
	// Used by shaders for vertex transformation.
	VP gmath.Mat4

	// InverseVP is the inverse of the VP matrix.
	// Used for screen-space calculations and ray tracing.
	InverseVP gmath.Mat4
}

// ComputeViewMatrix computes a view matrix from camera state.
// The view matrix transforms world coordinates to camera-relative coordinates.
//
// The implementation uses Euler angles (pitch, yaw, roll) in the standard Quake convention:
//   - Pitch: rotation around the right axis (X), positive = look down
//   - Yaw: rotation around the up axis (Z), positive = turn right
//   - Roll: rotation around the forward axis (Y), usually 0
//
// The camera is positioned at origin with orientation derived from angles.
func ComputeViewMatrix(camera CameraState) gmath.Mat4 {
	// Log camera setup for debugging
	slog.Debug("ComputeViewMatrix",
		"origin", camera.Origin,
		"angles", camera.Angles,
		"fov", camera.FOV)
	
	// Convert angles from degrees to radians
	pitchRad := float32(math.Pi) / 180.0 * camera.Angles.X
	yawRad := float32(math.Pi) / 180.0 * camera.Angles.Y
	rollRad := float32(math.Pi) / 180.0 * camera.Angles.Z

	// Compute forward, right, and up vectors from Euler angles.
	// This follows the standard Quake engine convention.
	forward := angleVectors(pitchRad, yawRad)
	right := angleVectorsRight(yawRad)
	up := angleVectorsUp(pitchRad, yawRad)

	// If roll is non-zero, rotate the right and up vectors around the forward axis.
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

	// Compute the target point by moving forward from the camera origin.
	target := gmath.Vec3{
		X: camera.Origin.X + forward.X,
		Y: camera.Origin.Y + forward.Y,
		Z: camera.Origin.Z + forward.Z,
	}

	// Use gmath.LookAt to build the view matrix.
	// LookAt creates a view matrix that looks from eye (origin) to target, with up vector.
	viewMatrix := gmath.LookAt(camera.Origin, target, up)
	slog.Info("View matrix computed",
		"origin", camera.Origin,
		"target", target,
		"up", up,
		"forward", forward,
		"right", right,
		"view_m00", viewMatrix[0],
		"view_m11", viewMatrix[5],
		"view_m22", viewMatrix[10],
		"view_m33", viewMatrix[15])
	return viewMatrix
}

// ComputeProjectionMatrix computes a perspective projection matrix.
//
// Parameters:
//   - fovDegrees: field of view in degrees (typically 90-96)
//   - aspect: aspect ratio (width / height)
//   - near: near clipping plane distance (typically 0.1)
//   - far: far clipping plane distance (typically 4096 for Quake)
//
// The projection matrix transforms camera-relative coordinates to normalized device coordinates.
func ComputeProjectionMatrix(fovDegrees, aspect, near, far float32) gmath.Mat4 {
	// Convert FOV from degrees to radians
	fovRad := float32(math.Pi) / 180.0 * fovDegrees

	// gogpu's gmath.Perspective expects FOV in radians and computes the projection correctly.
	return gmath.Perspective(fovRad, aspect, near, far)
}

// angleVectors computes the forward vector from pitch and yaw Euler angles.
// This follows Quake's angle convention where:
//   - Yaw rotates around the Z axis (up)
//   - Pitch rotates around the right axis (affects Z component)
//   - When yaw=0 and pitch=0, forward points along -Y (into the world)
func angleVectors(pitch, yaw float32) gmath.Vec3 {
	cosPitch := float32(math.Cos(float64(pitch)))
	sinPitch := float32(math.Sin(float64(pitch)))
	cosYaw := float32(math.Cos(float64(yaw)))
	sinYaw := float32(math.Sin(float64(yaw)))

	// Forward vector in Quake convention:
	// When pitch=0, yaw=0: forward = (0, -1, 0)  [pointing in -Y]
	// When pitch=0, yaw=π/2: forward = (1, 0, 0) [pointing in +X, to the right]
	// X = sin(yaw) * cos(pitch)
	// Y = -cos(yaw) * cos(pitch)
	// Z = -sin(pitch)  [negative because positive pitch = looking up]
	return gmath.Vec3{
		X: sinYaw * cosPitch,
		Y: -cosYaw * cosPitch,
		Z: -sinPitch,
	}
}

// angleVectorsRight computes the right vector from yaw angle.
// The right vector is perpendicular to the forward direction.
// When yaw=0: right = (1, 0, 0) [pointing in +X]
// When yaw=π/2: right = (0, -1, 0) [pointing in -Y]
func angleVectorsRight(yaw float32) gmath.Vec3 {
	cosYaw := float32(math.Cos(float64(yaw)))
	sinYaw := float32(math.Sin(float64(yaw)))

	// Right vector (perpendicular to forward in XY plane):
	// X = cos(yaw)
	// Y = sin(yaw)
	// Z = 0 (right is always horizontal in Quake)
	return gmath.Vec3{
		X: cosYaw,
		Y: sinYaw,
		Z: 0,
	}
}

// angleVectorsUp computes the up vector from pitch and yaw angles.
// The up vector is computed as cross(right, forward) to maintain orthogonality.
func angleVectorsUp(pitch, yaw float32) gmath.Vec3 {
	cosPitch := float32(math.Cos(float64(pitch)))
	sinPitch := float32(math.Sin(float64(pitch)))
	cosYaw := float32(math.Cos(float64(yaw)))
	sinYaw := float32(math.Sin(float64(yaw)))

	// Up vector: cross(right, forward)
	// right = (cos(yaw), sin(yaw), 0)
	// forward = (sin(yaw)*cos(pitch), -cos(yaw)*cos(pitch), -sin(pitch))
	right := gmath.Vec3{X: cosYaw, Y: sinYaw, Z: 0}
	forward := gmath.Vec3{
		X: sinYaw * cosPitch,
		Y: -cosYaw * cosPitch,
		Z: -sinPitch,
	}
	return right.Cross(forward)
}

// ConvertClientStateToCamera converts client prediction state to camera state.
// This is the main integration point between the client system and the renderer.
func ConvertClientStateToCamera(origin [3]float32, angles [3]float32, fov float32) CameraState {
	return CameraState{
		Origin: gmath.NewVec3(origin[0], origin[1], origin[2]),
		Angles: gmath.NewVec3(angles[0], angles[1], angles[2]),
		FOV:    fov,
		Time:   0, // Will be set by caller if needed
	}
}
