//go:build gogpu && !cgo
// +build gogpu,!cgo

// Package renderer provides GPU-accelerated rendering for the Ironwail-Go engine.
package renderer

import (
	"log/slog"
	"math"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

// CameraState holds the player's camera position and orientation for view setup.
// This is populated from client prediction state each frame.
type CameraState struct {
	// Origin is the camera's world position (player eye position).
	// This comes from client.PredictedOrigin.
	Origin types.Vec3

	// Angles are the pitch, yaw, and roll rotations in degrees.
	// Pitch: rotation around the right/X axis (up/down looking)
	// Yaw: rotation around the up/Z axis (left/right turning)
	// Roll: rotation around the forward/Y axis (rarely used, usually 0)
	// These come from client.ViewAngles.
	Angles types.Vec3

	// FOV is the field of view in degrees (typically 90 or 96 for widescreen).
	FOV float32

	// Time is the current game time, used for animation (view bob, etc).
	Time float32

	// WaterwarpFOV enables sinusoidal FOV oscillation for the r_waterwarp > 1 mode.
	// Both backends apply it through the shared projection FOV helper.
	WaterwarpFOV bool
}

// ViewMatrixData holds precomputed view and projection matrices for rendering.
// All matrices are cached to avoid recomputation each frame.
type ViewMatrixData struct {
	// View matrix transforms world coordinates to camera-relative coordinates.
	// Computed from camera position and orientation.
	View types.Mat4

	// Projection matrix transforms camera-relative coordinates to NDC (-1..1).
	// Computed from FOV, aspect ratio, and near/far clipping distances.
	Projection types.Mat4

	// VP is the combined View × Projection matrix.
	// Used by shaders for vertex transformation.
	VP types.Mat4

	// InverseVP is the inverse of the VP matrix.
	// Used for screen-space calculations and ray tracing.
	InverseVP types.Mat4
}

// ComputeViewMatrix computes a view matrix from camera state using the
// C Ironwail convention:
//
//	View = Rx(-roll) · Ry(-pitch) · Rz(-yaw) · T(-origin)
//
// There is no coordinate remapping in the view matrix — the projection
// matrix ([types.FrustumMatrix]) handles the Quake→clip-space conversion.
func ComputeViewMatrix(camera CameraState) types.Mat4 {
	slog.Debug("ComputeViewMatrix",
		"origin", camera.Origin,
		"angles", camera.Angles,
		"fov", camera.FOV)

	viewMatrix := types.ViewMatrix(camera.Angles, camera.Origin)

	slog.Debug("View matrix computed",
		"origin", camera.Origin,
		"angles", camera.Angles,
		"view_m00", viewMatrix[0],
		"view_m11", viewMatrix[5],
		"view_m22", viewMatrix[10],
		"view_m33", viewMatrix[15])
	return viewMatrix
}

// ComputeProjectionMatrix computes a Quake-style perspective projection matrix
// that also bakes in the Quake→clip-space coordinate conversion.
//
// Parameters:
//   - fovDegrees: horizontal field of view in degrees (typically 90-96)
//   - aspect: aspect ratio (width / height)
//   - near: near clipping plane distance (typically 0.1)
//   - far: far clipping plane distance (typically 4096 for Quake)
//
// The projection uses [types.FrustumMatrix] which follows C Ironwail's
// GL_FrustumMatrix — it maps Quake axes (X-forward, Y-left, Z-up) directly
// to clip space without requiring a separate coordinate-system remapping in
// the view matrix.
func ComputeProjectionMatrix(fovDegrees, aspect, near, far float32) types.Mat4 {
	fovxRad := fovDegrees * math.Pi / 180.0
	fovyRad := float32(2 * math.Atan(float64(float32(math.Tan(float64(fovxRad/2)))/aspect)))
	return types.FrustumMatrix(fovxRad, fovyRad, near, far)
}

// ConvertClientStateToCamera converts client prediction state to camera state.
// This is the main integration point between the client system and the renderer.
func ConvertClientStateToCamera(origin [3]float32, angles [3]float32, fov float32) CameraState {
	return CameraState{
		Origin: types.NewVec3(origin[0], origin[1], origin[2]),
		Angles: types.NewVec3(angles[0], angles[1], angles[2]),
		FOV:    fov,
		Time:   0, // Will be set by caller if needed
	}
}
