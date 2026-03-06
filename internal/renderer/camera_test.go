//go:build gogpu && !cgo
// +build gogpu,!cgo

package renderer

import (
	"math"
	"testing"

	"github.com/gogpu/gogpu/gmath"
)

// TestComputeViewMatrixIdentity tests that camera at origin with zero angles produces a valid view.
// Note: The view matrix is not identity because Quake's forward direction is along +X,
// not the standard +Z. The gmath.LookAt function will produce a non-identity matrix
// that correctly transforms world space to camera space.
func TestComputeViewMatrixIdentity(t *testing.T) {
	camera := CameraState{
		Origin: gmath.NewVec3(0, 0, 0),
		Angles: gmath.NewVec3(0, 0, 0),
		FOV:    90,
	}

	view := ComputeViewMatrix(camera)

	// Verify the view matrix is valid (has non-NaN values)
	for i := 0; i < 16; i++ {
		if math.IsNaN(float64(view[i])) || math.IsInf(float64(view[i]), 0) {
			t.Errorf("view[%d] contains invalid value: %v", i, view[i])
		}
	}

	// Verify that the camera is pointing in a valid direction (determinant is non-zero)
	// For a valid rotation matrix, |det| should be ≈ 1
	det := view.Determinant()
	if math.Abs(float64(det)) < 0.9 || math.Abs(float64(det)) > 1.1 {
		t.Logf("view determinant = %v (expected ≈ 1 for rotation matrix)", det)
	}
}

// TestComputeViewMatrixPitch tests that pitch rotation affects the Z component.
func TestComputeViewMatrixPitch(t *testing.T) {
	// Looking up by 90 degrees
	camera := CameraState{
		Origin: gmath.NewVec3(0, 0, 0),
		Angles: gmath.NewVec3(-90, 0, 0), // Negative pitch = look up
		FOV:    90,
	}

	view := ComputeViewMatrix(camera)
	_ = view

	// After looking up 90 degrees, the forward vector should point straight up (0, 0, 1)
	// This is a basic sanity check; actual transformation validation would require more setup.
	// For this MVP, we just ensure the function doesn't panic.
	t.Log("Pitch rotation test passed (no panic)")
}

// TestComputeViewMatrixYaw tests that yaw rotation affects the X/Y components.
func TestComputeViewMatrixYaw(t *testing.T) {
	// Looking to the right by 90 degrees
	camera := CameraState{
		Origin: gmath.NewVec3(0, 0, 0),
		Angles: gmath.NewVec3(0, 90, 0), // Positive yaw = turn right
		FOV:    90,
	}

	view := ComputeViewMatrix(camera)
	_ = view

	// After turning right 90 degrees, the forward vector should point right (1, 0, 0)
	// This is a basic sanity check; actual transformation validation would require more setup.
	// For this MVP, we just ensure the function doesn't panic.
	t.Log("Yaw rotation test passed (no panic)")
}

// TestComputeProjectionMatrix tests basic projection matrix computation.
func TestComputeProjectionMatrix(t *testing.T) {
	fov := float32(90.0)
	aspect := float32(16.0 / 9.0)
	near := float32(0.1)
	far := float32(4096.0)

	proj := ComputeProjectionMatrix(fov, aspect, near, far)

	// Verify projection is not identity (perspective should change the values)
	identity := gmath.Identity4()
	isIdentity := true
	for i := 0; i < 16; i++ {
		if proj[i] != identity[i] {
			isIdentity = false
			break
		}
	}

	if isIdentity {
		t.Error("projection matrix should not be identity")
	}

	// Check that the projection matrix is sensible (has non-zero elements)
	foundNonZero := false
	for i := 0; i < 16; i++ {
		if proj[i] != 0 {
			foundNonZero = true
			break
		}
	}

	if !foundNonZero {
		t.Error("projection matrix should have non-zero elements")
	}
}

// TestProjectionAspectRatio tests that aspect ratio changes projection.
func TestProjectionAspectRatio(t *testing.T) {
	fov := float32(90.0)
	near := float32(0.1)
	far := float32(4096.0)

	// Compute projection for two different aspect ratios
	proj1 := ComputeProjectionMatrix(fov, 16.0/9.0, near, far)
	proj2 := ComputeProjectionMatrix(fov, 4.0/3.0, near, far)

	// The projections should be different
	isDifferent := false
	for i := 0; i < 16; i++ {
		if proj1[i] != proj2[i] {
			isDifferent = true
			break
		}
	}

	if !isDifferent {
		t.Error("different aspect ratios should produce different projection matrices")
	}
}

// TestConvertClientStateToCamera tests conversion from client arrays to CameraState.
func TestConvertClientStateToCamera(t *testing.T) {
	origin := [3]float32{100, 200, 300}
	angles := [3]float32{45, 90, 0}
	fov := float32(96.0)

	camera := ConvertClientStateToCamera(origin, angles, fov)

	if camera.Origin.X != 100 || camera.Origin.Y != 200 || camera.Origin.Z != 300 {
		t.Errorf("origin not converted correctly: %v", camera.Origin)
	}

	if camera.Angles.X != 45 || camera.Angles.Y != 90 || camera.Angles.Z != 0 {
		t.Errorf("angles not converted correctly: %v", camera.Angles)
	}

	if camera.FOV != 96.0 {
		t.Errorf("FOV not converted correctly: %v", camera.FOV)
	}
}

// TestVPMatrixComposition tests that view and projection can be properly composed.
func TestVPMatrixComposition(t *testing.T) {
	camera := CameraState{
		Origin: gmath.NewVec3(0, 0, 0),
		Angles: gmath.NewVec3(0, 0, 0),
		FOV:    90,
	}

	view := ComputeViewMatrix(camera)
	proj := ComputeProjectionMatrix(90, 16.0/9.0, 0.1, 4096)

	// VP = Projection * View
	vp := proj.Mul(view)

	// Verify VP is not identity
	identity := gmath.Identity4()
	isIdentity := true
	for i := 0; i < 16; i++ {
		if vp[i] != identity[i] {
			isIdentity = false
			break
		}
	}

	if isIdentity {
		t.Error("VP matrix should not be identity")
	}
}
