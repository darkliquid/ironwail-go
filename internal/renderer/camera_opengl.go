//go:build (opengl || cgo) && !gogpu
// +build opengl cgo
// +build !gogpu

package renderer

import (
	"math"

	"github.com/darkliquid/ironwail-go/pkg/types"
)

// CameraState holds the player's camera position and orientation for view setup.
type CameraState struct {
	Origin types.Vec3
	Angles types.Vec3
	FOV    float32
	Time   float32

	// WaterwarpFOV enables sinusoidal FOV oscillation for the r_waterwarp > 1 mode.
	// When true and r_waterwarp > 1, UpdateCamera modulates the horizontal FOV using
	// the C Ironwail formula: fov = 2*atan(scale * tan(fov/2)), scale ≈ 0.97..1.00.
	// Mirrors C Ironwail R_SetupView r_waterwarp > 1 branch.
	WaterwarpFOV bool
}

// ViewMatrixData holds cached view/projection matrices for rendering.
type ViewMatrixData struct {
	View       types.Mat4
	Projection types.Mat4
	VP         types.Mat4
	InverseVP  types.Mat4
}

// ComputeViewMatrix computes a view matrix from camera state using Quake angle
// conventions. Delegates to [types.ViewMatrix] which builds:
//
//	View = Rx(-roll) · Ry(-pitch) · Rz(-yaw) · T(-origin)
func ComputeViewMatrix(camera CameraState) types.Mat4 {
	return types.ViewMatrix(camera.Angles, camera.Origin)
}

// ComputeProjectionMatrix computes a Quake-style perspective projection matrix.
// fovDegrees is the horizontal field of view in degrees. The function derives
// the vertical FOV from the aspect ratio and delegates to [types.FrustumMatrix],
// which bakes in the Quake→clip-space axis conversion.
func ComputeProjectionMatrix(fovDegrees, aspect, near, far float32) types.Mat4 {
	fovxRad := fovDegrees * (math.Pi / 180.0)
	fovyRad := float32(2.0 * math.Atan(math.Tan(float64(fovxRad)*0.5)/float64(aspect)))
	return types.FrustumMatrix(fovxRad, fovyRad, near, far)
}

// ConvertClientStateToCamera converts predicted client state to camera state.
func ConvertClientStateToCamera(origin [3]float32, angles [3]float32, fov float32) CameraState {
	return CameraState{
		Origin: types.NewVec3(origin[0], origin[1], origin[2]),
		Angles: types.NewVec3(angles[0], angles[1], angles[2]),
		FOV:    fov,
	}
}

// UpdateCamera updates the camera state and cached matrices.
// If camera.WaterwarpFOV is true and r_waterwarp > 1, the horizontal FOV is
// sinusoidally oscillated to produce the r_waterwarp 2 view-space warp effect.
// Mirrors C Ironwail R_SetupView r_waterwarp > 1 branch.
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

	r.viewMatrices.Projection = ComputeProjectionMatrix(projectionFOVForCamera(camera), aspect, nearPlane, farPlane)
	r.viewMatrices.VP = r.viewMatrices.Projection.Mul(r.viewMatrices.View)
}

// GetViewProjectionMatrix returns the cached view-projection matrix.
func (r *Renderer) GetViewProjectionMatrix() types.Mat4 {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.viewMatrices.VP
}
